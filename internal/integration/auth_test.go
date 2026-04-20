// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ory/dockertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// testAuthFrontend runs all auth sub-tests. Enforcement, public endpoint, and
// ownership tests use the provided factory; reload spins up its own instance
// because it needs direct control over the htpasswd file path.
// Skipped entirely when enterprise features are disabled.
func testAuthFrontend(ctx context.Context, t *testing.T, baseURL string) {
	if !enterprise.Enabled() {
		t.Skip("enterprise features are disabled")
	}

	t.Run("Enforcement", func(t *testing.T) {
		t.Parallel()

		testAuthEnforcement(ctx, t, baseURL)
	})

	t.Run("PublicEndpoints", func(t *testing.T) {
		t.Parallel()

		testPublicEndpoints(ctx, t, baseURL)
	})

	t.Run("Ownership", func(t *testing.T) {
		t.Parallel()

		testOwnership(ctx, t, baseURL)
	})

	t.Run("Reload", testAuthReload)
}

func testAuthEnforcement(ctx context.Context, t *testing.T, baseURL string) {
	// Protected endpoints: registry /v2/*, schematics, and UI wizard.
	// /healthz, /versions, and meta endpoints are public.
	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v2/"},
		{http.MethodHead, "/v2/"},
		{http.MethodGet, "/v2"},
		{http.MethodHead, "/v2"},
		{http.MethodPost, "/schematics"},
		{http.MethodGet, "/schematics/" + nonexistentSchematicID},
		{http.MethodGet, "/"},
		{http.MethodHead, "/"},
	}

	t.Run("NoCredentials", func(t *testing.T) {
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

	t.Run("IncorrectCredentials", func(t *testing.T) {
		t.Parallel()

		username, password := authCredentials()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		req.SetBasicAuth(username, password+"wrong")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("CorrectCredentials", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		addTestAuth(req)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("HealthzIsPublic", func(t *testing.T) {
		t.Parallel()

		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequestWithContext(ctx, method, baseURL+"/healthz", nil)
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"/healthz must always be reachable without credentials")
			})
		}
	})

	t.Run("V2AuthChallenge", func(t *testing.T) {
		t.Parallel()

		// OCI Distribution Spec: unauthenticated GET /v2/ → 401 with WWW-Authenticate
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)

		// Authenticated /v2/ must return 200
		req2, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		addTestAuth(req2)

		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err)

		t.Cleanup(func() { resp2.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp2.StatusCode)
	})
}

// testPublicEndpoints verifies that health, meta, and informational endpoints are
// reachable without credentials even when auth is active.
func testPublicEndpoints(ctx context.Context, t *testing.T, baseURL string) {
	publicEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/versions"},
		{http.MethodGet, "/secureboot/signing-cert.pem"},
		{http.MethodGet, "/oci/cosign/signing-key.pub"},
	}

	for _, ep := range publicEndpoints {
		t.Run(ep.method+"_"+ep.path, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, ep.method, baseURL+ep.path, nil)
			require.NoError(t, err)

			// deliberately NO auth
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

			assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
				"%s %s must be reachable without credentials", ep.method, ep.path)
		})
	}
}

// testAuthReload verifies that the provider hot-reloads the htpasswd file.
// It adds a new user and removes an existing one, then polls until the change
// propagates (up to 10 s - fsnotify usually fires within milliseconds).
func testAuthReload(t *testing.T) {
	t.Parallel()

	options := cmd.DefaultOptions
	options.Cache.OCI = cacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_auth_reload"

	// Write the initial htpasswd to a path we control.
	configDir := t.TempDir()
	htpasswdPath := filepath.Join(configDir, "htpasswd")

	require.NoError(t, os.WriteFile(htpasswdPath, htpasswdFile, 0o600))

	// Pre-configure auth so setupEnterprise won't overwrite our path.
	options.Authentication.Enabled = true
	options.Authentication.HTPasswdPath = htpasswdPath

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	checkStatus := func(username, password string) int {
		// Use /v2/ (registry discovery) - auth-protected endpoint that requires no body.
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
		require.NoError(t, err)

		req.SetBasicAuth(username, password)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		defer resp.Body.Close() //nolint:errcheck

		io.Copy(io.Discard, resp.Body) //nolint:errcheck

		return resp.StatusCode
	}

	// Verify initial state.
	require.Equal(t, http.StatusOK, checkStatus("alice", "alicetopsecret"),
		"alice must authenticate before reload")
	require.Equal(t, http.StatusUnauthorized, checkStatus("carol", "carolsecret"),
		"carol must not exist before reload")

	// Generate a fresh bcrypt hash for carol's password.
	carolHash, err := bcrypt.GenerateFromPassword([]byte("carolsecret"), bcrypt.MinCost)
	require.NoError(t, err)

	// New htpasswd: add carol, remove alice entirely.
	newContent := fmt.Sprintf("carol:%s\n", carolHash)

	require.NoError(t, os.WriteFile(htpasswdPath, []byte(newContent), 0o600))

	// Poll for up to 10 s - fsnotify normally reacts within a few milliseconds.
	deadline := time.Now().Add(10 * time.Second)
	carolAuthed := false

	for time.Now().Before(deadline) {
		if checkStatus("carol", "carolsecret") == http.StatusOK {
			carolAuthed = true

			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.True(t, carolAuthed, "carol should authenticate within 10 s of htpasswd update")
	require.Equal(t, http.StatusUnauthorized, checkStatus("alice", "alicetopsecret"),
		"alice must be rejected after removal from htpasswd")
}

// testOwnership verifies that owned schematics are only accessible to their creator.
// A schematic created by alice (via authenticated POST /schematics) should:
//   - be inaccessible to unauthenticated requests (401)
//   - be inaccessible to other authenticated users (403)
//   - be accessible to alice (200)
func testOwnership(ctx context.Context, t *testing.T, baseURL string) {
	// Create a schematic as alice.
	var ownedSchematicID string

	{
		c, err := client.New(baseURL, clientAuthCredentials()...)
		require.NoError(t, err)

		ownedSchematicID, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)
	}

	schematicURL := baseURL + "/schematics/" + ownedSchematicID

	t.Run("GetSchematic_NoCredentials_401", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, schematicURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("GetSchematic_WrongOwner_403", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, schematicURL, nil)
		require.NoError(t, err)

		req.SetBasicAuth("bob", "bobsecret")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("GetSchematic_Owner_200", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, schematicURL, nil)
		require.NoError(t, err)

		addTestAuth(req)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// testAuthS3NoRedirect asserts that the factory serves assets directly (no
// HTTP 302) when both S3 caching and authentication are active.
// S3 credentials must already be set in the environment by the caller.
func testAuthS3NoRedirect(t *testing.T, pool *dockertest.Pool) {
	options := cmd.DefaultOptions
	options.Cache.OCI = signingCacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_auth_s3_no_redirect"

	options.Cache.S3.Enabled = true
	options.Cache.S3.Bucket = "test-auth-s3"
	options.Cache.S3.Insecure = true
	options.Cache.S3.Endpoint = setupS3(t, pool, options.Cache.S3.Bucket)

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	// Ensure schematic exists.
	{
		c, err := client.New(baseURL, clientAuthCredentials()...)
		require.NoError(t, err)

		_, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)
	}

	// First download - builds and caches the asset in S3.
	resp := downloadAsset(ctx, t, baseURL, emptySchematicID, "v1.9.4", "kernel-amd64")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	// Second download - asset is in S3, but auth is active: must NOT redirect.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/image/"+emptySchematicID+"/v1.9.4/kernel-amd64", nil)
	require.NoError(t, err)

	addTestAuth(req)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("unexpected S3 redirect to %s - auth active, factory must serve directly", req.URL)
		},
	}

	resp2, err := noRedirectClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() { resp2.Body.Close() }) //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

// testAuthCDNNoRedirect asserts that the factory never redirects to CDN URLs
// when authentication is active. CDN URLs are fully public (no auth) so they
// must never be issued from an auth-gated factory.
// S3 credentials must already be set in the environment by the caller.
func testAuthCDNNoRedirect(t *testing.T, pool *dockertest.Pool) {
	options := cmd.DefaultOptions
	options.Cache.OCI = signingCacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_auth_cdn_no_redirect"

	options.Cache.S3.Enabled = true
	options.Cache.S3.Bucket = "test-auth-cdn"
	options.Cache.S3.Insecure = true
	options.Cache.S3.Endpoint = setupS3(t, pool, options.Cache.S3.Bucket)

	options.Cache.CDN.Enabled = true
	options.Cache.CDN.TrimPrefix = fmt.Sprintf("/%s", options.Cache.S3.Bucket)
	options.Cache.CDN.Host = setupMockCDN(t, pool, options.Cache.S3.Endpoint, options.Cache.S3.Bucket)

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	{
		c, err := client.New(baseURL, clientAuthCredentials()...)
		require.NoError(t, err)

		_, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)
	}

	// Build and cache the asset.
	resp := downloadAsset(ctx, t, baseURL, emptySchematicID, "v1.9.4", "kernel-amd64")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	// Cached asset available via CDN, but auth active - must NOT redirect.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/image/"+emptySchematicID+"/v1.9.4/kernel-amd64", nil)
	require.NoError(t, err)

	addTestAuth(req)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("unexpected CDN redirect to %s - auth active, factory must never redirect to CDN", req.URL)
		},
	}

	resp2, err := noRedirectClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() { resp2.Body.Close() }) //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

// assertRequiresAuth checks that the response is 401 with WWW-Authenticate set,
// as required by RFC 7235 and the OCI Distribution Spec.
func assertRequiresAuth(t *testing.T, resp *http.Response) {
	t.Helper()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("WWW-Authenticate"),
		"401 response must include WWW-Authenticate header")
}
