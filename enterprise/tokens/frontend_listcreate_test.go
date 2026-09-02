// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/tokens"
	"github.com/siderolabs/image-factory/internal/apitoken"
)

const testOrgID = "org_test"

var errBoom = errors.New("boom")

type fakeAuthProvider struct {
	orgID string
	ok    bool
}

func (f fakeAuthProvider) UsernameFromContext(context.Context) (string, bool) {
	return f.orgID, f.ok
}

type fakeManager struct {
	listErr           error
	createErr         error
	revokeErr         error
	createdName       string
	createdOrgID      string
	revokedOrgID      string
	revokedID         string
	createdScopes     []apitoken.Scope
	records           []tokens.Record
	createdDelegation apitoken.Delegation
	createdTTL        time.Duration
	createdStored     bool
}

func (f *fakeManager) List(_ context.Context, _ string) ([]tokens.Record, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.records, nil
}

func (f *fakeManager) Create(_ context.Context, orgID, name string, scopes []apitoken.Scope, delegation apitoken.Delegation, stored bool, requestedTTL time.Duration) (tokens.Record, string, error) {
	if f.createErr != nil {
		return tokens.Record{}, "", f.createErr
	}

	f.createdOrgID = orgID
	f.createdName = name
	f.createdScopes = scopes
	f.createdDelegation = delegation
	f.createdStored = stored
	f.createdTTL = requestedTTL

	now := time.Now()

	return tokens.Record{ID: "new-token-id", Name: name, Scopes: scopes, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}, "new-token-value", nil
}

func (f *fakeManager) Revoke(_ context.Context, orgID, id string) error {
	f.revokedOrgID = orgID
	f.revokedID = id

	return f.revokeErr
}

func doRequest(t *testing.T, f interface {
	Handle(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error
}, method, target, body string, params httprouter.Params,
) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, params)
	require.NoError(t, err)

	return w
}

func TestListCreateFrontendListHappyPath(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{records: []tokens.Record{{ID: "abc", Name: "my-node", Scopes: imageScopes}}}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodGet, "/tokens", "", nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tokens []struct {
			ID     string   `json:"id"`
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		} `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Tokens, 1)
	require.Equal(t, "abc", resp.Tokens[0].ID)
	require.Equal(t, "my-node", resp.Tokens[0].Name)
	require.Equal(t, []string{"image:read"}, resp.Tokens[0].Scopes)
}

func TestListCreateFrontendListRequiresAuth(t *testing.T) {
	t.Parallel()

	f := tokens.NewListCreateFrontend(&fakeManager{}, fakeAuthProvider{ok: false}, 10)

	w := doRequest(t, f, http.MethodGet, "/tokens", "", nil)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListCreateFrontendCreateHappyPath(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["image:read"],"ttl":"720h"}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testOrgID, mgr.createdOrgID)
	require.Equal(t, "my-node", mgr.createdName)
	require.Equal(t, imageScopes, mgr.createdScopes)
	require.Equal(t, 720*time.Hour, mgr.createdTTL)

	var resp createResponse

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "new-token-id", resp.ID)
	require.Equal(t, "my-node", resp.Name)
	require.Equal(t, "new-token-value", resp.Token)
	require.Equal(t, testOrgID, resp.OrgID)
	require.Equal(t, []string{"image:read"}, resp.Scopes)
	require.True(t, resp.Stored, "a create that does not say otherwise is recorded, so it can be listed and revoked")
	require.True(t, mgr.createdStored)
}

type createResponse struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Token  string   `json:"token"`
	OrgID  string   `json:"org_id"`
	Scopes []string `json:"scopes"`
	Stored bool     `json:"stored"`
}

func TestListCreateFrontendReportsStorage(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"a-link","scopes":["image:read"],"stored":false}`, nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp createResponse

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.False(t, resp.Stored, "the caller asked for a token the factory does not record")
	require.False(t, mgr.createdStored)
}

// TestListCreateFrontendStoresByDefault pins the fail-safe default: a caller who says nothing
// gets a token that can still be taken back.
func TestListCreateFrontendStoresByDefault(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"a-link","scopes":["image:read"]}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, mgr.createdStored)
}

func TestListCreateFrontendCreateRequiresScopes(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node"}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendCreateRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["admin"]}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendRejectsInvalidDelegation(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"unknown issuable scope": `{"name":"n","scopes":["token:issue"],"issuable_scopes":["future:scope"]}`,
		"without issue scope":    `{"name":"n","scopes":["image:read"],"issuable_scopes":["image:read"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mgr := &fakeManager{}
			f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)
			w := doRequest(t, f, http.MethodPost, "/tokens", body, nil)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Empty(t, mgr.createdScopes)
		})
	}
}

func TestListCreateFrontendNeverPropagatesCrossSubjectAuthority(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens",
		`{"name":"n","scopes":["token:issue"],"issuable_scopes":["image:read"],"any_subject":true}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, mgr.createdDelegation.AnySubject)
}

// TestListCreateFrontendRefusesAdminScope covers the rule that the bootstrap credential is not an
// HTTP feature: the caller here holds a full provider credential, the most authority the API
// recognizes, and still cannot mint one.
func TestListCreateFrontendRefusesAdminScope(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"alone":          `{"name":"n","scopes":["admin"]}`,
		"alongside pull": `{"name":"n","scopes":["image:read","admin"]}`,
		"ephemeral":      `{"scopes":["admin"],"stored":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mgr := &fakeManager{}
			f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

			w := doRequest(t, f, http.MethodPost, "/tokens", body, nil)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "unknown scope")
			require.Empty(t, mgr.createdScopes, "the token must never be minted")
		})
	}
}

// TestCreateFromBootstrapMayGrantMinting proves that the CLI credential can create a bounded minter.
func TestCreateFromBootstrapMayGrantMinting(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequestAsBootstrap(t, f, `{"name":"n","scopes":["token:issue"]}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []apitoken.Scope{"token:issue"}, mgr.createdScopes)

	refused := doRequestAsBootstrap(t, f, `{"name":"n","scopes":["admin"]}`)
	require.Equal(t, http.StatusBadRequest, refused.Code, "the retired admin scope stays invalid")
}

// TestCreateForAnotherIdentity covers the cross-tenant rule from both ends: a bootstrap credential may
// name the identity a token belongs to, and nothing else may, a full provider credential included.
func TestCreateForAnotherIdentity(t *testing.T) {
	t.Parallel()

	const other = "org_other"

	t.Run("bootstrap credential may", func(t *testing.T) {
		t.Parallel()

		mgr := &fakeManager{}
		f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

		w := doRequestAsBootstrap(t, f,
			`{"name":"n","scopes":["image:read"],"subject":"`+other+`"}`)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, other, mgr.createdOrgID, "the record belongs to the named identity")

		var resp createResponse

		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, other, resp.OrgID, "and the response reports it, since it is the registry username")
	})

	t.Run("full credential may not", func(t *testing.T) {
		t.Parallel()

		mgr := &fakeManager{}
		f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

		w := doRequest(t, f, http.MethodPost, "/tokens",
			`{"name":"n","scopes":["image:read"],"subject":"`+other+`"}`, nil)

		require.Equal(t, http.StatusForbidden, w.Code, "an htpasswd user must not reach another tenant")
		require.Empty(t, mgr.createdOrgID)
	})

	t.Run("minting token may not", func(t *testing.T) {
		t.Parallel()

		mgr := &fakeManager{}
		f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

		w := doRequestAs(t, f, []apitoken.Scope{"token:issue", "image:read"},
			`{"name":"n","scopes":["image:read"],"subject":"`+other+`"}`)

		require.Equal(t, http.StatusForbidden, w.Code)
		require.Empty(t, mgr.createdOrgID)
	})

	t.Run("naming your own identity is not a cross-tenant mint", func(t *testing.T) {
		t.Parallel()

		mgr := &fakeManager{}
		f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

		w := doRequest(t, f, http.MethodPost, "/tokens",
			`{"name":"n","scopes":["image:read"],"subject":"`+testOrgID+`"}`, nil)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, testOrgID, mgr.createdOrgID)
	})

	t.Run("absent subject still mints for the caller", func(t *testing.T) {
		t.Parallel()

		mgr := &fakeManager{}
		f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

		w := doRequestAsBootstrap(t, f, `{"name":"n","scopes":["image:read"]}`)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, testOrgID, mgr.createdOrgID)
	})
}

// TestCreateRejectsMalformedSubject covers the input validation on a value that reaches the JWT,
// the token index and every audit record the minted token goes on to produce.
func TestCreateRejectsMalformedSubject(t *testing.T) {
	t.Parallel()

	for name, subject := range map[string]string{
		"newline":  "org_a\nfake-audit-line",
		"tab":      "org_a\tb",
		"space":    "org a",
		"nul":      "org_a\u0000",
		"too long": strings.Repeat("o", 257),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mgr := &fakeManager{}
			f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

			body, err := json.Marshal(map[string]any{"name": "n", "scopes": []string{"image:read"}, "subject": subject})
			require.NoError(t, err)

			w := doRequestAsBootstrap(t, f, string(body))

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Empty(t, mgr.createdOrgID)
		})
	}
}

func TestListCreateFrontendCreateRejectsBadTTL(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["image:read"],"ttl":"forever"}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendCreateRejectsEmptyName(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"","scopes":["image:read"]}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdScopes)
}

func TestListCreateFrontendAllowsEmptyNameWhenNotRecorded(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"scopes":["image:read"],"stored":false}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, imageScopes, mgr.createdScopes)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendCreateRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `not json`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListCreateFrontendCreateRejectsAtCap(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{records: []tokens.Record{{ID: "a"}, {ID: "b"}}}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 2)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["image:read"]}`, nil)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendCapIgnoresEphemeralTokens(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{records: []tokens.Record{{ID: "a"}, {ID: "b"}}}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 2)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"scopes":["image:read"],"stored":false}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, imageScopes, mgr.createdScopes)
}

func TestListCreateFrontendListSurfacesManagerError(t *testing.T) {
	t.Parallel()

	f := tokens.NewListCreateFrontend(&fakeManager{listErr: errBoom}, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/tokens", nil)
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, nil)
	require.ErrorIs(t, err, errBoom)
}

func TestListCreateFrontendCreateSurfacesManagerError(t *testing.T) {
	t.Parallel()

	f := tokens.NewListCreateFrontend(&fakeManager{createErr: errBoom}, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/tokens", strings.NewReader(`{"name":"my-node","scopes":["image:read"]}`))
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, nil)
	require.ErrorIs(t, err, errBoom)
}

func TestRevokeFrontendHappyPath(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewRevokeFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true})

	w := doRequest(t, f, http.MethodPost, "/tokens/mytoken/revoke", "", httprouter.Params{{Key: "id", Value: "mytoken"}})

	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, testOrgID, mgr.revokedOrgID)
	require.Equal(t, "mytoken", mgr.revokedID)
}

func TestRevokeFrontendRequiresAuth(t *testing.T) {
	t.Parallel()

	f := tokens.NewRevokeFrontend(&fakeManager{}, fakeAuthProvider{ok: false})

	w := doRequest(t, f, http.MethodPost, "/tokens/mytoken/revoke", "", httprouter.Params{{Key: "id", Value: "mytoken"}})

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRevokeFrontendMapsNotFoundToStatusNotFound(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{revokeErr: tokens.ErrNotFound}
	f := tokens.NewRevokeFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true})

	w := doRequest(t, f, http.MethodPost, "/tokens/mytoken/revoke", "", httprouter.Params{{Key: "id", Value: "mytoken"}})

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRevokeFrontendSurfacesOtherErrors(t *testing.T) {
	t.Parallel()

	f := tokens.NewRevokeFrontend(&fakeManager{revokeErr: errBoom}, fakeAuthProvider{orgID: testOrgID, ok: true})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/tokens/mytoken/revoke", nil)
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, httprouter.Params{{Key: "id", Value: "mytoken"}})
	require.ErrorIs(t, err, errBoom)
}

func doRequestAs(
	t *testing.T,
	f *tokens.ListCreateFrontend,
	scopes []apitoken.Scope,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	ctx := apitoken.ContextWithClaims(t.Context(), apitoken.Claims{
		Scopes:         []apitoken.Scope{"token:issue"},
		IssuableScopes: scopes,
	})

	return doRequestWithContext(t, f, ctx, body)
}

func doRequestAsBootstrap(t *testing.T, f *tokens.ListCreateFrontend, body string) *httptest.ResponseRecorder {
	t.Helper()

	ctx := apitoken.ContextWithClaims(t.Context(), apitoken.Claims{
		Scopes:         []apitoken.Scope{"token:issue"},
		IssuableScopes: apitoken.Scopes(),
		AnySubject:     true,
	})

	return doRequestWithContext(t, f, ctx, body)
}

func doRequestWithContext(
	t *testing.T,
	f *tokens.ListCreateFrontend,
	ctx context.Context,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/tokens", strings.NewReader(body))
	w := httptest.NewRecorder()

	require.NoError(t, f.Handle(ctx, w, r, nil))

	return w
}

var minterScope = []apitoken.Scope{"token:issue"}

func TestCreateRefusesToGrantMoreThanTheCallerHolds(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequestAs(t, f, minterScope, `{"name":"n","scopes":["image:read"]}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, mgr.createdName, "the token must never be minted")
}

func TestCreateAllowsGrantingMintingWhenWithinDelegationCeiling(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequestAs(t, f, minterScope, `{"name":"n","scopes":["token:issue"]}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []apitoken.Scope{"token:issue"}, mgr.createdScopes)
}

func TestCreateAllowsGrantingWhatTheCallerHolds(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	caller := []apitoken.Scope{"token:issue", "image:read"}

	w := doRequestAs(t, f, caller, `{"name":"n","scopes":["image:read"]}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, imageScopes, mgr.createdScopes)
}

func TestCreateAttenuatesChildDelegationCeiling(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		body       string
		want       []apitoken.Scope
		wantStatus int
	}{
		{
			name:       "within parent ceiling",
			body:       `{"name":"n","scopes":["token:issue"],"issuable_scopes":["image:read"]}`,
			wantStatus: http.StatusOK,
			want:       []apitoken.Scope{"image:read"},
		},
		{
			name:       "outside parent ceiling",
			body:       `{"name":"n","scopes":["token:issue"],"issuable_scopes":["report:read"]}`,
			wantStatus: http.StatusForbidden,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mgr := &fakeManager{}
			f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

			w := doRequestAs(t, f, []apitoken.Scope{"token:issue", "image:read"}, test.body)

			require.Equal(t, test.wantStatus, w.Code)
			require.Equal(t, test.want, mgr.createdDelegation.IssuableScopes)
		})
	}
}

func TestCreateFromFullCredentialIsUnrestricted(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens",
		`{"name":"n","scopes":["token:issue"],"issuable_scopes":["image:read","token:issue"]}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []apitoken.Scope{"token:issue"}, mgr.createdScopes)
	require.Equal(t, []apitoken.Scope{"image:read", "token:issue"},
		mgr.createdDelegation.IssuableScopes)
}
