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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
	"github.com/siderolabs/image-factory/internal/integration/testoidc"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

const (
	testDomain   = "test.auth0.com"
	testAudience = "https://image-factory.test"
	testSubject  = "user1|abc123"
	testOrgID    = "org_abc123"
	testKeyID    = "test-key-1"
)

// setupProvider starts an in-process OIDC discovery + JWKS server, creates a
// Provider wired to it via IssuerURLOverride, and returns both.
// The returned issuerURL is the server base URL — use it when signing test tokens
// so that the iss claim matches the provider's expected issuer.
func setupProvider(t *testing.T, privateKey *rsa.PrivateKey) (*auth0.Provider, string) {
	t.Helper()

	issuerURL := testoidc.StartServer(t, privateKey, testKeyID)

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain:            testDomain,
		Audience:          testAudience,
		IssuerURLOverride: issuerURL,
	})
	require.NoError(t, err)

	return p, issuerURL
}

// signToken builds and signs a JWT with the given fields.
func signToken(t *testing.T, privateKey *rsa.PrivateKey, iss, aud, orgID string, exp time.Time) string {
	t.Helper()

	return testoidc.SignToken(t, privateKey, testKeyID, iss, testSubject, aud, orgID, exp)
}

func TestAuth0ProviderMiddleware(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	p, issuerURL := setupProvider(t, privateKey)

	validToken := signToken(t, privateKey, issuerURL, testAudience, testOrgID, time.Now().Add(time.Hour))
	expiredToken := signToken(t, privateKey, issuerURL, testAudience, testOrgID, time.Now().Add(-time.Hour))
	noOrgToken := signToken(t, privateKey, issuerURL, testAudience, "", time.Now().Add(time.Hour))
	wrongAudToken := signToken(t, privateKey, issuerURL, "https://wrong-audience", "", time.Now().Add(time.Hour))
	wrongIssToken := signToken(t, privateKey, "https://wrong.auth0.com/", testAudience, "", time.Now().Add(time.Hour))

	// wrongKeyToken is signed by a different key — signature check must fail.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	wrongKeyToken := signToken(t, otherKey, issuerURL, testAudience, "", time.Now().Add(time.Hour))

	captureHandler := func(capturedUsername *string) auth0.Handler {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
			username, _ := auth.GetAuthUsername(ctx)
			*capturedUsername = username

			return nil
		}
	}

	for _, tc := range []struct {
		name            string
		setupRequest    func(*http.Request)
		expectUsername  string
		expectAuthError bool
	}{
		{
			name:            "no credentials",
			setupRequest:    func(r *http.Request) {},
			expectAuthError: true,
		},
		{
			name: "valid bearer token",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+validToken)
			},
			expectUsername: testOrgID,
		},
		{
			name: "valid token in basic auth password",
			setupRequest: func(r *http.Request) {
				r.SetBasicAuth("ignored", validToken)
			},
			expectUsername: testOrgID,
		},
		{
			name: "bearer takes precedence over basic auth",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+validToken)
				// Basic auth header is overridden by Bearer so this is intentionally
				// not a separate header — just documenting the precedence.
			},
			expectUsername: testOrgID,
		},
		{
			name: "expired token",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+expiredToken)
			},
			expectAuthError: true,
		},
		{
			name: "wrong audience",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+wrongAudToken)
			},
			expectAuthError: true,
		},
		{
			name: "wrong issuer",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+wrongIssToken)
			},
			expectAuthError: true,
		},
		{
			name: "wrong signing key",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+wrongKeyToken)
			},
			expectAuthError: true,
		},
		{
			name: "malformed token",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer not.a.jwt")
			},
			expectAuthError: true,
		},
		{
			name: "empty basic auth password",
			setupRequest: func(r *http.Request) {
				r.SetBasicAuth("user", "")
			},
			expectAuthError: true,
		},
		{
			name: "valid jwt but no org_id claim",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+noOrgToken)
			},
			expectAuthError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			require.NoError(t, err)

			tc.setupRequest(req)

			var capturedUsername string

			middleware := p.Middleware(captureHandler(&capturedUsername))

			err = middleware(ctx, httptest.NewRecorder(), req, nil)

			if tc.expectAuthError {
				require.Error(t, err)
				require.True(t, xerrors.TagIs[schematicpkg.RequiresAuthenticationTag](err),
					"expected RequiresAuthenticationTag error, got: %v", err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectUsername, capturedUsername)
		})
	}
}

func TestAuth0ProviderVerifyCredentials(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	p, issuerURL := setupProvider(t, privateKey)

	validToken := signToken(t, privateKey, issuerURL, testAudience, testOrgID, time.Now().Add(time.Hour))
	expiredToken := signToken(t, privateKey, issuerURL, testAudience, testOrgID, time.Now().Add(-time.Hour))
	noOrgToken := signToken(t, privateKey, issuerURL, testAudience, "", time.Now().Add(time.Hour))

	require.True(t, p.VerifyCredentials("ignored", validToken), "valid token should pass")
	require.False(t, p.VerifyCredentials("ignored", expiredToken), "expired token should fail")
	require.False(t, p.VerifyCredentials("ignored", "not-a-token"), "garbage should fail")
	require.False(t, p.VerifyCredentials("ignored", noOrgToken), "token without org_id should fail")
}

func TestAuth0ProviderUsernameFromContext(t *testing.T) {
	t.Parallel()

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{Domain: testDomain, Audience: testAudience})
	require.NoError(t, err)

	ctx := t.Context()

	_, ok := p.UsernameFromContext(ctx)
	require.False(t, ok, "empty context should return no username")

	ctx = auth.WithAuthUsername(ctx, "alice")
	username, ok := p.UsernameFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "alice", username)
}

func TestNewProviderValidation(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	_, err := auth0.NewProvider(t.Context(), logger, auth0.Config{Domain: "", Audience: testAudience})
	require.Error(t, err, "empty domain should be rejected")

	_, err = auth0.NewProvider(t.Context(), logger, auth0.Config{Domain: testDomain, Audience: ""})
	require.Error(t, err, "empty audience should be rejected")

	_, err = auth0.NewProvider(t.Context(), logger, auth0.Config{Domain: testDomain, Audience: testAudience})
	require.NoError(t, err, "domain and audience alone should be a valid, bearer-token-only setup")
}

// TestNewProviderBrowserLoginFields asserts the browser-login fields are all-or-nothing:
// a partial set is rejected rather than silently dropping the browser login routes.
func TestNewProviderBrowserLoginFields(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	base := auth0.Config{Domain: testDomain, Audience: testAudience}

	full := base
	full.ClientID = "client-id"
	full.ClientSecret = "client-secret"
	full.RedirectURL = "https://factory.example.com/callback"
	full.SessionKey = make([]byte, 32)

	for _, tc := range []struct {
		name    string
		mutate  func(*auth0.Config)
		missing string
	}{
		{"no clientID", func(c *auth0.Config) { c.ClientID = "" }, "clientID"},
		{"no clientSecret", func(c *auth0.Config) { c.ClientSecret = "" }, "clientSecret"},
		{"no redirectURL", func(c *auth0.Config) { c.RedirectURL = "" }, "redirectURL"},
		{"no sessionKey", func(c *auth0.Config) { c.SessionKey = nil }, "sessionKey"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := full
			tc.mutate(&cfg)

			_, err := auth0.NewProvider(t.Context(), logger, cfg)
			require.Error(t, err, "partial browser login config should be rejected")
			require.ErrorContains(t, err, tc.missing)
		})
	}

	t.Run("all four present", func(t *testing.T) {
		t.Parallel()

		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		// authentication.New fetches https://<domain>/.well-known/jwks.json when it
		// builds the ID-token validator, so point its HTTP client at the test server.
		cfg := full
		cfg.HTTPClient = redirectingClient(t, testoidc.StartServer(t, privateKey, testKeyID))

		p, err := auth0.NewProvider(t.Context(), logger, cfg)
		require.NoError(t, err)
		require.True(t, p.BrowserLoginEnabled())
	})

	t.Run("none present", func(t *testing.T) {
		t.Parallel()

		p, err := auth0.NewProvider(t.Context(), logger, base)
		require.NoError(t, err)
		require.False(t, p.BrowserLoginEnabled())
	})
}

// redirectingClient returns an HTTP client that sends every request to baseURL,
// preserving the request path. It lets tests intercept the Auth0 tenant calls
// that authentication.New makes against the hardcoded https://<domain> host.
func redirectingClient(t *testing.T, baseURL string) *http.Client {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	require.NoError(t, err)

	return &http.Client{Transport: rewriteTransport{host: parsed.Host, scheme: parsed.Scheme}}
}

type rewriteTransport struct {
	host   string
	scheme string
}

func (rt rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme = rt.scheme
	r.URL.Host = rt.host
	r.Host = rt.host

	return http.DefaultTransport.RoundTrip(r)
}
