// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package nodetoken

import (
	"context"
	"time"

	"github.com/siderolabs/image-factory/internal/downloadtoken"
)

// Manager mints and tracks self-issued node tokens, backing NodeTokenManager.
type Manager struct {
	issuer  *downloadtoken.Issuer
	storage *Storage
}

// NewManager creates a Manager from a node-audience token issuer and its backing storage.
func NewManager(issuer *downloadtoken.Issuer, storage *Storage) *Manager {
	return &Manager{issuer: issuer, storage: storage}
}

// CreateNodeToken mints a new node token for orgID and records it in the index under its jti.
func (m *Manager) CreateNodeToken(ctx context.Context, orgID, name string) (id, token string, err error) {
	token, _, jti, err := m.issuer.Issue(orgID, 0)
	if err != nil {
		return "", "", err
	}

	if _, err := m.storage.Create(ctx, orgID, Record{ID: jti, Name: name, CreatedAt: time.Now()}); err != nil {
		return "", "", err
	}

	return jti, token, nil
}

// ListNodeTokens returns orgID's currently active node tokens.
func (m *Manager) ListNodeTokens(ctx context.Context, orgID string) ([]Record, error) {
	return m.storage.List(ctx, orgID)
}

// RevokeNodeToken removes id from orgID's index, taking it out of circulation once the
// verification cache next refreshes.
func (m *Manager) RevokeNodeToken(ctx context.Context, orgID, id string) error {
	return m.storage.Revoke(ctx, orgID, id)
}

// Verify reports the org ID a bearer credential authenticates, or ok=false if it isn't a
// currently valid node token. Satisfies NodeTokenVerifier.
func (m *Manager) Verify(ctx context.Context, tokenStr string) (orgID string, ok bool) {
	orgID, jti, err := m.issuer.Verify(tokenStr)
	if err != nil {
		return "", false
	}

	valid, err := m.storage.Valid(ctx, orgID, jti)
	if err != nil || !valid {
		return "", false
	}

	return orgID, true
}
