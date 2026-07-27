// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0_test

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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

// setupBrowserProvider builds a provider with the browser login flow enabled,
// pointed at an in-process OIDC server. That server serves discovery and JWKS
// only, so a code exchange against it fails, which the callback failure cases rely on.
func setupBrowserProvider(t *testing.T) *auth0.Provider {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	issuerURL := testoidc.StartServer(t, privateKey, testKeyID)

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
		HTTPClient:        redirectingClient(t, issuerURL),
	})
	require.NoError(t, err)
	require.True(t, p.BrowserLoginEnabled())
	require.NoError(t, p.Warmup(t.Context()))

	return p
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

// doLogin runs LoginHandler against target and returns the query parameters of
// the resulting Auth0 authorize URL together with the state cookie it set.
func doLogin(t *testing.T, p *auth0.Provider, target string) (url.Values, *http.Cookie) {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)

	require.NoError(t, p.LoginHandler()(t.Context(), rec, req, nil))
	require.Equal(t, http.StatusFound, rec.Code)

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https", location.Scheme)
	require.Equal(t, testDomain, location.Host)
	require.Equal(t, "/authorize", location.Path)

	stateCookie := findCookie(rec, "if_auth_state")
	require.NotNil(t, stateCookie, "login must leave the PKCE state behind for the callback")

	return location.Query(), stateCookie
}

// TestUnreachableAuth0DoesNotBlockStartup is the regression test for a tenant outage at
// boot. Building the Auth0 client fetches the tenant JWKS, so doing it in NewProvider made
// a network blip fatal for the whole factory, including the routes needing no auth at all.
// Construction must succeed, browser login must still claim its routes, and the handler
// that needs the client must answer 503 until Run's retry loop gets through.
func TestUnreachableAuth0DoesNotBlockStartup(t *testing.T) {
	t.Parallel()

	// An already-closed server refuses connections immediately, standing in for the outage.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead.Close()

	sessionKey := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, sessionKey)
	require.NoError(t, err)

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain:            testDomain,
		Audience:          testAudience,
		ClientID:          testClientID,
		ClientSecret:      "test-client-secret",
		RedirectURL:       testCallbackURL,
		ExternalURL:       testExternalURL,
		SessionKey:        sessionKey,
		IssuerURLOverride: dead.URL,
		HTTPClient:        redirectingClient(t, dead.URL),
	})
	require.NoError(t, err, "an unreachable tenant must not fail construction")
	require.True(t, p.BrowserLoginEnabled(), "routes must register so they can answer 503")

	// LoginHandler needs no Auth0 round trip, so it still hands out a state cookie.
	q, stateCookie := doLogin(t, p, "/login")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/callback?code=some-code&state="+url.QueryEscape(q.Get("state")), nil)
	req.AddCookie(stateCookie)

	err = p.CallbackHandler()(t.Context(), rec, req, nil)
	require.Error(t, err)
	require.True(t, xerrors.TagIs[enterrors.NotReadyTag](err),
		"the callback must report 503, not a redirect loop or a 500: %v", err)
}

// TestLoginHandlerAuthorizeRequest pins the parameters the authorize URL must carry.
// Losing any of them degrades the flow silently rather than failing outright: no
// offline_access means no refresh token, no code_challenge means no PKCE.
func TestLoginHandlerAuthorizeRequest(t *testing.T) {
	t.Parallel()

	q, _ := doLogin(t, setupBrowserProvider(t), "/login")

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

	first, firstCookie := doLogin(t, p, "/login")
	second, secondCookie := doLogin(t, p, "/login")

	require.NotEqual(t, first.Get("state"), second.Get("state"))
	require.NotEqual(t, first.Get("nonce"), second.Get("nonce"))
	require.NotEqual(t, first.Get("code_challenge"), second.Get("code_challenge"))
	require.NotEqual(t, firstCookie.Value, secondCookie.Value)
}

func TestLoginHandlerStateCookie(t *testing.T) {
	t.Parallel()

	_, cookie := doLogin(t, setupBrowserProvider(t), "/login")

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

	q, _ := doLogin(t, setupBrowserProvider(t), "/login?org=org_abc123")

	require.Equal(t, "org_abc123", q.Get("organization"))
}

// TestLoginHandlerRejectsForeignReturnTo checks the open redirect guard is wired
// into the handler: /login is unauthenticated, so ?return_to= is attacker controlled.
func TestLoginHandlerRejectsForeignReturnTo(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	for _, target := range []string{
		"/login?return_to=" + url.QueryEscape("https://evil.example/steal"),
		"/login?return_to=" + url.QueryEscape("//evil.example/steal"),
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			_, cookie := doLogin(t, p, target)
			require.NotContains(t, cookie.Value, "evil.example",
				"the cookie is encrypted, so this only guards against a plaintext regression")
		})
	}
}

// TestCallbackHandlerFailuresRedirectToLogin covers every way the callback can fail
// before a session exists. All must land the user back on /login rather than on an
// error page, and none may set a session cookie.
func TestCallbackHandlerFailuresRedirectToLogin(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	// Reused by the cases that need to get past the CSRF check.
	validQuery, validCookie := doLogin(t, p, "/login")
	validState := validQuery.Get("state")

	for _, tc := range []struct {
		cookie *http.Cookie
		name   string
		target string
	}{
		{
			name:   "auth0 reported an error",
			target: "/callback?error=access_denied&error_description=User+denied",
			cookie: validCookie,
		},
		{
			name:   "no state cookie",
			target: "/callback?code=some-code&state=" + validState,
		},
		{
			name:   "undecryptable state cookie",
			target: "/callback?code=some-code&state=" + validState,
			cookie: &http.Cookie{Name: "if_auth_state", Value: "not-a-valid-cookie"},
		},
		{
			name:   "state mismatch",
			target: "/callback?code=some-code&state=attacker-supplied",
			cookie: validCookie,
		},
		{
			name:   "missing code",
			target: "/callback?state=" + validState,
			cookie: validCookie,
		},
		{
			// The OIDC test server has no token endpoint, so the exchange 404s.
			name:   "code exchange fails",
			target: "/callback?code=some-code&state=" + validState,
			cookie: validCookie,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, nil)

			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			require.NoError(t, p.CallbackHandler()(t.Context(), rec, req, nil),
				"a failed login is not a server error")
			require.Equal(t, http.StatusFound, rec.Code)

			location, err := url.Parse(rec.Header().Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "/login", location.Path)

			if session := findCookie(rec, "if_session"); session != nil {
				require.Empty(t, session.Value, "a failed callback must not establish a session")
			}
		})
	}
}

// TestCallbackHandlerFailureKeepsReturnTo checks a failed exchange sends the user
// back to /login still pointed at what they were trying to reach. Dropping it means
// a transient Auth0 error silently relocates them to the root.
func TestCallbackHandlerFailureKeepsReturnTo(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	const target = "/image/abc/v1.12.0/metal-amd64.iso"

	// The state cookie is the only place return_to survives, so the login that
	// produced it has to carry the value.
	q, cookie := doLogin(t, p, "/login?return_to="+url.QueryEscape(target))

	rec := httptest.NewRecorder()
	// The OIDC test server has no token endpoint, so the exchange fails.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state="+q.Get("state"), nil)
	req.AddCookie(cookie)

	require.NoError(t, p.CallbackHandler()(t.Context(), rec, req, nil))
	require.Equal(t, http.StatusFound, rec.Code)

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/login", location.Path)
	require.Equal(t, target, location.Query().Get("return_to"))
}

// TestCallbackHandlerFailureWithoutReturnTo is the other half: a login started with
// no return_to must not gain one on the way back to /login.
func TestCallbackHandlerFailureWithoutReturnTo(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	q, cookie := doLogin(t, p, "/login")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state="+q.Get("state"), nil)
	req.AddCookie(cookie)

	require.NoError(t, p.CallbackHandler()(t.Context(), rec, req, nil))
	require.Equal(t, "/login", rec.Header().Get("Location"),
		"a login with no return_to must not gain one")
}

// TestCallbackHandlerClearsStateCookie checks the single-use state cookie is dropped
// once read, so a replayed callback cannot reuse the code verifier.
func TestCallbackHandlerClearsStateCookie(t *testing.T) {
	t.Parallel()

	p := setupBrowserProvider(t)

	q, cookie := doLogin(t, p, "/login")

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?code=some-code&state="+q.Get("state"), nil)
	req.AddCookie(cookie)

	require.NoError(t, p.CallbackHandler()(t.Context(), rec, req, nil))

	cleared := findCookie(rec, "if_auth_state")
	require.NotNil(t, cleared)
	require.Empty(t, cleared.Value)
	require.Negative(t, cleared.MaxAge)
	require.Equal(t, "/callback", cleared.Path, "must match the path login used or the browser keeps it")
}

// TestLogoutHandler checks logout is not just local: without the redirect to Auth0
// the SSO session survives and the next /login signs the same user straight back in.
func TestLogoutHandler(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/logout", nil)

	require.NoError(t, setupBrowserProvider(t).LogoutHandler()(t.Context(), rec, req, nil))
	require.Equal(t, http.StatusFound, rec.Code)

	session := findCookie(rec, "if_session")
	require.NotNil(t, session, "the local session cookie must be cleared")
	require.Empty(t, session.Value)
	require.Negative(t, session.MaxAge)

	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https", location.Scheme)
	require.Equal(t, testDomain, location.Host)
	require.Equal(t, "/v2/logout", location.Path)
	require.Equal(t, testClientID, location.Query().Get("client_id"))
	require.Equal(t, testExternalURL, location.Query().Get("returnTo"),
		"Auth0 only honors an allow-listed absolute returnTo")
}
