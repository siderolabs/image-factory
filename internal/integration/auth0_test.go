// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

// auth0_test.go: integration tests for the Auth0 authentication provider.
// Spins up a full image-factory instance backed by an in-process OIDC server
// and verifies that authenticated and unauthenticated requests are handled correctly.

package integration_test

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/internal/testoidc"
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

	return testoidc.SignToken(t, privateKey, testoidc.TokenOptions{
		KeyID:    auth0TestKeyID,
		Issuer:   iss,
		Subject:  "user|test",
		Audience: []string{aud},
		OrgID:    orgID,
		Expiry:   exp,
	})
}

// setupEnterpriseAuth0 configures opts for the auth0 provider using an
// in-process OIDC server. Returns token fixtures for use in test assertions.
func setupEnterpriseAuth0(t *testing.T, opts *cmd.Options) auth0TokenFixtures {
	t.Helper()

	privateKey := testoidc.GenerateKey()

	serverURL := testoidc.StartServer(t, privateKey, auth0TestKeyID)

	opts.Authentication.Enabled = true
	opts.Authentication.Provider = cmd.AuthProviderAuth0
	opts.Authentication.Auth0.Domain = auth0TestDomain
	opts.Authentication.Auth0.Audience = auth0TestAudience
	opts.SetAuth0IssuerURL(serverURL)

	now := time.Now()

	return auth0TokenFixtures{
		orgAToken:    auth0SignToken(t, privateKey, serverURL, auth0TestAudience, auth0OrgA, now.Add(time.Hour)),
		orgBToken:    auth0SignToken(t, privateKey, serverURL, auth0TestAudience, auth0OrgB, now.Add(time.Hour)),
		expiredToken: auth0SignToken(t, privateKey, serverURL, auth0TestAudience, auth0OrgA, now.Add(-time.Hour)),
		noOrgToken:   auth0SignToken(t, privateKey, serverURL, auth0TestAudience, "", now.Add(time.Hour)),
	}
}

func TestIntegrationAuth0BrowserRoutes(t *testing.T) {
	if !enterprise.Enabled() {
		t.Skip("enterprise features are disabled")
	}

	options := cmd.DefaultOptions
	options.Cache.OCI = cacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_auth0_browser"
	options.HTTP.ExternalURL = "http://image-factory.test"
	options.Authentication.Auth0.ClientID = "browser-client"
	options.Authentication.Auth0.ClientSecret = "browser-secret"
	options.Authentication.Auth0.SessionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))

	setupEnterpriseAuth0(t, &options)

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, test := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "login", method: http.MethodGet, path: "/login", status: http.StatusFound},
		{name: "logout GET", method: http.MethodGet, path: "/logout", status: http.StatusOK},
		{name: "logout POST", method: http.MethodPost, path: "/logout", status: http.StatusSeeOther},
		{name: "callback without state", method: http.MethodGet, path: "/callback", status: http.StatusFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, test.method, baseURL+test.path, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

			assert.Equal(t, test.status, resp.StatusCode)
		})
	}

	t.Run("unauthenticated request classes", func(t *testing.T) {
		for _, test := range []struct {
			headers          map[string]string
			name             string
			method           string
			path             string
			body             string
			location         string
			hxRedirect       string
			status           int
			expectsChallenge bool
		}{
			{
				name:             "machine request",
				method:           http.MethodGet,
				path:             "/",
				status:           http.StatusUnauthorized,
				expectsChallenge: true,
			},
			{
				name:     "browser navigation",
				method:   http.MethodGet,
				path:     "/",
				status:   http.StatusSeeOther,
				location: "/login",
				headers: map[string]string{
					"Accept":         "text/html,application/xhtml+xml",
					"Sec-Fetch-Mode": "navigate",
				},
			},
			{
				name:       "htmx request",
				method:     http.MethodGet,
				path:       "/ui/version-doc?version=v1.11.0",
				status:     http.StatusUnauthorized,
				hxRedirect: "/login",
				headers: map[string]string{
					"Hx-Current-Url": "http://image-factory.test/",
					"Hx-Request":     "true",
				},
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequestWithContext(ctx, test.method, baseURL+test.path, bytes.NewBufferString(test.body))
				require.NoError(t, err)

				for key, value := range test.headers {
					req.Header.Set(key, value)
				}

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close() //nolint:errcheck

				assert.Equal(t, test.status, resp.StatusCode)
				assert.Equal(t, test.location, resp.Header.Get("Location"))
				assert.Equal(t, test.hxRedirect, resp.Header.Get("Hx-Redirect"))
				assert.NotEmpty(t, resp.Header.Get("Server"))
				assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))

				if test.expectsChallenge {
					assert.Equal(t, []string{`Basic realm="Image Factory Enterprise", charset="UTF-8"`}, resp.Header.Values("WWW-Authenticate"))
				} else {
					assert.Empty(t, resp.Header.Values("WWW-Authenticate"))
				}
			})
		}
	})
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

	t.Run("BasicPasswordFlow", func(t *testing.T) {
		t.Parallel()

		testAuth0BasicPasswordFlow(ctx, t, baseURL, fixtures)
	})

	t.Run("Ownership", func(t *testing.T) {
		t.Parallel()

		testAuth0Ownership(ctx, t, baseURL, fixtures)
	})

	t.Run("BrowserRoutesAbsent", func(t *testing.T) {
		t.Parallel()

		testAuth0BrowserRoutesAbsent(ctx, t, baseURL)
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

// testAuth0BasicPasswordFlow verifies that an Auth0 access token sent as a Basic password is
// accepted for clients such as OCI registries that cannot send Bearer credentials directly.
func testAuth0BasicPasswordFlow(ctx context.Context, t *testing.T, baseURL string, fx auth0TokenFixtures) {
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

// testAuth0BrowserRoutesAbsent pins that a bearer-token-only deployment serves no
// browser-login routes.
func testAuth0BrowserRoutesAbsent(ctx context.Context, t *testing.T, baseURL string) {
	t.Helper()

	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, path := range []string{"/login", "/logout"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
			require.NoError(t, err)

			resp, err := noRedirect.Do(req)
			require.NoError(t, err)

			t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}
