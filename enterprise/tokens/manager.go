// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/ctxlog"
)

// Manager mints, tracks and verifies self-issued API tokens.
type Manager struct {
	logger  *zap.Logger
	issuer  *apitoken.Issuer
	storage *Storage
}

// NewManager creates a Manager from a token issuer and its backing storage.
func NewManager(logger *zap.Logger, issuer *apitoken.Issuer, storage *Storage) *Manager {
	return &Manager{
		logger:  logger.With(zap.String("component", "api-tokens")),
		issuer:  issuer,
		storage: storage,
	}
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
//
// Every rejection is logged, because the caller falls back to the configured auth provider and
// would otherwise report a plain credential failure for a request that never had one. The
// credential itself is never logged.
func (m *Manager) Verify(ctx context.Context, tokenStr string) (apitoken.Claims, bool) {
	logger := ctxlog.Logger(ctx, m.logger)

	claims, err := m.issuer.Verify(tokenStr)
	if err != nil {
		// Debug: the HTTP frontend offers every Basic password here, so an ordinary
		// username/password login reaches this branch on every request.
		logger.Debug("credential is not an API token", zap.Error(err))

		return apitoken.Claims{}, false
	}

	if !claims.Stored {
		return claims, true
	}

	valid, err := m.storage.Valid(ctx, claims.Subject, claims.ID)
	if err != nil {
		logger.Warn(
			"failed to look up stored API token",
			zap.Error(err),
			zap.String("sub", claims.Subject),
			zap.String("jti", claims.ID),
		)

		return apitoken.Claims{}, false
	}

	if !valid {
		logger.Warn(
			"stored API token is revoked, expired or unknown",
			zap.String("sub", claims.Subject),
			zap.String("jti", claims.ID),
		)

		return apitoken.Claims{}, false
	}

	return claims, true
}
