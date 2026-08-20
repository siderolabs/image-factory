// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth0_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	auth0option "github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
)

const testMachineScope = "machine"

// newManagementProvider starts a fake Management API + token endpoint on mux and returns
// a Provider wired to it via IssuerURLOverride.
func newManagementProvider(t *testing.T, mux *http.ServeMux) *auth0.Provider {
	t.Helper()

	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusOK, map[string]any{
			"access_token": "fake-management-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	p, err := auth0.NewProviderWithManagementOptions(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain:            testDomain,
		Audience:          testAudience,
		MachineScope:      testMachineScope,
		ClientID:          testClientID,
		ClientSecret:      testClientSecret,
		IssuerURLOverride: server.URL,
	}, auth0option.WithMaxAttempts(1))
	require.NoError(t, err)

	return p
}

func jsonResponse(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

// requestedPage parses GET /api/v2/clients's offset-pagination page query parameter.
func requestedPage(t *testing.T, r *http.Request) int {
	t.Helper()

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	require.NoError(t, err)

	return page
}

// TestCreateNodeClientHappyPath pins the two requests a create issues: the client, then its grant.
func TestCreateNodeClientHappyPath(t *testing.T) {
	t.Parallel()

	var sawGrant bool

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/clients", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any

		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "my-node", body["name"])
		require.Equal(t, "non_interactive", body["app_type"])

		metadata, ok := body["client_metadata"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "image-factory", metadata["managed_by"])
		require.Equal(t, testOrgID, metadata["org_id"])
		require.NotEmpty(t, metadata["created_at"])

		jsonResponse(t, w, http.StatusCreated, map[string]any{
			"client_id":     "new-client-id",
			"client_secret": "new-client-secret",
		})
	})
	mux.HandleFunc("POST /api/v2/client-grants", func(w http.ResponseWriter, r *http.Request) {
		sawGrant = true

		var body map[string]any

		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "new-client-id", body["client_id"])
		require.Equal(t, testAudience, body["audience"])
		require.Equal(t, []any{testMachineScope}, body["scope"])

		jsonResponse(t, w, http.StatusCreated, map[string]any{})
	})

	p := newManagementProvider(t, mux)

	clientID, clientSecret, err := p.CreateNodeClient(t.Context(), testOrgID, "my-node")
	require.NoError(t, err)
	require.Equal(t, "new-client-id", clientID)
	require.Equal(t, "new-client-secret", clientSecret)
	require.True(t, sawGrant, "the new client must be authorized via a client-grant")
}

// TestCreateNodeClientRequiresMachineScope checks an unconfigured MachineScope is rejected
// before any Management API call, rather than falling back to an unrestricted grant.
func TestCreateNodeClientRequiresMachineScope(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/clients", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("no client should be created when MachineScope is unconfigured")
	})

	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusOK, map[string]any{
			"access_token": "fake-management-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	p, err := auth0.NewProvider(t.Context(), zaptest.NewLogger(t), auth0.Config{
		Domain:            testDomain,
		Audience:          testAudience,
		ClientID:          testClientID,
		ClientSecret:      testClientSecret,
		IssuerURLOverride: server.URL,
	})
	require.NoError(t, err)

	_, _, err = p.CreateNodeClient(t.Context(), testOrgID, "my-node")
	require.ErrorIs(t, err, auth0.ErrMachineScopeRequired)
}

// TestCreateNodeClientCreateFails checks no grant is attempted when client creation fails.
func TestCreateNodeClientCreateFails(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/clients", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusBadRequest, map[string]any{"message": "invalid name"})
	})
	mux.HandleFunc("POST /api/v2/client-grants", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("a client-grant must not be requested when client creation fails")
	})

	p := newManagementProvider(t, mux)

	_, _, err := p.CreateNodeClient(t.Context(), testOrgID, "my-node")
	require.ErrorContains(t, err, "create node client")
}

// TestCreateNodeClientGrantFails checks the orphaned client is cleaned up when the grant fails.
func TestCreateNodeClientGrantFails(t *testing.T) {
	t.Parallel()

	var sawDelete bool

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/clients", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusCreated, map[string]any{
			"client_id":     "newclientid",
			"client_secret": "new-client-secret",
		})
	})
	mux.HandleFunc("POST /api/v2/client-grants", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusInternalServerError, map[string]any{"message": "boom"})
	})
	mux.HandleFunc("DELETE /api/v2/clients/newclientid", func(w http.ResponseWriter, _ *http.Request) {
		sawDelete = true

		w.WriteHeader(http.StatusNoContent)
	})

	p := newManagementProvider(t, mux)

	_, _, err := p.CreateNodeClient(t.Context(), testOrgID, "my-node")
	require.ErrorContains(t, err, "authorize node client")
	require.True(t, sawDelete, "the orphaned client must be cleaned up when the grant fails")
}

// TestCreateNodeClientGrantFailsAndCleanupFails checks the grant error surfaces, not the cleanup error.
func TestCreateNodeClientGrantFailsAndCleanupFails(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/clients", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusCreated, map[string]any{
			"client_id":     "newclientid",
			"client_secret": "new-client-secret",
		})
	})
	mux.HandleFunc("POST /api/v2/client-grants", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusInternalServerError, map[string]any{"message": "boom"})
	})
	mux.HandleFunc("DELETE /api/v2/clients/newclientid", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusInternalServerError, map[string]any{"message": "cleanup also failed"})
	})

	p := newManagementProvider(t, mux)

	_, _, err := p.CreateNodeClient(t.Context(), testOrgID, "my-node")
	require.ErrorContains(t, err, "authorize node client")
	require.NotContains(t, err.Error(), "cleanup", "the cleanup failure is logged, not folded into the returned error")
}

// TestListNodeClientsFiltersByOrgAndOwner checks clients from another org or owner are excluded.
func TestListNodeClientsFiltersByOrgAndOwner(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "100", r.URL.Query().Get("per_page"))

		if requestedPage(t, r) > 0 {
			jsonResponse(t, w, http.StatusOK, map[string]any{"clients": []map[string]any{}})

			return
		}

		jsonResponse(t, w, http.StatusOK, map[string]any{
			"clients": []map[string]any{
				{
					"client_id": "mine",
					"name":      "mine-node",
					"client_metadata": map[string]any{
						"managed_by": "image-factory",
						"org_id":     testOrgID,
						"created_at": "2026-01-01T00:00:00Z",
					},
				},
				{
					"client_id": "other-org",
					"name":      "other-org-node",
					"client_metadata": map[string]any{
						"managed_by": "image-factory",
						"org_id":     "org_other",
						"created_at": "2026-01-01T00:00:00Z",
					},
				},
				{
					"client_id":       "unmanaged",
					"name":            "some-other-app",
					"client_metadata": map[string]any{},
				},
			},
		})
	})

	p := newManagementProvider(t, mux)

	clients, err := p.ListNodeClients(t.Context(), testOrgID)
	require.NoError(t, err)
	require.Len(t, clients, 1)
	require.Equal(t, "mine", clients[0].ClientID)
	require.Equal(t, "mine-node", clients[0].Name)
	require.False(t, clients[0].CreatedAt.IsZero())
}

// TestListNodeClientsPaginates checks results from successive offset pages are stitched together.
func TestListNodeClientsPaginates(t *testing.T) {
	t.Parallel()

	const pageCount = 3

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients", func(w http.ResponseWriter, r *http.Request) {
		page := requestedPage(t, r)

		if page >= pageCount {
			jsonResponse(t, w, http.StatusOK, map[string]any{"clients": []map[string]any{}})

			return
		}

		jsonResponse(t, w, http.StatusOK, map[string]any{
			"clients": []map[string]any{
				{
					"client_id": fmt.Sprintf("client-%d", page),
					"name":      fmt.Sprintf("node-%d", page),
					"client_metadata": map[string]any{
						"managed_by": "image-factory",
						"org_id":     testOrgID,
						"created_at": "2026-01-01T00:00:00Z",
					},
				},
			},
		})
	})

	p := newManagementProvider(t, mux)

	clients, err := p.ListNodeClients(t.Context(), testOrgID)
	require.NoError(t, err)
	require.Len(t, clients, pageCount)

	for i, c := range clients {
		require.Equal(t, fmt.Sprintf("client-%d", i), c.ClientID)
	}
}

// TestListNodeClientsRequestFails checks a failing page request surfaces as an error.
func TestListNodeClientsRequestFails(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusInternalServerError, map[string]any{"message": "boom"})
	})

	p := newManagementProvider(t, mux)

	_, err := p.ListNodeClients(t.Context(), testOrgID)
	require.ErrorContains(t, err, "list node clients")
}

// TestDeleteNodeClientRejectsInvalidClientID checks the clientID shape guard runs before any network call.
func TestDeleteNodeClientRejectsInvalidClientID(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/clients/", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("an invalid clientID must never reach the Management API")
	})

	p := newManagementProvider(t, mux)

	for _, clientID := range []string{"../etc/passwd", "has a space", "has/a/slash", ""} {
		err := p.DeleteNodeClient(t.Context(), testOrgID, clientID)
		require.ErrorContains(t, err, "invalid node client id")
	}
}

// TestDeleteNodeClientRejectsCrossOrg checks a client owned by a different org is refused.
func TestDeleteNodeClientRejectsCrossOrg(t *testing.T) {
	t.Parallel()

	var sawDelete bool

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients/otherorgsclient", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusOK, map[string]any{
			"client_id": "otherorgsclient",
			"client_metadata": map[string]any{
				"managed_by": "image-factory",
				"org_id":     "org_other",
			},
		})
	})
	mux.HandleFunc("DELETE /api/v2/clients/otherorgsclient", func(w http.ResponseWriter, _ *http.Request) {
		sawDelete = true

		w.WriteHeader(http.StatusNoContent)
	})

	p := newManagementProvider(t, mux)

	err := p.DeleteNodeClient(t.Context(), testOrgID, "otherorgsclient")
	require.ErrorContains(t, err, "not found")
	require.False(t, sawDelete, "a cross-org delete must never reach the Management API")
}

// TestDeleteNodeClientHappyPath checks a matching-org client is looked up then deleted.
func TestDeleteNodeClientHappyPath(t *testing.T) {
	t.Parallel()

	var sawDelete bool

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients/myclient", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusOK, map[string]any{
			"client_id": "myclient",
			"client_metadata": map[string]any{
				"managed_by": "image-factory",
				"org_id":     testOrgID,
			},
		})
	})
	mux.HandleFunc("DELETE /api/v2/clients/myclient", func(w http.ResponseWriter, _ *http.Request) {
		sawDelete = true

		w.WriteHeader(http.StatusNoContent)
	})

	p := newManagementProvider(t, mux)

	require.NoError(t, p.DeleteNodeClient(t.Context(), testOrgID, "myclient"))
	require.True(t, sawDelete)
}

// TestDeleteNodeClientLookupFails checks a failing lookup skips the ownership check and delete.
func TestDeleteNodeClientLookupFails(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients/myclient", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusInternalServerError, map[string]any{"message": "boom"})
	})
	mux.HandleFunc("DELETE /api/v2/clients/myclient", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("a delete must not be attempted when the lookup fails")
	})

	p := newManagementProvider(t, mux)

	err := p.DeleteNodeClient(t.Context(), testOrgID, "myclient")
	require.ErrorContains(t, err, "look up node client")
}

// TestDeleteNodeClientLookupNotFoundMapsToSentinel checks a real Auth0 404 (e.g. an
// already-deleted client) maps to ErrNodeClientNotFound, not a generic wrapped error.
func TestDeleteNodeClientLookupNotFoundMapsToSentinel(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/clients/myclient", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(t, w, http.StatusNotFound, map[string]any{"message": "no such client"})
	})

	p := newManagementProvider(t, mux)

	err := p.DeleteNodeClient(t.Context(), testOrgID, "myclient")
	require.ErrorIs(t, err, auth0.ErrNodeClientNotFound)
}
