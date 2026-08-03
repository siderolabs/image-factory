// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
	"github.com/siderolabs/image-factory/internal/testoidc"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
)

const (
	testClientID    = "test-client-id"
	testCallbackURL = "https://factory.example.com/callback"
	testExternalURL = "https://factory.example.com"
)

// browserProvider is a provider with the browser login flow enabled, bundled with
// the issuer it was built against so tests can assert on the URLs it emits.
type browserProvider struct {
	*auth0.Provider

	issuer *url.URL
}

// setupBrowserProvider points a browser-login provider at an in-process OIDC server.
// That server serves discovery and JWKS only, so a code exchange against it 404s,
// which the callback failure cases rely on.
func setupBrowserProvider(t *testing.T) browserProvider {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	issuerURL := testoidc.StartServer(t, privateKey, testKeyID)

	issuer, err := url.Parse(issuerURL)
	require.NoError(t, err)

	sessionKey := make([]byte, 32)
	_, err = io.ReadFull(rand.Reader, sessionKey)
	require.NoError(t, err)

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain:            testDomain,
		Audience:          testAudience,
		ClientID:          testClientID,
		ClientSecret:      "test-client-secret",
		RedirectURL:       testCallbackURL,
		ExternalURL:       testExternalURL,
		SessionKey:        sessionKey,
		IssuerURLOverride: issuerURL,
	})
	require.NoError(t, err)
	require.True(t, p.BrowserLoginEnabled())

	return browserProvider{Provider: p, issuer: issuer}
}

// findCookie returns the Set-Cookie the recorder captured under name, or nil.
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() { //nolint:bodyclose // no body on a recorder result
		if c.Name == name {
			return c
		}
	}

	return nil
}

// login runs LoginHandler against target and returns the query parameters of
// the resulting Auth0 authorize URL together with the state cookie it set.
func (bp browserProvider) login(t *testing.T, target string) (url.Values, *http.Cookie) {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)

	require.NoError(t, bp.LoginHandler()(t.Context(), rec, req, nil))
	require.Equal(t, http.StatusFound, rec.Code)

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, bp.issuer.Host, location.Host)
	require.Equal(t, "/authorize", location.Path)

	stateCookie := findCookie(rec, "if_auth_state")
	require.NotNil(t, stateCookie, "login must leave the PKCE state behind for the callback")

	return location.Query(), stateCookie
}

// TestLoginHandlerAuthorizeRequest pins the parameters the authorize URL must carry.
// Losing any of them degrades the flow silently rather than failing outright: no
// offline_access means no refresh token, no code_challenge means no PKCE.
func TestLoginHandlerAuthorizeRequest(t *testing.T) {
	t.Parallel()

	q, _ := setupBrowserProvider(t).login(t, "/login")

	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, testClientID, q.Get("client_id"))
	require.Equal(t, testCallbackURL, q.Get("redirect_uri"))
	require.Equal(t, testAudience, q.Get("audience"))
	require.Equal(t, "S256", q.Get("code_challenge_method"))
	require.NotEmpty(t, q.Get("code_challenge"))
	require.NotEmpty(t, q.Get("state"))
	require.NotEmpty(t, q.Get("nonce"))

	require.Equal(t, []string{"openid", "offline_access"}, strings.Fields(q.Get("scope")))
	require.NotContains(t, q.Get("code_challenge"), "=", "challenge must be raw base64url")
	require.Empty(t, q.Get("organization"), "no ?org= was supplied")
}

// TestLoginHandlerStateIsPerRequest guards against the state and nonce being derived
// from anything stable, which would defeat the CSRF and ID token replay checks.
func TestLoginHandlerStateIsPerRequest(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	first, firstCookie := p.login(t, "/login")
	second, secondCookie := p.login(t, "/login")

	require.NotEqual(t, first.Get("state"), second.Get("state"))
	require.NotEqual(t, first.Get("nonce"), second.Get("nonce"))
	require.NotEqual(t, first.Get("code_challenge"), second.Get("code_challenge"))
	require.NotEqual(t, firstCookie.Value, secondCookie.Value)
}

func TestLoginHandlerStateCookie(t *testing.T) {
	t.Parallel()

	_, cookie := setupBrowserProvider(t).login(t, "/login")

	require.Equal(t, "/callback", cookie.Path, "state cookie is scoped to the callback route")
	require.True(t, cookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite,
		"Lax still allows the top-level GET Auth0 redirects back with")
	require.Positive(t, cookie.MaxAge)
}

// TestLoginHandlerForwardsOrg covers Omni linking straight into one Auth0
// organization, skipping Auth0's own org picker.
func TestLoginHandlerForwardsOrg(t *testing.T) {
	t.Parallel()

	q, _ := setupBrowserProvider(t).login(t, "/login?org=org_abc123")

	require.Equal(t, "org_abc123", q.Get("organization"))
}

// TestLoginHandlerRejectsForeignReturnTo checks the open redirect guard is wired
// into the handler: /login is unauthenticated, so ?return_to= is attacker controlled.
// The state cookie is encrypted, so the stored value is read back the only way it can
// be, by failing a callback and looking at where it sends the user.
func TestLoginHandlerRejectsForeignReturnTo(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	for _, target := range []string{
		"/login?return_to=" + url.QueryEscape("https://evil.example/steal"),
		"/login?return_to=" + url.QueryEscape("//evil.example/steal"),
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			_, cookie := p.login(t, target)

			location := p.failCallback(t, "/callback?code=some-code&state=mismatch", cookie)
			require.Equal(t, "/steal", location.Query().Get("return_to"),
				"the host must be stripped before the return_to is stored")
		})
	}
}

// failCallback runs a callback expected to send the user back to /login, and returns
// where to.
func (bp browserProvider) failCallback(t *testing.T, target string, cookie *http.Cookie) *url.URL {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)

	if cookie != nil {
		req.AddCookie(cookie)
	}

	require.NoError(t, bp.CallbackHandler()(t.Context(), rec, req, nil), "a retryable login failure is not an error")
	require.Equal(t, http.StatusFound, rec.Code)

	if session := findCookie(rec, "if_session"); session != nil {
		require.Empty(t, session.Value, "a failed callback must not establish a session")
	}

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/login", location.Path)

	return location
}

// TestCallbackHandlerRetryableFailuresRedirectToLogin covers the callbacks that are
// not part of a live login flow. /login mints fresh state, so these are worth retrying.
func TestCallbackHandlerRetryableFailuresRedirectToLogin(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	_, validCookie := p.login(t, "/login")

	for _, tc := range []struct {
		cookie *http.Cookie
		name   string
		target string
	}{
		{
			name:   "stray callback",
			target: "/callback",
		},
		{
			// The cookie came back, so the browser does keep cookies; only its contents
			// are useless. Starting over mints one this build can read.
			name:   "undecryptable state cookie",
			target: "/callback?code=some-code&state=whatever",
			cookie: &http.Cookie{Name: "if_auth_state", Value: "not-a-valid-cookie"},
		},
		{
			name:   "state mismatch",
			target: "/callback?code=some-code&state=attacker-supplied",
			cookie: validCookie,
		},
		{
			// Another tab finished the login and consumed the shared state cookie. The
			// session proves cookies work, so starting over will succeed.
			name:   "state cookie already spent but signed in",
			target: "/callback?code=some-code&state=whatever",
			cookie: p.IssueSessionCookie(t, "access-token"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p.failCallback(t, tc.target, tc.cookie)
		})
	}
}

// TestCallbackHandlerExpiredStateCookieIsRetryable covers a login left open past
// stateMaxAge. The cookie came back, so the browser keeps cookies and /login will
// work; only the age check rejected it. It also still knows where the user was going.
func TestCallbackHandlerExpiredStateCookieIsRetryable(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	const target = "/image/abc/v1.12.0/metal-amd64.iso"

	location := p.failCallback(t, "/callback?code=some-code&state=stale", p.ExpiredStateCookie(t, "stale", target))
	require.Equal(t, target, location.Query().Get("return_to"))
}

// TestCallbackHandlerStateMismatchKeepsCookie pins that a callback carrying the wrong
// state leaves the cookie alone. Clearing it would let a second tab — or a link an
// attacker hands the user — cancel the login actually in progress.
func TestCallbackHandlerStateMismatchKeepsCookie(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	_, cookie := p.login(t, "/login")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state=wrong", nil)
	req.AddCookie(cookie)

	require.NoError(t, p.CallbackHandler()(t.Context(), rec, req, nil))
	require.Nil(t, findCookie(rec, "if_auth_state"), "the in-progress login must keep its state")
}

// TestCallbackHandlerTerminalFailuresRenderErrorPage covers the callbacks Auth0 has
// already answered. Redirecting those to /login loops: Auth0 re-authorizes the same
// user off its own SSO session and the callback fails identically, forever.
func TestCallbackHandlerTerminalFailuresRenderErrorPage(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	validQuery, validCookie := p.login(t, "/login")
	validState := validQuery.Get("state")

	for _, tc := range []struct {
		name   string
		target string
	}{
		{
			name:   "auth0 reported an error",
			target: "/callback?error=access_denied&error_description=User+denied&state=" + validState,
		},
		{
			name:   "missing code",
			target: "/callback?state=" + validState,
		},
		{
			// The OIDC test server has no token endpoint, so the exchange 404s.
			name:   "code exchange fails",
			target: "/callback?code=some-code&state=" + validState,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, nil)
			req.AddCookie(validCookie)

			err := p.CallbackHandler()(t.Context(), rec, req, nil)
			require.True(t, xerrors.TagIs[enterrors.RejectedTag](err), "got %v", err)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Empty(t, rec.Header().Get("Location"), "an error page must not bounce back to /login")
			require.Contains(t, rec.Body.String(), "/logout", "the page must offer a way out")

			if session := findCookie(rec, "if_session"); session != nil {
				require.Empty(t, session.Value, "a failed callback must not establish a session")
			}
		})
	}
}

// TestCallbackHandlerCodeWithoutStateIsTerminal covers a browser that will not keep
// our cookies at all. Auth0 answered a login this browser started, so the state cookie
// was set and did not come back; sending it to /login just loses it again, forever.
func TestCallbackHandlerCodeWithoutStateIsTerminal(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state=whatever", nil)

	err := p.CallbackHandler()(t.Context(), rec, req, nil)
	require.True(t, xerrors.TagIs[enterrors.RejectedTag](err), "got %v", err)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Get("Location"))
	require.Contains(t, rec.Body.String(), "cookies are enabled")
}

// TestCallbackHandlerFailureKeepsReturnTo checks a retryable failure sends the user
// back to /login still pointed at what they were trying to reach. Dropping it means
// a transient failure silently relocates them to the root.
func TestCallbackHandlerFailureKeepsReturnTo(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	const target = "/image/abc/v1.12.0/metal-amd64.iso"

	// The state cookie is the only place return_to survives, so the login that
	// produced it has to carry the value.
	_, cookie := p.login(t, "/login?return_to="+url.QueryEscape(target))

	location := p.failCallback(t, "/callback?code=some-code&state=mismatch", cookie)
	require.Equal(t, target, location.Query().Get("return_to"))
}

// TestCallbackHandlerFailureWithoutReturnTo is the other half: a login started with
// no return_to must not gain one on the way back to /login.
func TestCallbackHandlerFailureWithoutReturnTo(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	_, cookie := p.login(t, "/login")

	location := p.failCallback(t, "/callback?code=some-code&state=mismatch", cookie)
	require.Empty(t, location.RawQuery, "a login with no return_to must not gain one")
}

// TestCallbackHandlerClearsStateCookie checks the single-use state cookie is dropped
// once read, so a replayed callback cannot reuse the code verifier.
func TestCallbackHandlerClearsStateCookie(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	q, cookie := p.login(t, "/login")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state="+q.Get("state"), nil)
	req.AddCookie(cookie)

	// The exchange fails against the test server; the cookie is cleared before that.
	require.Error(t, p.CallbackHandler()(t.Context(), rec, req, nil))

	cleared := findCookie(rec, "if_auth_state")
	require.NotNil(t, cleared)
	require.Empty(t, cleared.Value)
	require.Negative(t, cleared.MaxAge)
	require.Equal(t, "/callback", cleared.Path, "must match the path login used or the browser keeps it")
}

// TestLogoutHandlerGETConfirms checks a GET only offers the form. Logging out on GET
// lets any <img src="/logout"> or link prefetcher do it on the user's behalf.
func TestLogoutHandlerGETConfirms(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)

	require.NoError(t, p.LogoutHandler()(t.Context(), rec, req, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `method="POST"`)
	require.Nil(t, findCookie(rec, "if_session"), "a GET must not log the user out")
}

// TestLogoutHandler checks logout is not just local: without the redirect to Auth0
// the SSO session survives and the next /login signs the same user straight back in.
func TestLogoutHandler(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/logout", nil)

	require.NoError(t, p.LogoutHandler()(t.Context(), rec, req, nil))
	require.Equal(t, http.StatusSeeOther, rec.Code, "the browser must follow with a GET, not replay the POST")

	for _, name := range []string{"if_session", "if_auth_state"} {
		cookie := findCookie(rec, name)
		require.NotNil(t, cookie, "%s must be cleared", name)
		require.Empty(t, cookie.Value)
		require.Negative(t, cookie.MaxAge)
	}

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, p.issuer.Host, location.Host)
	require.Equal(t, "/v2/logout", location.Path)
	require.Equal(t, testClientID, location.Query().Get("client_id"))
	require.Equal(t, testExternalURL, location.Query().Get("returnTo"),
		"Auth0 only honors an allow-listed absolute returnTo")
}

// TestLogoutHandlerRejectsCrossOrigin covers an auto-submitting form on another site.
// SameSite=Lax withholds our cookies from it, but the handler does not need them: the
// redirect to Auth0 is followed with Auth0's own cookies and drops the SSO session.
func TestLogoutHandlerRejectsCrossOrigin(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	for _, site := range []string{"cross-site", "same-site"} {
		t.Run(site, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/logout", nil)
			req.Header.Set("Sec-Fetch-Site", site)

			require.Error(t, p.LogoutHandler()(t.Context(), rec, req, nil))
			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Empty(t, rec.Header().Get("Location"), "the browser must not reach Auth0's logout endpoint")
		})
	}
}

// TestMiddlewareDeniesByClient pins how an unauthenticated request is turned away.
// Each client shape needs a different answer: a redirect a browser will follow, a
// header htmx will act on, and a challenge for everything else. Getting it wrong is
// silent — the user sees a blank page or a native password box over the app.
func TestMiddlewareDeniesByClient(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	for _, tc := range []struct {
		assert  func(*testing.T, *httptest.ResponseRecorder)
		headers map[string]string
		name    string
		method  string
	}{
		{
			name:    "browser navigation",
			method:  http.MethodGet,
			headers: map[string]string{"Accept": "text/html,application/xhtml+xml"},
			assert: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusSeeOther, rec.Code)
				require.Equal(t, "/login?return_to=%2Fimage%2Fabc", rec.Header().Get("Location"))
			},
		},
		{
			name:    "browser form post",
			method:  http.MethodPost,
			headers: map[string]string{"Accept": "text/html"},
			assert: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusSeeOther, rec.Code, "a 302 would have the browser replay the body at /login")
			},
		},
		{
			name:   "htmx request",
			method: http.MethodPost,
			headers: map[string]string{
				"Accept":         "*/*",
				"HX-Request":     "true",
				"HX-Current-URL": "https://factory.example.com/ui/page",
			},
			assert: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, "/login?return_to=%2Fui%2Fpage", rec.Header().Get("Hx-Redirect"),
					"htmx must be sent to the page the user is on, not to the XHR endpoint")
				require.Empty(t, rec.Header().Values("WWW-Authenticate"),
					"a challenge on an XHR pops the browser's Basic auth dialog over the page")
			},
		},
		{
			name:    "api client",
			method:  http.MethodGet,
			headers: map[string]string{"Accept": "application/json"},
			assert: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Empty(t, rec.Header().Get("Location"), "an API client cannot follow a login redirect")
				require.NotEmpty(t, rec.Header().Values("WWW-Authenticate"))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), tc.method, "/image/abc", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()

			next := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
				t.Fatal("handler must not run for an unauthenticated request")

				return nil
			}

			require.Error(t, p.Middleware(next)(t.Context(), rec, req, nil))
			tc.assert(t, rec)
		})
	}
}
