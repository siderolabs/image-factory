// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package nodetoken provides HTTP handlers for self-serve Auth0 node token management.
package nodetoken

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
)

// maxCreateBodyBytes bounds the /node-tokens POST body, which only ever needs to carry a name.
const maxCreateBodyBytes = 1 << 12

// NodeTokenManager is the subset of auth0.Provider used to manage node tokens.
// Defined locally to avoid an import cycle with pkg/enterprise.
type NodeTokenManager interface {
	CreateNodeClient(ctx context.Context, orgID, name string) (clientID, clientSecret string, err error)
	ListNodeClients(ctx context.Context, orgID string) ([]auth0.NodeClient, error)
	DeleteNodeClient(ctx context.Context, orgID, clientID string) error
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
	audience  string
	tokenURL  string
	maxPerOrg int
}

// NewListCreateFrontend creates the list/create node-token plugin. audience and tokenURL are
// echoed back to the caller on create, so a node can build its own client-credentials request.
func NewListCreateFrontend(manager NodeTokenManager, authProv AuthProvider, maxPerOrg int, audience, tokenURL string) *ListCreateFrontend {
	return &ListCreateFrontend{
		manager:   manager,
		authProv:  authProv,
		maxPerOrg: maxPerOrg,
		audience:  audience,
		tokenURL:  tokenURL,
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

// Handle implements enterprise.FrontendPlugin.
func (f *ListCreateFrontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	orgID, ok := f.authProv.UsernameFromContext(ctx)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)

		return nil
	}

	if r.Method == http.MethodPost {
		return f.create(ctx, w, r, orgID)
	}

	return f.list(ctx, w, orgID)
}

type nodeTokenResponse struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
}

func (f *ListCreateFrontend) list(ctx context.Context, w http.ResponseWriter, orgID string) error {
	clients, err := f.manager.ListNodeClients(ctx, orgID)
	if err != nil {
		return err
	}

	tokens := make([]nodeTokenResponse, len(clients))
	for i, c := range clients {
		tokens[i] = nodeTokenResponse{ID: c.ClientID, Name: c.Name, CreatedAt: c.CreatedAt}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	return json.NewEncoder(w).Encode(struct {
		Tokens []nodeTokenResponse `json:"tokens"`
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

	// Racy against a concurrent create from the same org; Auth0's tenant-wide client ceiling
	// is the hard backstop, so this check only needs to catch the common case.
	existing, err := f.manager.ListNodeClients(ctx, orgID)
	if err != nil {
		return err
	}

	if len(existing) >= f.maxPerOrg {
		http.Error(w, "node token limit reached for this organization", http.StatusConflict)

		return nil
	}

	clientID, clientSecret, err := f.manager.CreateNodeClient(ctx, orgID, name)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	return json.NewEncoder(w).Encode(struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Audience     string `json:"audience"`
		TokenURL     string `json:"token_url"`
	}{
		ID:           clientID,
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Audience:     f.audience,
		TokenURL:     f.tokenURL,
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
	orgID, ok := f.authProv.UsernameFromContext(ctx)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)

		return nil
	}

	err := f.manager.DeleteNodeClient(ctx, orgID, p.ByName("id"))

	switch {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)

		return nil
	case errors.Is(err, auth0.ErrInvalidNodeClientID), errors.Is(err, auth0.ErrNodeClientNotFound):
		http.Error(w, "node token not found", http.StatusNotFound)

		return nil
	default:
		return err
	}
}
