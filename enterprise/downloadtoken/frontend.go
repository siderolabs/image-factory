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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/downloadtoken"
)

// AuthProvider is a subset of enterprise.AuthProvider used for identity extraction.
// Defined locally to avoid an import cycle with pkg/enterprise.
type AuthProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// Issuer creates signed JWT download tokens.
// Defined locally to avoid an import cycle with pkg/enterprise.
type Issuer interface {
	Issue(subject string, requestedTTL time.Duration) (string, time.Duration, error)
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

// parseTTL parses the ttl query parameter; an absent parameter means "unspecified" and
// yields a zero duration, for which the issuer grants its configured default.
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

// Handle implements enterprise.FrontendPlugin.
func (f *Frontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	username, ok := f.authProv.UsernameFromContext(ctx)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)

		return nil
	}

	rawTTL := r.URL.Query().Get("ttl")

	requestedTTL, ok := parseTTL(rawTTL)
	if !ok {
		http.Error(w, fmt.Sprintf("invalid ttl %q: expected a positive Go duration, e.g. 30m", rawTTL), http.StatusBadRequest)

		return nil
	}

	token, ttl, err := f.issuer.Issue(username, requestedTTL)
	if err != nil {
		if errors.Is(err, downloadtoken.ErrTTLOutOfRange) {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return nil
		}

		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	return json.NewEncoder(w).Encode(struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(ttl.Seconds()),
	})
}
