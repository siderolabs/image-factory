// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens

import (
	"context"
	"time"

	"github.com/siderolabs/image-factory/internal/apitoken"
)

// Manager mints, tracks and verifies self-issued API tokens.
type Manager struct {
	issuer  *apitoken.Issuer
	storage *Storage
}

// NewManager creates a Manager from a token issuer and its backing storage.
func NewManager(issuer *apitoken.Issuer, storage *Storage) *Manager {
	return &Manager{issuer: issuer, storage: storage}
}

// Create mints a token for orgID carrying scopes. A stored token is recorded in storage
// under its jti, which is both what lists it and what keeps it valid.
func (m *Manager) Create(ctx context.Context, orgID, name string, scopes []apitoken.Scope, delegation apitoken.Delegation, stored bool, requestedTTL time.Duration) (Record, string, error) {
	token, err := m.issuer.IssueWithDelegation(orgID, scopes, delegation, stored, requestedTTL)
	if err != nil {
		return Record{}, "", err
	}

	record := Record{
		ID:             token.ID,
		Name:           name,
		Scopes:         token.Scopes,
		IssuableScopes: token.IssuableScopes,
		CreatedAt:      token.IssuedAt,
		ExpiresAt:      token.ExpiresAt,
	}

	if !token.Stored {
		return record, token.Signed, nil
	}

	if err := m.storage.Create(ctx, orgID, record); err != nil {
		return Record{}, "", err
	}

	return record, token.Signed, nil
}

// List returns orgID's currently active stored tokens.
func (m *Manager) List(ctx context.Context, orgID string) ([]Record, error) {
	return m.storage.List(ctx, orgID)
}

// Revoke tombstones id in orgID's storage, taking it out of circulation for every replica.
func (m *Manager) Revoke(ctx context.Context, orgID, id string) error {
	return m.storage.Revoke(ctx, orgID, id)
}

// Verify reports the claims a bearer credential authenticates, or ok=false if it isn't a
// currently valid API token. Satisfies enterprise.TokenVerifier.
func (m *Manager) Verify(ctx context.Context, tokenStr string) (apitoken.Claims, bool) {
	claims, err := m.issuer.Verify(tokenStr)
	if err != nil {
		return apitoken.Claims{}, false
	}

	if !claims.Stored {
		return claims, true
	}

	valid, err := m.storage.Valid(ctx, claims.Subject, claims.ID)
	if err != nil || !valid {
		return apitoken.Claims{}, false
	}

	return claims, true
}
