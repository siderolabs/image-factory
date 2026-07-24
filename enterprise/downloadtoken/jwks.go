// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package downloadtoken

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// JWKSProvider returns a pre-built JWKS document.
// Defined locally to avoid an import cycle with pkg/enterprise.
type JWKSProvider interface {
	JWKS() []byte
}

// JWKSFrontend is the FrontendPlugin that serves the JWKS public key.
type JWKSFrontend struct {
	provider JWKSProvider
}

// NewJWKSFrontend creates a JWKS plugin.
func NewJWKSFrontend(provider JWKSProvider) *JWKSFrontend {
	return &JWKSFrontend{provider: provider}
}

// Methods implements enterprise.FrontendPlugin.
func (f *JWKSFrontend) Methods() []string {
	return []string{http.MethodGet}
}

// Path implements enterprise.FrontendPlugin.
func (f *JWKSFrontend) Path() string {
	return "/.well-known/jwks.json"
}

// PublicRoute implements enterprise.PublicRoute.
func (f *JWKSFrontend) PublicRoute() {}

// Handle implements enterprise.FrontendPlugin.
func (f *JWKSFrontend) Handle(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	w.Header().Set("Content-Type", "application/json")
	w.Write(f.provider.JWKS()) //nolint:errcheck

	return nil
}
