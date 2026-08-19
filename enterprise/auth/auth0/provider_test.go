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
		ClientID:          testClientID,
		ClientSecret:      testClientSecret,
		IssuerURLOverride: issuerURL,
	})
	require.NoError(t, err)

	return p, issuerURL
}

// signToken builds and signs a JWT with the given fields.
func signToken(t *testing.T, privateKey *rsa.PrivateKey, iss, aud, orgID string, exp time.Time) string {
	t.Helper()

	return testoidc.SignToken(t, privateKey, testoidc.TokenOptions{
		KeyID:    testKeyID,
		Issuer:   iss,
		Subject:  testSubject,
		Audience: []string{aud},
		OrgID:    orgID,
		Expiry:   exp,
	})
}

// captureHandler records the username the middleware authenticated as.
func captureHandler(capturedUsername *string) auth0.Handler {
	return func(ctx context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		*capturedUsername, _ = auth.GetAuthUsername(ctx)

		return nil
	}
}

// denyHandler fails the test if the middleware lets a request through.
func denyHandler(t *testing.T) auth0.Handler {
	return func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		t.Fatal("handler must not run for an unauthenticated request")

		return nil
	}
}

// requireNoSession checks a response did not sign anyone in. A cleared cookie is fine; a
// populated one is not.
func requireNoSession(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if session := auth0.FindCookie(rec, "if_session"); session != nil {
		require.Empty(t, session.Value, "this response must not establish a session")
	}
}

func TestAuth0ProviderMiddleware(t *testing.T) {
	t.Parallel()

	privateKey := testoidc.GenerateKey()

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

	// Auth0 issues a multi-valued aud; ours only has to be present, not alone.
	multiAudToken := testoidc.SignToken(t, privateKey, testoidc.TokenOptions{
		KeyID:    testKeyID,
		Issuer:   issuerURL,
		Subject:  testSubject,
		Audience: []string{testAudience, "https://other-api.test"},
		OrgID:    testOrgID,
		Expiry:   time.Now().Add(time.Hour),
	})

	// The JWKS server only publishes testKeyID, so this drives the refetch path.
	unknownKidToken := testoidc.SignToken(t, privateKey, testoidc.TokenOptions{
		KeyID:    "unknown-kid",
		Issuer:   issuerURL,
		Subject:  testSubject,
		Audience: []string{testAudience},
		OrgID:    testOrgID,
		Expiry:   time.Now().Add(time.Hour),
	})

	// Expired, but inside the clock-skew window; pins that the skew widens the window.
	recentlyExpiredToken := signToken(t, privateKey, issuerURL, testAudience, testOrgID, time.Now().Add(-10*time.Second))

	// nbf a minute out: go-oidc allows 5m, which clockSkew eats into. Pins that the two
	// stay compatible, so bumping clockSkew toward 5m fails here rather than in production.
	notYetValidToken := testoidc.SignToken(t, privateKey, testoidc.TokenOptions{
		KeyID:     testKeyID,
		Issuer:    issuerURL,
		Subject:   testSubject,
		Audience:  []string{testAudience},
		OrgID:     testOrgID,
		Expiry:    time.Now().Add(time.Hour),
		NotBefore: time.Now().Add(time.Minute),
	})

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
		{
			name: "audience is one of several",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+multiAudToken)
			},
			expectUsername: testOrgID,
		},
		{
			name: "unknown key id",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+unknownKidToken)
			},
			expectAuthError: true,
		},
		{
			name: "expired within the clock-skew window",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+recentlyExpiredToken)
			},
			expectUsername: testOrgID,
		},
		{
			name: "nbf within go-oidc's tolerance",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+notYetValidToken)
			},
			expectUsername: testOrgID,
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

// TestAuth0ProviderMachineScope covers the artifact-only allowlist applied to tokens
// carrying the machine scope.
func TestAuth0ProviderMachineScope(t *testing.T) {
	t.Parallel()

	const machineScope = "factory:machine"

	privateKey := testoidc.GenerateKey()
	issuerURL := testoidc.StartServer(t, privateKey, testKeyID)

	newProvider := func(scope string) *auth0.Provider {
		p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
			Domain:            testDomain,
			Audience:          testAudience,
			ClientID:          testClientID,
			ClientSecret:      testClientSecret,
			MachineScope:      scope,
			IssuerURLOverride: issuerURL,
		})
		require.NoError(t, err)

		return p
	}

	signMachineToken := func(opts testoidc.TokenOptions) string {
		opts.KeyID = testKeyID
		opts.Issuer = issuerURL
		opts.Subject = testSubject
		opts.Audience = []string{testAudience}
		opts.OrgID = testOrgID
		opts.Expiry = time.Now().Add(time.Hour)

		return testoidc.SignToken(t, privateKey, opts)
	}

	// Auth0 puts the grant in scope for client credentials and in permissions under RBAC.
	scopeClaimToken := signMachineToken(testoidc.TokenOptions{Scope: "openid " + machineScope})
	permissionsClaimToken := signMachineToken(testoidc.TokenOptions{Permissions: []string{machineScope}})
	humanToken := signMachineToken(testoidc.TokenOptions{Scope: "openid profile"})

	for _, test := range []struct {
		name         string
		token        string
		method       string
		path         string
		scope        string
		expectDenied bool
	}{
		{name: "image download", token: scopeClaimToken, method: http.MethodGet, path: "/image/abc/v1.9.0/metal-amd64.iso", scope: machineScope},
		{name: "image head", token: scopeClaimToken, method: http.MethodHead, path: "/image/abc/v1.9.0/metal-amd64.iso", scope: machineScope},
		{name: "registry root", token: scopeClaimToken, method: http.MethodGet, path: "/v2", scope: machineScope},
		{name: "registry manifest", token: scopeClaimToken, method: http.MethodGet, path: "/v2/installer/manifests/v1.9.0", scope: machineScope},
		{name: "permissions claim is read too", token: permissionsClaimToken, method: http.MethodGet, path: "/schematics/abc", scope: machineScope, expectDenied: true},

		{name: "schematic creation", token: scopeClaimToken, method: http.MethodPost, path: "/schematics", scope: machineScope, expectDenied: true},
		{name: "schematic read", token: scopeClaimToken, method: http.MethodGet, path: "/schematics/abc", scope: machineScope, expectDenied: true},
		{name: "pxe", token: scopeClaimToken, method: http.MethodGet, path: "/pxe/abc/v1.9.0/metal-amd64", scope: machineScope, expectDenied: true},
		{name: "sbom", token: scopeClaimToken, method: http.MethodGet, path: "/spdx/abc/v1.9.0/amd64", scope: machineScope, expectDenied: true},
		{name: "ui", token: scopeClaimToken, method: http.MethodGet, path: "/ui/schematics", scope: machineScope, expectDenied: true},

		// A token without the scope, and any token when no scope is configured, keep full access.
		{name: "token without the scope", token: humanToken, method: http.MethodPost, path: "/schematics", scope: machineScope},
		{name: "scope not configured", token: scopeClaimToken, method: http.MethodPost, path: "/schematics"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var capturedUsername string

			r := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			r.Header.Set("Authorization", "Bearer "+test.token)

			err := newProvider(test.scope).Middleware(captureHandler(&capturedUsername))(t.Context(), httptest.NewRecorder(), r, nil)

			if test.expectDenied {
				require.Error(t, err)
				require.Truef(t, xerrors.TagIs[schematicpkg.ForbiddenTag](err), "expected ForbiddenTag error, got: %v", err)

				// The frontend reads the principal back off the request to attribute the
				// denial in the audit log, since the wrapped handler never runs.
				principal, ok := auth.GetAuthUsername(r.Context())
				require.True(t, ok, "denied request should still carry the principal")
				require.Equal(t, testOrgID, principal)

				return
			}

			require.NoError(t, err)
			require.Equal(t, testOrgID, capturedUsername)
		})
	}
}

// TestAuth0ProviderChallengeOrder pins Basic ahead of Bearer on a 401, since OCI clients
// authenticate with the first scheme they recognize and only Basic is usable here.
func TestAuth0ProviderChallengeOrder(t *testing.T) {
	t.Parallel()

	p, _ := setupProvider(t, testoidc.GenerateKey())

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

			require.Error(t, p.Middleware(denyHandler(t))(t.Context(), w, r, nil))

			require.Equal(t, []string{
				`Basic realm="Image Factory Enterprise", charset="UTF-8"`,
				`Bearer realm="Image Factory Enterprise"`,
			}, w.Header().Values("WWW-Authenticate"))
		})
	}
}

// TestBearerOnlyDeniesBrowserWithChallenge pins deny's false branch: with no /login route to
// reach, an html-shaped client gets the same 401 as any other.
func TestBearerOnlyDeniesBrowserWithChallenge(t *testing.T) {
	t.Parallel()

	p, _ := setupProvider(t, testoidc.GenerateKey())
	require.False(t, p.BrowserLoginEnabled())

	for _, test := range []struct {
		headers map[string]string
		name    string
	}{
		{
			name:    "browser navigation",
			headers: map[string]string{"Accept": "text/html,application/xhtml+xml", "Sec-Fetch-Mode": "navigate"},
		},
		{
			name:    "htmx request",
			headers: map[string]string{"Hx-Request": "true", "Hx-Current-Url": "https://factory.example.com/"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			for k, v := range test.headers {
				r.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			require.Error(t, p.Middleware(denyHandler(t))(t.Context(), w, r, nil))

			require.Empty(t, w.Header().Get("Location"), "there is no /login to redirect to")
			require.Empty(t, w.Header().Get("Hx-Redirect"))
			require.NotEmpty(t, w.Header().Values("WWW-Authenticate"), "a denied browser still gets the challenge")
		})
	}
}

func TestAuth0ProviderUsernameFromContext(t *testing.T) {
	t.Parallel()

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain: testDomain, Audience: testAudience, ClientID: testClientID, ClientSecret: testClientSecret,
	})
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
	require.Error(t, err, "clientID and clientSecret are required even for a bearer-token-only setup")

	_, err = auth0.NewProvider(t.Context(), logger, auth0.Config{
		Domain: testDomain, Audience: testAudience, ClientID: testClientID, ClientSecret: testClientSecret,
	})
	require.NoError(t, err, "domain, audience, clientID and clientSecret is a valid, bearer-token-only setup")

	// The domain is the trust anchor, so anything that could point elsewhere is rejected
	// rather than trimmed into shape.
	for _, domain := range []string{
		"user@evil.example",
		"tenant.auth0.com/path",
		"tenant.auth0.com?q=1",
		"tenant.auth0.com#frag",
		"tenant.auth0.com/../evil.example",
	} {
		_, err = auth0.NewProvider(t.Context(), logger, auth0.Config{
			Domain: domain, Audience: testAudience, ClientID: testClientID, ClientSecret: testClientSecret,
		})
		require.Errorf(t, err, "domain %q should be rejected", domain)
	}

	// A scheme or trailing slash is what the Auth0 console shows, so accept it.
	for _, domain := range []string{"https://" + testDomain, "https://" + testDomain + "/", testDomain + "/"} {
		_, err = auth0.NewProvider(t.Context(), logger, auth0.Config{
			Domain: domain, Audience: testAudience, ClientID: testClientID, ClientSecret: testClientSecret,
		})
		require.NoErrorf(t, err, "domain %q should be accepted", domain)
	}

	// The result becomes the issuer, which is compared byte-for-byte against iss, so
	// accepting a mixed-case domain is not enough — it has to come back lowercased.
	for _, domain := range []string{"Tenant.Auth0.com", "HTTPS://Tenant.Auth0.com/"} {
		normalized, normalizeErr := auth0.NormalizeDomain(domain)
		require.NoErrorf(t, normalizeErr, "domain %q should be accepted", domain)
		require.Equalf(t, "tenant.auth0.com", normalized, "domain %q should normalize to lowercase", domain)
	}

	// Each of these trims to the empty host, which url.Parse accepts; without an explicit
	// check they yield issuer "https:///" and fail on the first token instead of at startup.
	for _, domain := range []string{"", "/", "https://", "http://", "https:///"} {
		_, normalizeErr := auth0.NormalizeDomain(domain)
		require.Errorf(t, normalizeErr, "domain %q should be rejected", domain)
	}
}

// TestNewProviderClientCredentialsRequired asserts clientID and clientSecret are required
// unconditionally — node-token management needs them independent of browser login.
func TestNewProviderClientCredentialsRequired(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	base := auth0.Config{Domain: testDomain, Audience: testAudience, ClientID: testClientID, ClientSecret: testClientSecret}

	for _, tc := range []struct {
		name    string
		mutate  func(*auth0.Config)
		missing string
	}{
		{"no clientID", func(c *auth0.Config) { c.ClientID = "" }, "clientID"},
		{"no clientSecret", func(c *auth0.Config) { c.ClientSecret = "" }, "clientSecret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := base
			tc.mutate(&cfg)

			_, err := auth0.NewProvider(t.Context(), logger, cfg)
			require.Error(t, err, "missing client credentials should be rejected even with browser login disabled")
			require.ErrorContains(t, err, tc.missing)
		})
	}

	t.Run("present, sessionKey absent", func(t *testing.T) {
		t.Parallel()

		p, err := auth0.NewProvider(t.Context(), logger, base)
		require.NoError(t, err, "clientID and clientSecret alone, with no sessionKey, is a valid management-only setup")
		require.False(t, p.BrowserLoginEnabled())
	})
}

// TestNewProviderSessionKeyOptional asserts sessionKey is the sole field that toggles browser
// login, independent of clientID/clientSecret which TestNewProviderClientCredentialsRequired
// covers.
func TestNewProviderSessionKeyOptional(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	full := auth0.Config{
		Domain:       testDomain,
		Audience:     testAudience,
		ClientID:     testClientID,
		ClientSecret: testClientSecret,
		ExternalURL:  "https://factory.example.com",
		SessionKey:   make([]byte, 32),
	}

	t.Run("sessionKey present", func(t *testing.T) {
		t.Parallel()

		p, err := auth0.NewProvider(t.Context(), logger, full)
		require.NoError(t, err)
		require.True(t, p.BrowserLoginEnabled())
		require.Equal(t, "/callback", p.CallbackPath(),
			"the callback route is fixed, and Auth0 is told externalURL + this path")
	})

	t.Run("sessionKey absent", func(t *testing.T) {
		t.Parallel()

		cfg := full
		cfg.SessionKey = nil

		p, err := auth0.NewProvider(t.Context(), logger, cfg)
		require.NoError(t, err)
		require.False(t, p.BrowserLoginEnabled())
	})

	// The callback URL and the logout returnTo are both derived from externalURL, so a bad
	// value must fail at startup rather than 404 the user after a successful Auth0 login.
	for _, tc := range []struct {
		name    string
		mutate  func(*auth0.Config)
		expects string
	}{
		{"no externalURL", func(c *auth0.Config) { c.ExternalURL = "" }, "externalURL must be absolute"},
		{"relative externalURL", func(c *auth0.Config) { c.ExternalURL = "/factory" }, "externalURL must be absolute"},
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
// The Basic cases are the point: treating any password as a token loops a browser's cached
// credential through /login forever.
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

// TestSafeReturnTo asserts the post-login redirect is clamped to a site-relative path;
// /login is unauthenticated, so ?return_to= is attacker controlled.
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
