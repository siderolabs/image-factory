// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

// auth0_test.go: integration tests for the Auth0 authentication provider.
// Spins up a full image-factory instance backed by an in-process OIDC server
// and verifies that authenticated and unauthenticated requests are handled correctly.

package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/internal/integration/testoidc"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

const (
	auth0TestDomain   = "test.auth0.com"
	auth0TestAudience = "https://image-factory.test"
	auth0TestKeyID    = "integration-test-key"

	auth0OrgA = "org_aaa111"
	auth0OrgB = "org_bbb222"
)

// auth0TokenFixtures holds pre-signed tokens for a test run.
type auth0TokenFixtures struct {
	orgAToken    string
	orgBToken    string
	expiredToken string
	noOrgToken   string
}

// auth0SignToken creates and signs a JWT using the shared test helper.
func auth0SignToken(t *testing.T, privateKey *rsa.PrivateKey, iss, aud, orgID string, exp time.Time) string {
	t.Helper()

	return testoidc.SignToken(t, privateKey, auth0TestKeyID, iss, "user|test", aud, orgID, exp)
}

// setupEnterpriseAuth0 configures opts for the auth0 provider using an
// in-process OIDC server. Returns token fixtures for use in test assertions.
func setupEnterpriseAuth0(t *testing.T, opts *cmd.Options) auth0TokenFixtures {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serverURL := testoidc.StartServer(t, privateKey, auth0TestKeyID)

	opts.Authentication.Enabled = true
	opts.Authentication.Provider = "auth0"
	opts.Authentication.Auth0.Domain = auth0TestDomain
	opts.Authentication.Auth0.Audience = auth0TestAudience
	opts.Authentication.Auth0.IssuerURLOverride = serverURL

	now := time.Now()

	return auth0TokenFixtures{
		orgAToken:    auth0SignToken(t, privateKey, serverURL, auth0TestAudience, auth0OrgA, now.Add(time.Hour)),
		orgBToken:    auth0SignToken(t, privateKey, serverURL, auth0TestAudience, auth0OrgB, now.Add(time.Hour)),
		expiredToken: auth0SignToken(t, privateKey, serverURL, auth0TestAudience, auth0OrgA, now.Add(-time.Hour)),
		noOrgToken:   auth0SignToken(t, privateKey, serverURL, auth0TestAudience, "", now.Add(time.Hour)),
	}
}

func TestIntegrationAuth0(t *testing.T) {
	if !enterprise.Enabled() {
		t.Skip("enterprise features are disabled")
	}

	options := cmd.DefaultOptions
	options.Cache.OCI = cacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_auth0"

	fixtures := setupEnterpriseAuth0(t, &options)

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	t.Run("Enforcement", func(t *testing.T) {
		t.Parallel()

		testAuth0Enforcement(ctx, t, baseURL, fixtures)
	})

	t.Run("NodeFlow", func(t *testing.T) {
		t.Parallel()

		testAuth0NodeFlow(ctx, t, baseURL, fixtures)
	})

	t.Run("Ownership", func(t *testing.T) {
		t.Parallel()

		testAuth0Ownership(ctx, t, baseURL, fixtures)
	})

	t.Run("BrowserRedirect", func(t *testing.T) {
		t.Parallel()

		testAuth0BrowserRedirect(ctx, t, baseURL)
	})
}

// testAuth0Enforcement verifies that protected endpoints reject missing/invalid
// tokens and accept a valid JWT as a Bearer token.
func testAuth0Enforcement(ctx context.Context, t *testing.T, baseURL string, fx auth0TokenFixtures) {
	t.Helper()

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v2/"},
		{http.MethodPost, "/schematics"},
		{http.MethodGet, "/schematics/" + nonexistentSchematicID},
		{http.MethodGet, "/"},
	}

	t.Run("NoCredentials_401", func(t *testing.T) {
		t.Parallel()

		for _, ep := range protectedEndpoints {
			t.Run(ep.method+"_"+ep.path, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequestWithContext(ctx, ep.method, baseURL+ep.path, bytes.NewReader([]byte("customization: {}")))
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

				assertRequiresAuth(t, resp)
			})
		}
	})

	t.Run("ExpiredToken_401", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+fx.expiredToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("NoOrgID_401", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+fx.noOrgToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("ValidBearerToken_200", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+fx.orgAToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// testAuth0NodeFlow verifies that a JWT sent as the Basic Auth password (the
// way Talos injects a node token as a registry credential) is accepted.
func testAuth0NodeFlow(ctx context.Context, t *testing.T, baseURL string, fx auth0TokenFixtures) {
	t.Helper()

	t.Run("JWTAsBasicAuthPassword_200", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		// Username is ignored; JWT goes in the password field.
		req.SetBasicAuth("node", fx.orgAToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("JWTAsBasicAuthPassword_SchematicCreate_200", func(t *testing.T) {
		t.Parallel()

		c, err := client.New(baseURL, client.WithBasicAuth("node", fx.orgAToken))
		require.NoError(t, err)

		_, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)
	})
}

// testAuth0Ownership verifies that org-scoped schematics are not accessible
// to tokens from a different org.
func testAuth0Ownership(ctx context.Context, t *testing.T, baseURL string, fx auth0TokenFixtures) {
	t.Helper()

	// Create a schematic as org A.
	c, err := client.New(baseURL, client.WithBasicAuth("node", fx.orgAToken))
	require.NoError(t, err)

	schematicID, _, err := c.SchematicCreate(ctx, schematicpkg.Schematic{})
	require.NoError(t, err)

	schematicURL := baseURL + "/schematics/" + schematicID

	t.Run("Owner_200", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, schematicURL, nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+fx.orgAToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("DifferentOrg_403", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, schematicURL, nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+fx.orgBToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("NoCredentials_401", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, schematicURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})
}

// testAuth0BrowserRedirect verifies that browser requests without credentials
// get a redirect to /login rather than a 401, and that /login returns 404 when
// browser login is not configured (no ClientID/SessionKey).
func testAuth0BrowserRedirect(ctx context.Context, t *testing.T, baseURL string) {
	t.Helper()

	// Browser login is NOT configured in this test (no ClientID/SessionKey),
	// so /login and /logout should be 404.
	t.Run("LoginRoute_404_WhenBrowserLoginDisabled", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{"/login", "/logout"} {
			t.Run(path, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
				require.NoError(t, err)

				noRedirectClient := &http.Client{
					CheckRedirect: func(*http.Request, []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}

				resp, err := noRedirectClient.Do(req)
				require.NoError(t, err)

				t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			})
		}
	})

	// Without browser login, a browser request to a protected endpoint
	// should return 401 (no redirect available).
	t.Run("BrowserRequest_NoLogin_401", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
		require.NoError(t, err)

		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		noRedirectClient := &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		resp, err := noRedirectClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
