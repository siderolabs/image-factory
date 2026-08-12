// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
	"github.com/siderolabs/image-factory/internal/testoidc"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
)

// testAuthCode is the authorization code the browser carries back from Auth0.
const testAuthCode = "test-authorization-code"

// fakeTenant serves /oauth/token, the half of the flow Auth0 performs once the browser hands
// the code back.
type fakeTenant struct {
	// authorize is the /authorize query of the login in flight: the nonce to echo, and the
	// challenge the code verifier has to hash to.
	authorize url.Values

	// tweak adjusts the token set before signing, for the tests that need a bad one.
	tweak func(access, id *testoidc.TokenOptions)

	// issuer is only known once the server is listening, so it is filled in afterwards.
	issuer string
}

// serveToken answers the code exchange, asserting what Auth0 would reject.
// assert, not require: this runs on the test server's goroutine, where FailNow may not be called.
func (ft *fakeTenant) serveToken(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.PostFormValue("grant_type"))
		assert.Equal(t, testAuthCode, r.PostFormValue("code"))
		assert.Equal(t, testCallbackURL, r.PostFormValue("redirect_uri"),
			"Auth0 matches this against the redirect the code was issued for")
		assert.Equal(t, testClientSecret, r.PostFormValue("client_secret"),
			"the credentials belong in the body: a header would have x/oauth2 probe for the auth style first")

		assert.Equal(t, ft.authorize.Get("code_challenge"),
			oauth2.S256ChallengeFromVerifier(r.PostFormValue("code_verifier")), "PKCE verifier")

		expiry := time.Now().Add(time.Hour)
		access := testoidc.TokenOptions{
			KeyID: testKeyID, Issuer: ft.issuer, Subject: testSubject,
			Audience: []string{testAudience}, OrgID: testOrgID, Expiry: expiry,
		}
		// The ID token is audienced to the application rather than the API, and carries the nonce.
		id := testoidc.TokenOptions{
			KeyID: testKeyID, Issuer: ft.issuer, Subject: testSubject,
			Audience: []string{testClientID}, Nonce: ft.authorize.Get("nonce"), Expiry: expiry,
		}

		if ft.tweak != nil {
			ft.tweak(&access, &id)
		}

		// Signed with the key the JWKS server publishes, so the tokens verify.
		key := testoidc.GenerateKey()

		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"access_token": testoidc.SignToken(t, key, access),
			"id_token":     testoidc.SignToken(t, key, id),
			"expires_in":   3600,
			"token_type":   "Bearer",
		})
	}
}

// signIn runs /login and the callback the browser comes back to, returning the provider that
// served them, the callback's response, and the error it reported.
func signIn(t *testing.T, returnTo string, tweak func(access, id *testoidc.TokenOptions)) (browserProvider, *httptest.ResponseRecorder, error) {
	t.Helper()

	tenant := &fakeTenant{tweak: tweak}
	p := newBrowserProvider(t, map[string]http.HandlerFunc{"/oauth/token": tenant.serveToken(t)})

	// The handler only runs once a callback reaches it, by which point both of these are set.
	tenant.issuer = p.issuer.String()

	q, stateCookie := p.login(t, "/login?return_to="+url.QueryEscape(returnTo))
	tenant.authorize = q

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/callback?code="+testAuthCode+"&state="+url.QueryEscape(q.Get("state")), nil)
	req.AddCookie(stateCookie)

	return p, rec, p.CallbackHandler()(t.Context(), rec, req, nil)
}

// TestBrowserLoginRoundTrip walks a login through to an authenticated request.
func TestBrowserLoginRoundTrip(t *testing.T) {
	t.Parallel()

	const imagePath = "/image/abc/v1.12.0/metal-amd64.iso"

	p, rec, err := signIn(t, imagePath, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, imagePath, rec.Header().Get("Location"), "the user must land where they were headed")

	cleared := auth0.FindCookie(rec, "if_auth_state")
	require.NotNil(t, cleared, "the state cookie is single use and must be spent")
	require.Empty(t, cleared.Value)

	session := auth0.FindCookie(rec, "if_session")
	require.NotNil(t, session, "a completed login must establish a session")

	// The point of the round trip: the cookie the callback issued authenticates a request, as
	// the identity carried by the access token it was exchanged for.
	var username string

	authed := httptest.NewRequestWithContext(t.Context(), http.MethodGet, imagePath, nil)
	authed.Header.Set("Accept", "text/html")
	authed.AddCookie(session)

	rec = httptest.NewRecorder()
	require.NoError(t, p.Middleware(captureHandler(&username))(t.Context(), rec, authed, nil))

	require.Equal(t, testOrgID, username, "the org_id of the exchanged access token is the principal")
	require.Empty(t, rec.Header().Get("Location"), "a signed-in browser must not be sent back to /login")
}

// TestBrowserLoginRejectsBadTokenSet: a token set that verifies as a JWT but does not belong
// to this login, or to this factory, must not become a session.
func TestBrowserLoginRejectsBadTokenSet(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		tweak func(access, id *testoidc.TokenOptions)
		name  string
	}{
		{
			// A token set lifted from another login: correctly signed, wrong flow. The nonce is
			// what ties the ID token to the /login that started this one.
			name:  "id token nonce from another login",
			tweak: func(_, id *testoidc.TokenOptions) { id.Nonce = "nonce-from-another-login" },
		},
		{
			// An access token for another API is rejected on every later request, so accepting
			// it would hand out a session that authenticates nothing.
			name:  "access token for another audience",
			tweak: func(access, _ *testoidc.TokenOptions) { access.Audience = []string{"https://other-api.test"} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, rec, err := signIn(t, "/", tc.tweak)

			// Terminal: a bounce through /login returns the same token set off Auth0's SSO session.
			require.True(t, xerrors.TagIs[enterrors.RespondedTag](err), "got %v", err)
			require.Equal(t, http.StatusForbidden, rec.Code)

			requireNoSession(t, rec)
		})
	}
}
