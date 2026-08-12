// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0_test

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
	"github.com/siderolabs/image-factory/internal/testoidc"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret"
	testCallbackURL  = "https://factory.example.com/callback"
	testExternalURL  = "https://factory.example.com"
)

// browserProvider is a provider with the browser login flow enabled, bundled with
// the issuer it was built against so tests can assert on the URLs it emits.
type browserProvider struct {
	*auth0.Provider

	issuer *url.URL
	key    *rsa.PrivateKey
}

// validAccessToken mints a token this provider accepts, for the paths that need a
// request to authenticate rather than to be turned away.
func (bp browserProvider) validAccessToken(t *testing.T) string {
	t.Helper()

	return signToken(t, bp.key, bp.issuer.String(), testAudience, testOrgID, time.Now().Add(time.Hour))
}

// setupBrowserProvider points a browser-login provider at an in-process OIDC server that
// serves discovery and JWKS only, so a code exchange against it 404s.
func setupBrowserProvider(t *testing.T) browserProvider {
	t.Helper()

	return newBrowserProvider(t, nil)
}

// newBrowserProvider builds a browser-login provider against an in-process OIDC server
// serving routes on top of the key set, for the tests that need /oauth/token to answer.
func newBrowserProvider(t *testing.T, routes map[string]http.HandlerFunc) browserProvider {
	t.Helper()

	key := testoidc.GenerateKey()
	issuerURL := testoidc.StartServerWithRoutes(t, key, testKeyID, routes)

	issuer, err := url.Parse(issuerURL)
	require.NoError(t, err)

	// No test here depends on the key being random, only on it being the right length.
	sessionKey := make([]byte, 32)

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain:            testDomain,
		Audience:          testAudience,
		ClientID:          testClientID,
		ClientSecret:      testClientSecret,
		ExternalURL:       testExternalURL,
		SessionKey:        sessionKey,
		IssuerURLOverride: issuerURL,
	})
	require.NoError(t, err)
	require.True(t, p.BrowserLoginEnabled())

	return browserProvider{Provider: p, issuer: issuer, key: key}
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

	stateCookie := auth0.FindCookie(rec, "if_auth_state")
	require.NotNil(t, stateCookie, "login must leave the PKCE state behind for the callback")

	return location.Query(), stateCookie
}

// TestLoginHandlerAuthorizeRequest pins the authorize URL parameters, each of which degrades
// the flow silently when lost: PKCE, the ID token binding, and a JWT rather than an opaque token.
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

	// offline_access is absent: nothing renews a session.
	require.Equal(t, []string{"openid"}, strings.Fields(q.Get("scope")))
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

// TestLoginHandlerRejectsForeignReturnTo checks the open redirect guard is wired into the
// handler. The state cookie is encrypted, so a failed callback is the only way to read it back.
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

	requireNoSession(t, rec)

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

	const imagePath = "/image/abc/v1.12.0/metal-amd64.iso"

	_, returnToCookie := p.login(t, "/login?return_to="+url.QueryEscape(imagePath))

	signedInCookie, err := p.IssueSessionCookie("access-token")
	require.NoError(t, err)

	expiredCookie, err := p.ExpiredStateCookie("stale", imagePath)
	require.NoError(t, err)

	for _, tc := range []struct {
		cookie *http.Cookie
		name   string
		target string
		// returnTo is what the /login redirect must carry; empty means it must gain none.
		returnTo string
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
			cookie: signedInCookie,
		},
		{
			// Past stateMaxAge only the age check rejected it, so it still knows
			// where the user was headed.
			name:     "expired state cookie",
			target:   "/callback?code=some-code&state=stale",
			cookie:   expiredCookie,
			returnTo: imagePath,
		},
		{
			name:     "state mismatch keeps return_to",
			target:   "/callback?code=some-code&state=mismatch",
			cookie:   returnToCookie,
			returnTo: imagePath,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			location := p.failCallback(t, tc.target, tc.cookie)
			require.Equal(t, tc.returnTo, location.Query().Get("return_to"))
		})
	}
}

// TestCallbackHandlerStateMismatchKeepsCookie pins that a wrong-state callback leaves the
// cookie alone; clearing it would cancel the login actually in progress.
func TestCallbackHandlerStateMismatchKeepsCookie(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	_, cookie := p.login(t, "/login")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state=wrong", nil)
	req.AddCookie(cookie)

	require.NoError(t, p.CallbackHandler()(t.Context(), rec, req, nil))
	require.Nil(t, auth0.FindCookie(rec, "if_auth_state"), "the in-progress login must keep its state")
}

// TestCallbackHandlerTerminalFailuresRenderErrorPage covers the callbacks Auth0 has already
// answered, where a bounce through /login loops off Auth0's own SSO session.
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
			require.True(t, xerrors.TagIs[enterrors.RespondedTag](err), "got %v", err)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Empty(t, rec.Header().Get("Location"), "an error page must not bounce back to /login")
			require.Contains(t, rec.Body.String(), "/logout", "the page must offer a way out")

			requireNoSession(t, rec)
		})
	}
}

// TestCallbackHandlerCodeWithoutStateIsTerminal covers a browser that keeps no cookies at
// all: the state cookie was set and did not come back, so /login would only lose it again.
func TestCallbackHandlerCodeWithoutStateIsTerminal(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state=whatever", nil)

	err := p.CallbackHandler()(t.Context(), rec, req, nil)
	require.True(t, xerrors.TagIs[enterrors.RespondedTag](err), "got %v", err)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Get("Location"))
	require.Contains(t, rec.Body.String(), "cookies are enabled")
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

	cleared := auth0.FindCookie(rec, "if_auth_state")
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
	require.Nil(t, auth0.FindCookie(rec, "if_session"), "a GET must not log the user out")
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
		cookie := auth0.FindCookie(rec, name)
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

// TestLogoutHandlerRejectsCrossOrigin covers an auto-submitting form on another site, which
// SameSite=Lax does not stop: the Auth0 redirect is followed with Auth0's own cookies.
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

// TestMiddlewareDeniesByClient pins how each client shape is turned away: a redirect for a
// browser, a header for htmx, a challenge for everything else. Getting it wrong is silent.
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

			require.Error(t, p.Middleware(denyHandler(t))(t.Context(), rec, req, nil))
			tc.assert(t, rec)
		})
	}
}

// TestCookieAuthenticatedResponsesAreNotCacheable pins the headers that keep a shared cache
// off a session; nothing on /image/* sets Cache-Control, so a CDN may store it heuristically.
func TestCookieAuthenticatedResponsesAreNotCacheable(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	next := func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		w.WriteHeader(http.StatusOK)

		return nil
	}

	t.Run("cookie authenticated", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/abc/v1.11.0/metal-amd64.iso", nil)
		sessionCookie, err := p.IssueSessionCookie(p.validAccessToken(t))
		require.NoError(t, err)

		req.AddCookie(sessionCookie)

		rec := httptest.NewRecorder()
		require.NoError(t, p.Middleware(next)(t.Context(), rec, req, nil))

		require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		require.Contains(t, rec.Header().Values("Vary"), "Cookie")
	})

	// Still Vary, so a cache cannot serve this anonymous copy to a signed-in visitor.
	t.Run("anonymous", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/image/abc/v1.11.0/metal-amd64.iso", nil)
		req.Header.Set("Accept", "application/json")

		rec := httptest.NewRecorder()
		require.Error(t, p.Middleware(next)(t.Context(), rec, req, nil))
		require.Contains(t, rec.Header().Values("Vary"), "Cookie")
	})

	// The bearer case is TestMachineCredentialPullIsUnaffectedByBrowserLogin, which pins the
	// same absent Vary along with the rest of what the cookie path must not do.
}

// TestMachineCredentialPullIsUnaffectedByBrowserLogin pins that a credentialed node pull
// skips the session-cookie path entirely: no decrypt, no cache lookup, no cache headers.
func TestMachineCredentialPullIsUnaffectedByBrowserLogin(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)
	token := p.validAccessToken(t)

	next := func(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		w.WriteHeader(http.StatusOK)

		return nil
	}

	for _, tc := range []struct {
		auth func(*http.Request)
		name string
	}{
		{
			name: "basic credential, as Talos and OCI clients send it",
			auth: func(r *http.Request) { r.SetBasicAuth("token", token) },
		},
		{
			name: "bearer credential",
			auth: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/org/installer/blobs/sha256:abc", nil)
			tc.auth(req)

			// A stale browser session must not interfere with a credentialed pull.
			sessionCookie, err := p.IssueSessionCookie("some-other-access-token")
			require.NoError(t, err)

			req.AddCookie(sessionCookie)

			rec := httptest.NewRecorder()
			require.NoError(t, p.Middleware(next)(t.Context(), rec, req, nil))
			require.Equal(t, http.StatusOK, rec.Code, "the pull has to reach the handler")

			require.Empty(t, rec.Header().Values("Vary"),
				"the cookie path must not run for a credentialed pull")
			require.Empty(t, rec.Header().Values("Cache-Control"))
			require.Empty(t, rec.Result().Cookies(), //nolint:bodyclose // no body on a recorder result
				"a node pull must never be answered with a Set-Cookie")
		})
	}
}
