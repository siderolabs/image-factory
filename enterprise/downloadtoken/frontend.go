// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package downloadtoken provides HTTP handlers for download token issuance and JWKS.
package downloadtoken

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// AuthProvider is a subset of enterprise.AuthProvider used for identity extraction.
// Defined locally to avoid an import cycle with pkg/enterprise.
type AuthProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// Issuer creates signed JWT download tokens.
// Defined locally to avoid an import cycle with pkg/enterprise.
type Issuer interface {
	Issue(subject string) (string, error)
}

// Frontend is the FrontendPlugin that issues download tokens.
type Frontend struct {
	issuer   Issuer
	authProv AuthProvider
}

// NewFrontend creates a download-token issuance plugin.
func NewFrontend(issuer Issuer, authProv AuthProvider) *Frontend {
	return &Frontend{issuer: issuer, authProv: authProv}
}

// Methods implements enterprise.FrontendPlugin.
func (f *Frontend) Methods() []string {
	return []string{http.MethodPost}
}

// Path implements enterprise.FrontendPlugin.
func (f *Frontend) Path() string {
	return "/download-token"
}

// Handle implements enterprise.FrontendPlugin.
func (f *Frontend) Handle(ctx context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	username, ok := f.authProv.UsernameFromContext(ctx)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)

		return nil
	}

	token, err := f.issuer.Issue(username)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")

	return json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}
