// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package tokens provides HTTP handlers and storage for self-issued API tokens.
package tokens

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/apitoken"
)

// maxCreateBodyBytes bounds the /tokens POST body, which only ever needs to carry a name,
// a TTL and a scope list.
const maxCreateBodyBytes = 1 << 12

// TokenManager is the subset of Manager used by the HTTP frontends.
type TokenManager interface {
	Create(ctx context.Context, orgID, name string, scopes []apitoken.Scope, stored bool, requestedTTL time.Duration) (Record, string, error)
	List(ctx context.Context, orgID string) ([]Record, error)
	Revoke(ctx context.Context, orgID, id string) error
}

// AuthProvider is a subset of enterprise.AuthProvider used for identity extraction.
// Defined locally to avoid an import cycle with pkg/enterprise.
type AuthProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// ListCreateFrontend is the FrontendPlugin that lists and creates API tokens.
type ListCreateFrontend struct {
	manager   TokenManager
	authProv  AuthProvider
	maxPerOrg int
}

// NewListCreateFrontend creates the list/create plugin serving /tokens.
func NewListCreateFrontend(manager TokenManager, authProv AuthProvider, maxPerOrg int) *ListCreateFrontend {
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
	return "/tokens"
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
	tokens, err := f.manager.List(ctx, orgID)
	if err != nil {
		return err
	}

	return writeJSON(w, struct {
		Tokens []Record `json:"tokens"`
	}{Tokens: tokens})
}

type createRequest struct {
	Stored *bool    `json:"stored"`
	Name   string   `json:"name"`
	TTL    string   `json:"ttl"`
	Scopes []string `json:"scopes"`
}

// createParams is an accepted create request.
type createParams struct {
	name   string
	scopes []apitoken.Scope
	ttl    time.Duration
	stored bool
}

// parseTTL parses a requested ttl; an absent value means "unspecified" and yields a zero
// duration, for which the issuer grants its configured default.
func parseTTL(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, true
	}

	ttl, err := time.ParseDuration(raw)
	if err != nil || ttl <= 0 {
		return 0, false
	}

	return ttl, true
}

// decodeCreateBody returns the reason a request is unacceptable, empty when it is fine; every
// reason is answered with the same 400, so the caller doesn't need to tell them apart.
func (f *ListCreateFrontend) decodeCreateBody(r *http.Request) (params createParams, reason string) {
	var body createRequest

	if err := json.NewDecoder(io.LimitReader(r.Body, maxCreateBodyBytes)).Decode(&body); err != nil {
		return createParams{}, "malformed request body"
	}

	if len(body.Scopes) == 0 {
		return createParams{}, `a non-empty "scopes" list is required`
	}

	scopes, err := apitoken.ParseScopes(strings.Join(body.Scopes, " "))
	if err != nil {
		return createParams{}, err.Error()
	}

	stored := body.Stored == nil || *body.Stored

	name := strings.TrimSpace(body.Name)
	if name == "" && stored {
		return createParams{}, `a non-empty "name" is required`
	}

	ttl, ok := parseTTL(body.TTL)
	if !ok {
		return createParams{}, "invalid ttl: expected a positive Go duration, e.g. 720h"
	}

	return createParams{name: name, scopes: scopes, ttl: ttl, stored: stored}, ""
}

func (f *ListCreateFrontend) create(ctx context.Context, w http.ResponseWriter, r *http.Request, orgID string) error {
	params, reason := f.decodeCreateBody(r)
	if reason != "" {
		http.Error(w, reason, http.StatusBadRequest)

		return nil
	}

	if callerScopes, viaToken := apitoken.ScopesFromContext(ctx); viaToken && !apitoken.CanGrant(callerScopes, params.scopes) {
		http.Error(w, "the authenticating token may not grant these scopes", http.StatusForbidden)

		return nil
	}

	if params.stored {
		existing, err := f.manager.List(ctx, orgID)
		if err != nil {
			return err
		}

		// Racy against a concurrent create from the same org; a low-frequency admin action, so a
		// retry after hitting the cap is an acceptable cost of not synchronizing this check.
		if len(existing) >= f.maxPerOrg {
			http.Error(w, "token limit reached for this organization", http.StatusConflict)

			return nil
		}
	}

	record, token, err := f.manager.Create(ctx, orgID, params.name, params.scopes, params.stored, params.ttl)
	if err != nil {
		if errors.Is(err, apitoken.ErrTTLOutOfRange) || errors.Is(err, apitoken.ErrUnknownScope) {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return nil
		}

		return err
	}

	return writeJSON(w, struct {
		Token string `json:"token"`
		OrgID string `json:"org_id"`

		Record

		Stored bool `json:"stored"`
	}{
		Token:  token,
		OrgID:  orgID,
		Record: record,
		Stored: params.stored,
	})
}

// RevokeFrontend is the FrontendPlugin that revokes an API token.
type RevokeFrontend struct {
	manager  TokenManager
	authProv AuthProvider
}

// NewRevokeFrontend creates the revocation plugin serving /tokens/:id/revoke.
func NewRevokeFrontend(manager TokenManager, authProv AuthProvider) *RevokeFrontend {
	return &RevokeFrontend{manager: manager, authProv: authProv}
}

// Methods implements enterprise.FrontendPlugin.
func (f *RevokeFrontend) Methods() []string {
	return []string{http.MethodPost}
}

// Path implements enterprise.FrontendPlugin.
func (f *RevokeFrontend) Path() string {
	return "/tokens/:id/revoke"
}

// Handle implements enterprise.FrontendPlugin.
func (f *RevokeFrontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	orgID, ok := requireOrgID(ctx, w, f.authProv)
	if !ok {
		return nil
	}

	err := f.manager.Revoke(ctx, orgID, p.ByName("id"))

	switch {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)

		return nil
	case errors.Is(err, ErrNotFound):
		http.Error(w, "token not found", http.StatusNotFound)

		return nil
	default:
		return err
	}
}
