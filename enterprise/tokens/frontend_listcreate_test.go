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
	listErr       error
	createErr     error
	revokeErr     error
	createdName   string
	createdOrgID  string
	revokedOrgID  string
	revokedID     string
	createdScopes []apitoken.Scope
	records       []tokens.Record
	createdTTL    time.Duration
	createdStored bool
}

func (f *fakeManager) List(_ context.Context, _ string) ([]tokens.Record, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.records, nil
}

func (f *fakeManager) Create(_ context.Context, orgID, name string, scopes []apitoken.Scope, stored bool, requestedTTL time.Duration) (tokens.Record, string, error) {
	if f.createErr != nil {
		return tokens.Record{}, "", f.createErr
	}

	f.createdOrgID = orgID
	f.createdName = name
	f.createdScopes = scopes
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

	mgr := &fakeManager{records: []tokens.Record{{ID: "abc", Name: "my-node", Scopes: pullScope}}}
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
	require.Equal(t, []string{"pull"}, resp.Tokens[0].Scopes)
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

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["pull"],"ttl":"720h"}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testOrgID, mgr.createdOrgID)
	require.Equal(t, "my-node", mgr.createdName)
	require.Equal(t, pullScope, mgr.createdScopes)
	require.Equal(t, 720*time.Hour, mgr.createdTTL)

	var resp createResponse

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "new-token-id", resp.ID)
	require.Equal(t, "my-node", resp.Name)
	require.Equal(t, "new-token-value", resp.Token)
	require.Equal(t, testOrgID, resp.OrgID)
	require.Equal(t, []string{"pull"}, resp.Scopes)
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

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"a-link","scopes":["download"],"stored":false}`, nil)

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

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"a-link","scopes":["download"]}`, nil)

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

func TestListCreateFrontendCreateRejectsBadTTL(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["pull"],"ttl":"forever"}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendCreateRejectsEmptyName(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"","scopes":["pull"]}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdScopes)
}

func TestListCreateFrontendAllowsEmptyNameWhenNotRecorded(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"scopes":["download"],"stored":false}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, downloadScope, mgr.createdScopes)
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

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"my-node","scopes":["pull"]}`, nil)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestListCreateFrontendCapIgnoresUnstoredTokens(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{records: []tokens.Record{{ID: "a"}, {ID: "b"}}}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 2)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"scopes":["download"],"stored":false}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, downloadScope, mgr.createdScopes)
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

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/tokens", strings.NewReader(`{"name":"my-node","scopes":["pull"]}`))
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

func doRequestAs(t *testing.T, f interface {
	Handle(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error
}, scopes []apitoken.Scope, target, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	ctx := apitoken.ContextWithScopes(t.Context(), scopes)
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
	w := httptest.NewRecorder()

	require.NoError(t, f.Handle(ctx, w, r, nil))

	return w
}

var minterScope = []apitoken.Scope{apitoken.ScopeToken}

func TestCreateRefusesToGrantMoreThanTheCallerHolds(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequestAs(t, f, minterScope, "/tokens", `{"name":"n","scopes":["pull"]}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, mgr.createdName, "the token must never be minted")
}

func TestCreateRefusesToGrantMinting(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequestAs(t, f, minterScope, "/tokens", `{"name":"n","scopes":["token"]}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, mgr.createdName)
}

func TestCreateAllowsGrantingWhatTheCallerHolds(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	caller := []apitoken.Scope{apitoken.ScopeToken, apitoken.ScopePull}

	w := doRequestAs(t, f, caller, "/tokens", `{"name":"n","scopes":["pull"]}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, pullScope, mgr.createdScopes)
}

func TestCreateFromFullCredentialIsUnrestricted(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := tokens.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/tokens", `{"name":"n","scopes":["token"]}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []apitoken.Scope{apitoken.ScopeToken}, mgr.createdScopes)
}
