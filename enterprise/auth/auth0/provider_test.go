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
	"encoding/base64"
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

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
	"github.com/siderolabs/image-factory/internal/testoidc"
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
			// Bearer and Basic share one header, so both can only appear as two header
			// values, and Header.Get reads the first.
			name: "second authorization header value is ignored",
			setupRequest: func(r *http.Request) {
				r.Header.Add("Authorization", "Bearer "+validToken)
				r.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
					[]byte("ignored:"+expiredToken),
				))
			},
			expectUsername: testOrgID,
		},
		{
			// RFC 9110 makes the auth scheme case-insensitive.
			name: "lowercase bearer scheme",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "bearer "+validToken)
			},
			expectUsername: testOrgID,
		},
		{
			name: "unsupported authorization scheme",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Token "+validToken)
			},
			expectAuthError: true,
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

// TestAuth0ProviderChallengeOrder pins Basic ahead of Bearer on a 401.
// containerd and go-containerregistry both authenticate with the first scheme they
// recognize, and only Basic is usable here: the Bearer realm is a description rather
// than a token endpoint, while the machine path carries the JWT as the Basic password.
func TestAuth0ProviderChallengeOrder(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	p, _ := setupProvider(t, privateKey)

	for _, test := range []struct {
		setupFn func(*http.Request)
		name    string
	}{
		{name: "no credentials", setupFn: func(*http.Request) {}},
		{
			name:    "invalid token",
			setupFn: func(r *http.Request) { r.Header.Set("Authorization", "Bearer not-a-token") },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/", nil)
			test.setupFn(r)

			w := httptest.NewRecorder()

			next := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
				t.Fatal("handler must not run for an unauthenticated request")

				return nil
			}

			require.Error(t, p.Middleware(next)(t.Context(), w, r, nil))

			require.Equal(t, []string{
				`Basic realm="Image Factory Enterprise", charset="UTF-8"`,
				`Bearer realm="Image Factory Enterprise"`,
			}, w.Header().Values("WWW-Authenticate"))
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
	full.ExternalURL = "https://factory.example.com"
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

		p, err := auth0.NewProvider(t.Context(), logger, full)
		require.NoError(t, err)
		require.True(t, p.BrowserLoginEnabled())
		require.Equal(t, "/callback", p.CallbackPath())
	})

	t.Run("none present", func(t *testing.T) {
		t.Parallel()

		p, err := auth0.NewProvider(t.Context(), logger, base)
		require.NoError(t, err)
		require.False(t, p.BrowserLoginEnabled())
	})

	// The callback route and the logout returnTo are derived from these two, so a
	// value they cannot be derived from must fail at startup rather than 404 the
	// user after a successful Auth0 login.
	for _, tc := range []struct {
		name    string
		mutate  func(*auth0.Config)
		expects string
	}{
		{"relative redirectURL", func(c *auth0.Config) { c.RedirectURL = "/callback" }, "must be absolute"},
		{"redirectURL without path", func(c *auth0.Config) { c.RedirectURL = "https://factory.example.com" }, "must include a path"},
		{"redirectURL with root path", func(c *auth0.Config) { c.RedirectURL = "https://factory.example.com/" }, "must include a path"},
		{"no externalURL", func(c *auth0.Config) { c.ExternalURL = "" }, "externalURL is required"},
		// ":" and "*" are wildcard markers to httprouter, so these would register a
		// pattern rather than a literal route.
		{"redirectURL path with colon", func(c *auth0.Config) { c.RedirectURL = "https://factory.example.com/:cb" }, "must not contain"},
		{"redirectURL path with star", func(c *auth0.Config) { c.RedirectURL = "https://factory.example.com/*cb" }, "must not contain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := full
			tc.mutate(&cfg)

			_, err := auth0.NewProvider(t.Context(), logger, cfg)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expects)
		})
	}
}

// TestExtractToken covers which Authorization headers yield a token candidate.
// The Basic cases are the point: treating any password as a token feeds a browser's
// cached credential to the validator on every navigation, and the redirect to /login
// that follows resends the same header, which is a silent loop.
func TestExtractToken(t *testing.T) {
	t.Parallel()

	const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln"

	for _, test := range []struct {
		name     string
		header   string
		expected string
	}{
		{"absent", "", ""},
		{"bearer", "Bearer " + jwt, jwt},
		{"bearer lowercase", "bearer " + jwt, jwt},
		{"bearer mixed case", "BeArEr " + jwt, jwt},
		{"unsupported scheme", "Token " + jwt, ""},
		{"scheme only", "Bearer", ""},
		{"basic with JWT password", "Basic " + basicValue("ignored", jwt), jwt},
		{"basic with htpasswd password", "Basic " + basicValue("developer", "SideroTest"), ""},
		{"basic with empty password", "Basic " + basicValue("developer", ""), ""},
		{"basic with two segments", "Basic " + basicValue("x", "header.payload"), ""},
		{"basic with four segments", "Basic " + basicValue("x", "a.b.c.d"), ""},
		{"basic with empty segment", "Basic " + basicValue("x", "a..c"), ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if test.header != "" {
				r.Header.Set("Authorization", test.header)
			}

			require.Equal(t, test.expected, auth0.ExtractToken(r))
		})
	}
}

// basicValue builds the base64 credentials part of a Basic Authorization header.
func basicValue(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// TestSafeReturnTo asserts that the post-login redirect target is clamped to a
// site-relative path. /login is unauthenticated, so ?return_to= is attacker
// controlled and would otherwise be an open redirect off the back of a real login.
func TestSafeReturnTo(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", "/"},
		{"root", "/", "/"},
		{"path", "/image/abc/v1.12.0/metal-amd64.iso", "/image/abc/v1.12.0/metal-amd64.iso"},
		{"path with query", "/ui/wizard?step=2", "/ui/wizard?step=2"},
		{"absolute URL", "https://evil.example/x", "/x"},
		{"protocol relative", "//evil.example/x", "/x"},
		{"backslash", `/\evil.example`, "/%5Cevil.example"},
		{"backslash slash", `/\/evil.example`, "/%5C/evil.example"},
		{"javascript scheme", "javascript:alert(1)", "/"},
		{"relative", "foo/bar", "/"},
		{"userinfo", "https://user:pass@evil.example/x", "/x"},
		// url.Parse skips authority parsing when the rest starts with "///", so these
		// keep their leading slashes and stay protocol-relative to a browser.
		{"triple slash", "///evil.example", "/"},
		{"quadruple slash", "////evil.example", "/"},
		{"scheme with extra slashes", "http:////evil.example", "/"},
		{"scheme relative with path", "//evil.example", "/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := auth0.SafeReturnTo(test.input)

			require.Equal(t, test.expected, got)
			require.True(t, strings.HasPrefix(got, "/"), "result must be site-relative")
			require.False(t, strings.HasPrefix(got, "//"), "result must not be protocol-relative")
		})
	}
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
