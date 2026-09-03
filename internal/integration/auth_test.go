// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v4"
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

	t.Run("ImageReadTokens", func(t *testing.T) {
		t.Parallel()

		testImageReadTokens(ctx, t, baseURL)
	})

	t.Run("APITokens", func(t *testing.T) {
		t.Parallel()

		testAPITokens(ctx, t, baseURL)
	})
}

func createToken(ctx context.Context, t *testing.T, baseURL, body string) (status int, decoded map[string]any) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/tokens", strings.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	addTestAuth(req)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}

	require.NoError(t, json.Unmarshal(raw, &decoded))

	return resp.StatusCode, decoded
}

func getWithToken(ctx context.Context, t *testing.T, url, token string) int {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close() //nolint:errcheck

	return resp.StatusCode
}

func testAPITokens(ctx context.Context, t *testing.T, baseURL string) {
	t.Helper()

	c, err := client.New(baseURL, clientAuthCredentials()...)
	require.NoError(t, err)

	schematicID, _, err := c.SchematicCreate(ctx, schematicpkg.Schematic{})
	require.NoError(t, err)

	t.Run("RequiresAuth", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/tokens",
			strings.NewReader(`{"name":"n","scopes":["image:read"]}`))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("RejectsBadRequests", func(t *testing.T) {
		t.Parallel()

		for name, body := range map[string]string{
			"no name":        `{"scopes":["image:read"]}`,
			"long ephemeral": `{"scopes":["image:read"],"stored":false,"ttl":"8760h"}`,
			"short stored":   `{"name":"n","scopes":["image:read"],"stored":true,"ttl":"1m"}`,
			"no scopes":      `{"name":"n"}`,
			"unknown scope":  `{"name":"n","scopes":["root"]}`,
			"bad ttl":        `{"name":"n","scopes":["image:read"],"ttl":"forever"}`,
			"ttl too long":   `{"name":"n","scopes":["image:read"],"ttl":"9000h"}`,
			"malformed":      `not json`,
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				status, _ := createToken(ctx, t, baseURL, body)
				assert.Equal(t, http.StatusBadRequest, status)
			})
		}
	})

	t.Run("ImageReadScope", func(t *testing.T) {
		t.Parallel()

		status, created := createToken(ctx, t, baseURL, `{"name":"e2e-image-read","scopes":["image:read"]}`)
		require.Equal(t, http.StatusOK, status)

		token, _ := created["token"].(string)
		require.NotEmpty(t, token)
		assert.Equal(t, true, created["stored"], "a create that does not say otherwise is recorded")

		assert.Equal(t, http.StatusOK, getWithToken(ctx, t, baseURL+"/v2/", token))
		assert.Equal(t, http.StatusOK,
			getWithToken(ctx, t, baseURL+"/image/"+schematicID+"/v1.9.0/kernel-amd64", token))

		assert.Equal(t, http.StatusOK,
			getWithToken(ctx, t, baseURL+"/pxe/"+schematicID+"/v1.9.0/metal-amd64", token))

		for _, path := range []string{
			"/schematics/" + schematicID,
			"/tokens",
		} {
			assert.Equal(t, http.StatusUnauthorized, getWithToken(ctx, t, baseURL+path, token),
				"a pull token must not reach %s", path)
		}

		// A stored token travels in a query string like an ephemeral one: what reaches a URL is
		// already in the access logs in front of the factory, and a revocable credential is the
		// better one to have taken that risk with.
		assert.Equal(t, http.StatusOK,
			getWithToken(ctx, t, baseURL+"/image/"+schematicID+"/v1.9.0/kernel-amd64?token="+token, ""))
	})

	t.Run("SchematicScope", func(t *testing.T) {
		t.Parallel()

		status, created := createToken(ctx, t, baseURL, `{"name":"e2e-schematic","scopes":["schematic:create","schematic:read","report:read"]}`)
		require.Equal(t, http.StatusOK, status)

		token, _ := created["token"].(string)
		require.NotEmpty(t, token)

		sc, err := client.New(baseURL, client.WithBearerToken(token))
		require.NoError(t, err)

		ownID, _, err := sc.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, getWithToken(ctx, t, baseURL+"/schematics/"+ownID, token))

		assert.Equal(t, http.StatusUnauthorized,
			getWithToken(ctx, t, baseURL+"/image/"+ownID+"/v1.9.0/kernel-amd64", token))
	})

	t.Run("ListAndRevoke", func(t *testing.T) {
		t.Parallel()

		status, created := createToken(ctx, t, baseURL, `{"name":"e2e-revoke","scopes":["image:read"]}`)
		require.Equal(t, http.StatusOK, status)

		token, _ := created["token"].(string)
		id, _ := created["id"].(string)
		require.NotEmpty(t, id)

		require.Equal(t, http.StatusOK, getWithToken(ctx, t, baseURL+"/v2/", token))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/tokens", nil)
		require.NoError(t, err)

		addTestAuth(req)

		listResp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { listResp.Body.Close() }) //nolint:errcheck

		require.Equal(t, http.StatusOK, listResp.StatusCode)

		var listed struct {
			Tokens []struct {
				ID     string   `json:"id"`
				Name   string   `json:"name"`
				Scopes []string `json:"scopes"`
			} `json:"tokens"`
		}

		require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))

		found := false

		for _, tok := range listed.Tokens {
			if tok.ID == id {
				found = true

				assert.Equal(t, "e2e-revoke", tok.Name)
				assert.Equal(t, []string{"image:read"}, tok.Scopes)
			}
		}

		assert.True(t, found, "the minted token should appear in the listing")

		revokeReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			baseURL+"/tokens/"+id+"/revoke", nil)
		require.NoError(t, err)

		addTestAuth(revokeReq)

		revokeResp, err := http.DefaultClient.Do(revokeReq)
		require.NoError(t, err)

		t.Cleanup(func() { revokeResp.Body.Close() }) //nolint:errcheck

		require.Equal(t, http.StatusNoContent, revokeResp.StatusCode)

		assert.Eventually(t, func() bool {
			return getWithToken(ctx, t, baseURL+"/v2/", token) == http.StatusUnauthorized
		}, time.Minute, time.Second, "a revoked token should stop working")
	})

	t.Run("ClientDownloadToken", func(t *testing.T) {
		t.Parallel()

		token, err := c.DownloadToken(ctx, time.Hour)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		assert.Equal(t, http.StatusOK,
			getWithToken(ctx, t, baseURL+"/image/"+schematicID+"/v1.9.0/kernel-amd64?token="+token, ""))
	})

	t.Run("RetiredDownloadTokenRoute", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/download-token?ttl=1h", nil)
		require.NoError(t, err)

		addTestAuth(req)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "the alias is gone; /tokens covers it")
	})

	t.Run("EphemeralTokenIsNotListed", func(t *testing.T) {
		t.Parallel()

		status, created := createToken(ctx, t, baseURL, `{"scopes":["image:read"],"stored":false}`)
		require.Equal(t, http.StatusOK, status)

		assert.Equal(t, false, created["stored"], "the caller asked for a token the factory does not record")

		token, _ := created["token"].(string)
		require.NotEmpty(t, token)

		assert.Equal(t, http.StatusOK,
			getWithToken(ctx, t, baseURL+"/image/"+schematicID+"/v1.9.0/kernel-amd64?token="+token, ""))
	})

	t.Run("AdminScopeIsNotMintableOverHTTP", func(t *testing.T) {
		t.Parallel()

		// The caller here is a full htpasswd credential, which mints anything else it likes.
		status, _ := createToken(ctx, t, baseURL, `{"name":"e2e-admin","scopes":["admin"]}`)
		assert.Equal(t, http.StatusBadRequest, status, "the bootstrap credential is subcommand-only")
	})

	t.Run("MintingTokenIsRefusedFromQueryString", func(t *testing.T) {
		t.Parallel()

		// Short enough to be ephemeral, which is what would otherwise let it into a URL.
		status, created := createToken(ctx, t, baseURL, `{"scopes":["token:read"],"stored":false,"ttl":"1h"}`)
		require.Equal(t, http.StatusOK, status)

		minter, _ := created["token"].(string)
		require.NotEmpty(t, minter)

		assert.Equal(t, http.StatusOK, getWithToken(ctx, t, baseURL+"/tokens", minter),
			"the header still works")

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/tokens?token="+minter, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("MintingForAnotherIdentityNeedsAdmin", func(t *testing.T) {
		t.Parallel()

		// The caller is a full htpasswd credential, the most authority the API recognizes.
		status, _ := createToken(ctx, t, baseURL, `{"name":"e2e-other","scopes":["image:read"],"subject":"org_someone_else"}`)
		assert.Equal(t, http.StatusForbidden, status, "a tenant must not mint into another tenant")

		// Naming your own identity is not a cross-tenant mint, so it is allowed.
		self, _ := authCredentials()

		status, created := createToken(ctx, t, baseURL,
			fmt.Sprintf(`{"name":"e2e-self","scopes":["image:read"],"subject":%q}`, self))
		require.Equal(t, http.StatusOK, status)
		assert.Equal(t, self, created["org_id"])
	})

	t.Run("DelegationCeilingCannotEscalate", func(t *testing.T) {
		t.Parallel()

		status, created := createToken(ctx, t, baseURL,
			`{"name":"e2e-minter","scopes":["token:issue"],"issuable_scopes":["image:read"]}`)
		require.Equal(t, http.StatusOK, status)

		minter, _ := created["token"].(string)
		require.NotEmpty(t, minter)

		mint := func(body string) int {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/tokens",
				strings.NewReader(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+minter)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close() //nolint:errcheck

			return resp.StatusCode
		}

		assert.Equal(t, http.StatusOK, mint(`{"name":"x","scopes":["image:read"]}`))
		assert.Equal(t, http.StatusForbidden, mint(`{"name":"x","scopes":["token:issue"]}`))
		assert.Equal(t, http.StatusForbidden, mint(`{"name":"x","scopes":["report:read"]}`))
	})
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
		{http.MethodGet, "/.well-known/jwks.json"},
	}

	for _, ep := range publicEndpoints {
		t.Run(ep.method+"_"+ep.path, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, ep.method, baseURL+ep.path, nil)
			require.NoError(t, err)

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

		ownedSchematicID, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
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

	t.Run("PostSchematic_OwnerMismatch_403", func(t *testing.T) {
		t.Parallel()

		c, err := client.New(baseURL, clientAuthCredentials()...)
		require.NoError(t, err)

		_, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{Owner: "bob"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 403")
	})
}

// testImageReadTokens verifies the download flow of an ephemeral API token: create a schematic,
// mint an ephemeral image:read token with auth, then download using the token alone (no auth headers).
func testImageReadTokens(ctx context.Context, t *testing.T, baseURL string) {
	t.Helper()

	// Create a schematic with auth.
	c, err := client.New(baseURL, clientAuthCredentials()...)
	require.NoError(t, err)

	schematicID, _, err := c.SchematicCreate(ctx, schematicpkg.Schematic{})
	require.NoError(t, err)

	t.Run("MintRequiresAuth", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/tokens",
			strings.NewReader(`{"scopes":["image:read"],"stored":false}`))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("RequestedTTL", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name       string
			ttl        string
			expectCode int
		}{
			{name: "in range", ttl: "1h", expectCode: http.StatusOK},
			{name: "below min", ttl: "1s", expectCode: http.StatusBadRequest},
			{name: "above ephemeral maximum", ttl: "24h", expectCode: http.StatusBadRequest},
			{name: "not a duration", ttl: "forever", expectCode: http.StatusBadRequest},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				status, created := createToken(ctx, t, baseURL,
					fmt.Sprintf(`{"scopes":["image:read"],"stored":false,"ttl":%q}`, test.ttl))

				require.Equal(t, test.expectCode, status)

				if test.expectCode != http.StatusOK {
					return
				}

				token, _ := created["token"].(string)
				assert.NotEmpty(t, token)
			})
		}
	})

	t.Run("TokenAndDownload", func(t *testing.T) {
		t.Parallel()

		token := getImageReadToken(ctx, t, baseURL)
		downloadURL := baseURL + "/image/" + schematicID + "/v1.9.0/kernel-amd64?token=" + token

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("TokenReusableAcrossFiles", func(t *testing.T) {
		t.Parallel()

		token := getImageReadToken(ctx, t, baseURL)

		// Same token works for multiple files under the same schematic.
		for _, path := range []string{"kernel-amd64", "cmdline-metal-amd64"} {
			downloadURL := baseURL + "/image/" + schematicID + "/v1.9.0/" + path + "?token=" + token

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
			require.NoError(t, err)

			// deliberately NO auth
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

			assert.Equal(t, http.StatusOK, resp.StatusCode,
				"token must be accepted for %s", path)
		}
	})

	t.Run("TamperedTokenRejected", func(t *testing.T) {
		t.Parallel()

		tampered := tamperSignature(t, getImageReadToken(ctx, t, baseURL))
		downloadURL := baseURL + "/image/" + schematicID + "/v1.9.0/kernel-amd64?token=" + tampered

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("CrossOwnerRejected", func(t *testing.T) {
		t.Parallel()

		// alice's token should not access bob's schematic.
		bobClient, err := client.New(baseURL, client.WithBasicAuth("bob", "bobsecret"))
		require.NoError(t, err)

		bobSchematicID, _, err := bobClient.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)

		// Get alice's image-read token.
		aliceToken := getImageReadToken(ctx, t, baseURL)
		downloadURL := baseURL + "/image/" + bobSchematicID + "/v1.9.0/kernel-amd64?token=" + aliceToken

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("TokenRejectedOnWrite", func(t *testing.T) {
		t.Parallel()

		token := getImageReadToken(ctx, t, baseURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			baseURL+"/tokens?token="+token, strings.NewReader(`{"scopes":["image:read"],"stored":false}`))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})

	t.Run("Scopes", func(t *testing.T) {
		t.Parallel()

		testImageReadTokenScopes(ctx, t, baseURL, schematicID)
	})

	t.Run("PXE", func(t *testing.T) {
		t.Parallel()

		testImageReadTokenPXE(ctx, t, baseURL, schematicID)
	})

	t.Run("JWKSEndpoint", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/.well-known/jwks.json", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var doc struct {
			Keys []json.RawMessage `json:"keys"`
		}

		require.NoError(t, json.Unmarshal(body, &doc))
		assert.NotEmpty(t, doc.Keys)
	})
}

func testImageReadTokenScopes(ctx context.Context, t *testing.T, baseURL, schematicID string) {
	t.Helper()

	t.Run("AcceptedInAuthorizationHeader", func(t *testing.T) {
		t.Parallel()

		token := getImageReadToken(ctx, t, baseURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			baseURL+"/image/"+schematicID+"/v1.9.0/kernel-amd64", nil)
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("RejectedOutsideItsScope", func(t *testing.T) {
		t.Parallel()

		token := getImageReadToken(ctx, t, baseURL)

		for _, target := range []string{
			"/v2/siderolabs/imager/manifests/latest",
			"/schematics/" + schematicID,
		} {
			t.Run(target, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+target, nil)
				require.NoError(t, err)

				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

				assertRequiresAuth(t, resp)
			})
		}
	})
}

// testImageReadTokenPXE verifies that /pxe accepts an image-read token and forwards that
// same token into the asset URLs of the script it returns, so a boot needs no
// credential of its own anywhere.
func testImageReadTokenPXE(ctx context.Context, t *testing.T, baseURL, schematicID string) {
	t.Helper()

	const talosVersion = "v1.11.0"

	for _, test := range []struct {
		name       string
		path       string
		directives []string
		inHeader   bool
	}{
		{name: "standard", path: "metal-amd64", directives: []string{"kernel", "initrd"}},
		{name: "secureboot", path: "metal-amd64-secureboot", directives: []string{"kernel"}},
		{name: "token in header", path: "metal-amd64", directives: []string{"kernel", "initrd"}, inHeader: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			token := getImageReadToken(ctx, t, baseURL)

			scriptURL := baseURL + "/pxe/" + schematicID + "/" + talosVersion + "/" + test.path
			if !test.inHeader {
				scriptURL += "?token=" + token
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
			require.NoError(t, err)

			if test.inHeader {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"),
				"the script body is a bearer credential")

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			script := string(body)

			if _, password := authCredentials(); password != "" {
				assert.NotContains(t, script, password,
					"the token path must not embed userinfo credentials")
			}

			assetURLs := pxeAssetURLs(t, script, test.directives...)
			require.Len(t, assetURLs, len(test.directives))

			for _, assetURL := range assetURLs {
				parsed, err := url.Parse(assetURL)
				require.NoError(t, err)

				assert.Equal(t, token, parsed.Query().Get("token"))

				// The forwarded token has to actually authenticate the asset fetch,
				// which is the only assertion proving the script can boot.
				assetReq, err := http.NewRequestWithContext(ctx, http.MethodHead, assetURL, nil)
				require.NoError(t, err)

				assetResp, err := http.DefaultClient.Do(assetReq) //nolint:bodyclose // closed by the cleanup below
				require.NoError(t, err)

				t.Cleanup(func() { assetResp.Body.Close() }) //nolint:errcheck

				assert.Equal(t, http.StatusOK, assetResp.StatusCode, "fetching %s", assetURL)
			}
		})
	}

	t.Run("tampered", func(t *testing.T) {
		t.Parallel()

		tampered := tamperSignature(t, getImageReadToken(ctx, t, baseURL))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			baseURL+"/pxe/"+schematicID+"/"+talosVersion+"/metal-amd64?token="+tampered, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		assertRequiresAuth(t, resp)
	})
}

// getImageReadToken mints an ephemeral image:read API token, the kind a URL may carry.
func getImageReadToken(ctx context.Context, t *testing.T, baseURL string) string {
	t.Helper()

	status, created := createToken(ctx, t, baseURL, `{"scopes":["image:read"],"stored":false}`)
	require.Equal(t, http.StatusOK, status)

	token, _ := created["token"].(string)
	require.NotEmpty(t, token)
	require.Equal(t, false, created["stored"])

	return token
}

// testAuthS3NoRedirect asserts that the factory serves assets directly (no
// HTTP 302) when both S3 caching and authentication are active.
// S3 credentials must already be set in the environment by the caller.
func testAuthS3NoRedirect(t *testing.T, pool dockertest.Pool) {
	options := cmd.DefaultOptions
	options.Cache.OCI = signingCacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_auth_s3_no_redirect"

	options.Cache.S3.Enabled = true
	options.Cache.S3.Bucket = "test-auth-s3"
	options.Cache.S3.Insecure = true
	options.Cache.S3.Endpoint = setupS3(t, pool, options.Cache.S3.Bucket)

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	{
		c, err := client.New(baseURL, clientAuthCredentials()...)
		require.NoError(t, err)

		_, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)
	}

	resp := downloadAsset(ctx, t, baseURL, emptySchematicID, "v1.9.4", "kernel-amd64")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	io.Copy(io.Discard, resp.Body) //nolint:errcheck

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

func testAuthCDNNoRedirect(t *testing.T, pool dockertest.Pool) {
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

		_, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
		require.NoError(t, err)
	}

	resp := downloadAsset(ctx, t, baseURL, emptySchematicID, "v1.9.4", "kernel-amd64")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	io.Copy(io.Discard, resp.Body) //nolint:errcheck

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

func tamperSignature(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "expected a three-segment JWT")

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	signature[0] ^= 0xff
	parts[2] = base64.RawURLEncoding.EncodeToString(signature)

	return strings.Join(parts, ".")
}

func assertRequiresAuth(t *testing.T, resp *http.Response) {
	t.Helper()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("WWW-Authenticate"),
		"401 response must include WWW-Authenticate header")
}
