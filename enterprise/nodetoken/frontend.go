// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package nodetoken provides HTTP handlers and backing storage for self-serve, self-issued
// node token management.
package nodetoken

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// maxCreateBodyBytes bounds the /node-tokens POST body, which only ever needs to carry a name.
const maxCreateBodyBytes = 1 << 12

// NodeTokenManager is the subset of Manager used by the HTTP frontends.
type NodeTokenManager interface {
	CreateNodeToken(ctx context.Context, orgID, name string) (id, token string, err error)
	ListNodeTokens(ctx context.Context, orgID string) ([]Record, error)
	RevokeNodeToken(ctx context.Context, orgID, id string) error
}

// AuthProvider is a subset of enterprise.AuthProvider used for identity extraction.
// Defined locally to avoid an import cycle with pkg/enterprise.
type AuthProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// ListCreateFrontend is the FrontendPlugin that lists and creates node tokens.
type ListCreateFrontend struct {
	manager   NodeTokenManager
	authProv  AuthProvider
	maxPerOrg int
}

// NewListCreateFrontend creates the list/create node-token plugin.
func NewListCreateFrontend(manager NodeTokenManager, authProv AuthProvider, maxPerOrg int) *ListCreateFrontend {
	return &ListCreateFrontend{
		manager:   manager,
		authProv:  authProv,
		maxPerOrg: maxPerOrg,
	}
}

// Methods implements enterprise.FrontendPlugin.
func (f *ListCreateFrontend) Methods() []string {
	return []string{http.MethodGet, http.MethodPost}
}

// Path implements enterprise.FrontendPlugin.
func (f *ListCreateFrontend) Path() string {
	return "/node-tokens"
}

// requireOrgID extracts the authenticated org ID from ctx, or writes a 401 and reports ok=false.
func requireOrgID(ctx context.Context, w http.ResponseWriter, authProv AuthProvider) (orgID string, ok bool) {
	orgID, ok = authProv.UsernameFromContext(ctx)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}

	return orgID, ok
}

// writeJSON sets the standard no-store JSON response headers and encodes v.
func writeJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	return json.NewEncoder(w).Encode(v)
}

// Handle implements enterprise.FrontendPlugin.
func (f *ListCreateFrontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	orgID, ok := requireOrgID(ctx, w, f.authProv)
	if !ok {
		return nil
	}

	if r.Method == http.MethodPost {
		return f.create(ctx, w, r, orgID)
	}

	return f.list(ctx, w, orgID)
}

func (f *ListCreateFrontend) list(ctx context.Context, w http.ResponseWriter, orgID string) error {
	tokens, err := f.manager.ListNodeTokens(ctx, orgID)
	if err != nil {
		return err
	}

	return writeJSON(w, struct {
		Tokens []Record `json:"tokens"`
	}{Tokens: tokens})
}

// decodeCreateBody returns ok=false for both a malformed body and an empty name; the caller
// responds with the same 400 either way, so the two don't need to be told apart.
func decodeCreateBody(r *http.Request) (name string, ok bool) {
	var body struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, maxCreateBodyBytes)).Decode(&body); err != nil {
		return "", false
	}

	name = strings.TrimSpace(body.Name)

	return name, name != ""
}

func (f *ListCreateFrontend) create(ctx context.Context, w http.ResponseWriter, r *http.Request, orgID string) error {
	name, ok := decodeCreateBody(r)
	if !ok {
		http.Error(w, `a non-empty "name" is required`, http.StatusBadRequest)

		return nil
	}

	// Racy against a concurrent create from the same org; a low-frequency admin action, so a
	// retry after hitting the cap is an acceptable cost of not synchronizing this check.
	existing, err := f.manager.ListNodeTokens(ctx, orgID)
	if err != nil {
		return err
	}

	if len(existing) >= f.maxPerOrg {
		http.Error(w, "node token limit reached for this organization", http.StatusConflict)

		return nil
	}

	id, token, err := f.manager.CreateNodeToken(ctx, orgID, name)
	if err != nil {
		return err
	}

	return writeJSON(w, struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
		OrgID string `json:"org_id"`
	}{
		ID:    id,
		Name:  name,
		Token: token,
		OrgID: orgID,
	})
}

// RevokeFrontend is the FrontendPlugin that revokes a node token.
type RevokeFrontend struct {
	manager  NodeTokenManager
	authProv AuthProvider
}

// NewRevokeFrontend creates the node-token revocation plugin.
func NewRevokeFrontend(manager NodeTokenManager, authProv AuthProvider) *RevokeFrontend {
	return &RevokeFrontend{manager: manager, authProv: authProv}
}

// Methods implements enterprise.FrontendPlugin.
func (f *RevokeFrontend) Methods() []string {
	return []string{http.MethodPost}
}

// Path implements enterprise.FrontendPlugin.
func (f *RevokeFrontend) Path() string {
	return "/node-tokens/:id/revoke"
}

// Handle implements enterprise.FrontendPlugin.
func (f *RevokeFrontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	orgID, ok := requireOrgID(ctx, w, f.authProv)
	if !ok {
		return nil
	}

	err := f.manager.RevokeNodeToken(ctx, orgID, p.ByName("id"))

	switch {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)

		return nil
	case errors.Is(err, ErrNotFound):
		http.Error(w, "node token not found", http.StatusNotFound)

		return nil
	default:
		return err
	}
}
