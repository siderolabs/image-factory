// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package nodetoken_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/nodetoken"
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
	listErr      error
	createErr    error
	revokeErr    error
	createdName  string
	createdOrgID string
	revokedOrgID string
	revokedID    string
	tokens       []nodetoken.Record
}

func (f *fakeManager) ListNodeTokens(_ context.Context, _ string) ([]nodetoken.Record, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.tokens, nil
}

func (f *fakeManager) CreateNodeToken(_ context.Context, orgID, name string) (string, string, error) {
	if f.createErr != nil {
		return "", "", f.createErr
	}

	f.createdOrgID = orgID
	f.createdName = name

	return "new-token-id", "new-token-value", nil
}

func (f *fakeManager) RevokeNodeToken(_ context.Context, orgID, id string) error {
	f.revokedOrgID = orgID
	f.revokedID = id

	return f.revokeErr
}

// doRequest runs a request through f.Handle and returns the recorded response, asserting that
// Handle itself returned no error (i.e. the plugin answered the request rather than deferring
// to the frontend's generic error handling).
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

// TestListCreateFrontendListHappyPath checks GET returns the manager's node tokens as JSON.
func TestListCreateFrontendListHappyPath(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{tokens: []nodetoken.Record{{ID: "abc", Name: "my-node"}}}
	f := nodetoken.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodGet, "/node-tokens", "", nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tokens []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Tokens, 1)
	require.Equal(t, "abc", resp.Tokens[0].ID)
	require.Equal(t, "my-node", resp.Tokens[0].Name)
}

// TestListCreateFrontendListRequiresAuth checks GET without a resolvable identity is rejected.
func TestListCreateFrontendListRequiresAuth(t *testing.T) {
	t.Parallel()

	f := nodetoken.NewListCreateFrontend(&fakeManager{}, fakeAuthProvider{ok: false}, 10)

	w := doRequest(t, f, http.MethodGet, "/node-tokens", "", nil)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestListCreateFrontendCreateHappyPath checks POST creates a token and returns it exactly once.
func TestListCreateFrontendCreateHappyPath(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := nodetoken.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/node-tokens", `{"name":"my-node"}`, nil)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testOrgID, mgr.createdOrgID)
	require.Equal(t, "my-node", mgr.createdName)

	var resp struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
		OrgID string `json:"org_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "new-token-id", resp.ID)
	require.Equal(t, "my-node", resp.Name)
	require.Equal(t, "new-token-value", resp.Token)
	require.Equal(t, testOrgID, resp.OrgID)
}

// TestListCreateFrontendCreateRejectsEmptyName checks an empty name is rejected before create.
func TestListCreateFrontendCreateRejectsEmptyName(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := nodetoken.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/node-tokens", `{"name":""}`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Empty(t, mgr.createdName)
}

// TestListCreateFrontendCreateRejectsMalformedBody checks non-JSON is rejected before create.
func TestListCreateFrontendCreateRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := nodetoken.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	w := doRequest(t, f, http.MethodPost, "/node-tokens", `not json`, nil)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListCreateFrontendCreateRejectsAtCap checks create is refused once the org is at maxPerOrg.
func TestListCreateFrontendCreateRejectsAtCap(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{tokens: []nodetoken.Record{{ID: "a"}, {ID: "b"}}}
	f := nodetoken.NewListCreateFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true}, 2)

	w := doRequest(t, f, http.MethodPost, "/node-tokens", `{"name":"my-node"}`, nil)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Empty(t, mgr.createdName)
}

// TestListCreateFrontendListSurfacesManagerError checks a list failure surfaces as an error.
func TestListCreateFrontendListSurfacesManagerError(t *testing.T) {
	t.Parallel()

	f := nodetoken.NewListCreateFrontend(&fakeManager{listErr: errBoom}, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/node-tokens", nil)
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, nil)
	require.ErrorIs(t, err, errBoom)
}

// TestListCreateFrontendCreateSurfacesManagerError checks a create failure surfaces as an error.
func TestListCreateFrontendCreateSurfacesManagerError(t *testing.T) {
	t.Parallel()

	f := nodetoken.NewListCreateFrontend(&fakeManager{createErr: errBoom}, fakeAuthProvider{orgID: testOrgID, ok: true}, 10)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/node-tokens", strings.NewReader(`{"name":"my-node"}`))
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, nil)
	require.ErrorIs(t, err, errBoom)
}

// TestRevokeFrontendHappyPath checks POST revokes the named token for the caller's org.
func TestRevokeFrontendHappyPath(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{}
	f := nodetoken.NewRevokeFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true})

	w := doRequest(t, f, http.MethodPost, "/node-tokens/mytoken/revoke", "", httprouter.Params{{Key: "id", Value: "mytoken"}})

	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, testOrgID, mgr.revokedOrgID)
	require.Equal(t, "mytoken", mgr.revokedID)
}

// TestRevokeFrontendRequiresAuth checks POST without a resolvable identity is rejected.
func TestRevokeFrontendRequiresAuth(t *testing.T) {
	t.Parallel()

	f := nodetoken.NewRevokeFrontend(&fakeManager{}, fakeAuthProvider{ok: false})

	w := doRequest(t, f, http.MethodPost, "/node-tokens/mytoken/revoke", "", httprouter.Params{{Key: "id", Value: "mytoken"}})

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRevokeFrontendMapsNotFoundToStatusNotFound checks the not-found sentinel maps to a 404,
// not the generic 500 other manager errors get.
func TestRevokeFrontendMapsNotFoundToStatusNotFound(t *testing.T) {
	t.Parallel()

	mgr := &fakeManager{revokeErr: nodetoken.ErrNotFound}
	f := nodetoken.NewRevokeFrontend(mgr, fakeAuthProvider{orgID: testOrgID, ok: true})

	w := doRequest(t, f, http.MethodPost, "/node-tokens/mytoken/revoke", "", httprouter.Params{{Key: "id", Value: "mytoken"}})

	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestRevokeFrontendSurfacesOtherErrors checks a non-sentinel revoke failure surfaces as an error.
func TestRevokeFrontendSurfacesOtherErrors(t *testing.T) {
	t.Parallel()

	f := nodetoken.NewRevokeFrontend(&fakeManager{revokeErr: errBoom}, fakeAuthProvider{orgID: testOrgID, ok: true})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/node-tokens/mytoken/revoke", nil)
	w := httptest.NewRecorder()

	err := f.Handle(t.Context(), w, r, httprouter.Params{{Key: "id", Value: "mytoken"}})
	require.ErrorIs(t, err, errBoom)
}
