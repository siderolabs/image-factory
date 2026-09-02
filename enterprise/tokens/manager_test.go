// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/tokens"
	"github.com/siderolabs/image-factory/internal/apitoken"
)

var imageScopes = []apitoken.Scope{"image:read"}

func newTestIssuer(t *testing.T) *apitoken.Issuer {
	t.Helper()

	issuer, err := apitoken.GenerateIssuer(apitoken.TTL{}, apitoken.StorageTTL{
		Stored:    apitoken.TTL{Default: time.Hour, Min: time.Minute, Max: 24 * time.Hour},
		Ephemeral: apitoken.TTL{Default: time.Minute, Min: time.Second, Max: time.Hour},
	})
	require.NoError(t, err)

	return issuer
}

func newTestManager(t *testing.T) *tokens.Manager {
	t.Helper()

	return tokens.NewManager(newTestIssuer(t), newTestStorage(t, time.Minute))
}

func TestManagerCreateIsVerifiable(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	storage := newTestStorage(t, time.Minute)
	mgr := tokens.NewManager(issuer, storage)
	ctx := t.Context()

	record, token, err := mgr.Create(ctx, "org_a", "my-node", imageScopes, apitoken.Delegation{}, true, 0)
	require.NoError(t, err)
	require.NotEmpty(t, record.ID)
	require.NotEmpty(t, token)
	require.Equal(t, time.Hour, record.ExpiresAt.Sub(record.CreatedAt))

	claims, err := issuer.Verify(token)
	require.NoError(t, err)
	require.Equal(t, "org_a", claims.Subject)
	require.Equal(t, record.ID, claims.ID)
	require.Equal(t, imageScopes, claims.Scopes)

	valid, err := storage.Valid(ctx, "org_a", claims.ID)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestManagerCreatePersistsDelegationCeiling(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	mgr := tokens.NewManager(issuer, newTestStorage(t, time.Minute))
	delegation := apitoken.Delegation{IssuableScopes: []apitoken.Scope{
		"image:read",
		"report:read",
	}}

	record, signed, err := mgr.Create(t.Context(), "org_a", "issuer",
		[]apitoken.Scope{"token:issue"}, delegation, true, 0)
	require.NoError(t, err)
	require.Equal(t, delegation.IssuableScopes, record.IssuableScopes)

	claims, err := issuer.Verify(signed)
	require.NoError(t, err)
	require.Equal(t, delegation.IssuableScopes, claims.IssuableScopes)
	require.False(t, claims.AnySubject)

	listed, err := mgr.List(t.Context(), "org_a")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, delegation.IssuableScopes, listed[0].IssuableScopes)
}

func TestManagerCreateEphemeralTokenIsNotRecorded(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	record, token, err := mgr.Create(ctx, "org_a", "", imageScopes, apitoken.Delegation{}, false, 0)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	listed, err := mgr.List(ctx, "org_a")
	require.NoError(t, err)
	require.Empty(t, listed)

	require.ErrorIs(t, mgr.Revoke(ctx, "org_a", record.ID), tokens.ErrNotFound)

	// Nothing to look up, so verification rests on the signature and the claim alone.
	claims, ok := mgr.Verify(ctx, token)
	require.True(t, ok)
	require.Equal(t, "org_a", claims.Subject)
	require.Equal(t, imageScopes, claims.Scopes)
	require.False(t, claims.Stored)
}

// TestManagerCreateStoredTokenIsRecorded is the other half: the same call with stored=true has to
// reach storage, or a revoke would have nothing to remove.
func TestManagerCreateStoredTokenIsRecorded(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	_, token, err := mgr.Create(ctx, "org_a", "my-node", imageScopes, apitoken.Delegation{}, true, 0)
	require.NoError(t, err)

	claims, ok := mgr.Verify(ctx, token)
	require.True(t, ok)
	require.True(t, claims.Stored)

	listed, err := mgr.List(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, listed, 1)
}

func TestManagerCreateRespectsRequestedTTL(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	record, _, err := mgr.Create(ctx, "org_a", "my-node", imageScopes, apitoken.Delegation{}, true, 2*time.Hour)
	require.NoError(t, err)
	require.Equal(t, 2*time.Hour, record.ExpiresAt.Sub(record.CreatedAt))

	_, _, err = mgr.Create(ctx, "org_a", "too-long", imageScopes, apitoken.Delegation{}, true, 48*time.Hour)
	require.ErrorIs(t, err, apitoken.ErrTTLOutOfRange)
}

func TestManagerListAndRevoke(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	record, _, err := mgr.Create(ctx, "org_a", "my-node", imageScopes, apitoken.Delegation{}, true, 0)
	require.NoError(t, err)

	listed, err := mgr.List(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, record.ID, listed[0].ID)
	require.Equal(t, "my-node", listed[0].Name)
	require.Equal(t, imageScopes, listed[0].Scopes)

	require.NoError(t, mgr.Revoke(ctx, "org_a", record.ID))

	listed, err = mgr.List(ctx, "org_a")
	require.NoError(t, err)
	require.Empty(t, listed)
}

func TestManagerRevokeNotFound(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	err := mgr.Revoke(t.Context(), "org_a", "does-not-exist")
	require.ErrorIs(t, err, tokens.ErrNotFound)
}

func TestManagerVerifyAcceptsValidToken(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	_, token, err := mgr.Create(ctx, "org_a", "my-node", imageScopes, apitoken.Delegation{}, true, 0)
	require.NoError(t, err)

	claims, ok := mgr.Verify(ctx, token)
	require.True(t, ok)
	require.Equal(t, "org_a", claims.Subject)
	require.Equal(t, imageScopes, claims.Scopes)
}

func TestManagerVerifyRejectsGarbage(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	_, ok := mgr.Verify(t.Context(), "not-a-token")
	require.False(t, ok)
}

func TestManagerVerifyRejectsForeignKey(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	token, err := newTestIssuer(t).Issue("org_a", imageScopes, true, 0)
	require.NoError(t, err)

	_, ok := mgr.Verify(t.Context(), token.Signed)
	require.False(t, ok)
}

func TestManagerVerifyRejectsRevokedToken(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	storage := newTestStorage(t, time.Millisecond)
	ctx := t.Context()

	mgr := tokens.NewManager(issuer, storage)

	record, token, err := mgr.Create(ctx, "org_a", "my-node", imageScopes, apitoken.Delegation{}, true, 0)
	require.NoError(t, err)
	require.NoError(t, mgr.Revoke(ctx, "org_a", record.ID))

	require.Eventually(t, func() bool {
		_, ok := mgr.Verify(ctx, token)

		return !ok
	}, time.Second, time.Millisecond)
}
