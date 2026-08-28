// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package nodetoken_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/nodetoken"
	"github.com/siderolabs/image-factory/internal/downloadtoken"
)

// newTestIssuer creates a NodeAudience Issuer for tests.
func newTestIssuer(t *testing.T) *downloadtoken.Issuer {
	t.Helper()

	issuer, err := downloadtoken.GenerateIssuer(downloadtoken.TTL{Default: time.Hour, Min: time.Minute, Max: 24 * time.Hour}, downloadtoken.NodeAudience)
	require.NoError(t, err)

	return issuer
}

func newTestManager(t *testing.T) *nodetoken.Manager {
	t.Helper()

	return nodetoken.NewManager(newTestIssuer(t), newTestStorage(t, time.Minute))
}

func TestManagerCreateNodeTokenIsVerifiable(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	storage := newTestStorage(t, time.Minute)
	mgr := nodetoken.NewManager(issuer, storage)
	ctx := t.Context()

	id, token, err := mgr.CreateNodeToken(ctx, "org_a", "my-node")
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotEmpty(t, token)

	sub, jti, err := issuer.Verify(token)
	require.NoError(t, err)
	require.Equal(t, "org_a", sub)
	require.Equal(t, id, jti)

	valid, err := storage.Valid(ctx, "org_a", jti)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestManagerListAndRevokeNodeToken(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	id, _, err := mgr.CreateNodeToken(ctx, "org_a", "my-node")
	require.NoError(t, err)

	tokens, err := mgr.ListNodeTokens(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, id, tokens[0].ID)
	require.Equal(t, "my-node", tokens[0].Name)

	require.NoError(t, mgr.RevokeNodeToken(ctx, "org_a", id))

	tokens, err = mgr.ListNodeTokens(ctx, "org_a")
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestManagerRevokeNodeTokenNotFound(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	err := mgr.RevokeNodeToken(t.Context(), "org_a", "does-not-exist")
	require.ErrorIs(t, err, nodetoken.ErrNotFound)
}

func TestManagerVerifyAcceptsValidNodeToken(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	_, token, err := mgr.CreateNodeToken(ctx, "org_a", "my-node")
	require.NoError(t, err)

	orgID, ok := mgr.Verify(ctx, token)
	require.True(t, ok)
	require.Equal(t, "org_a", orgID)
}

func TestManagerVerifyRejectsGarbage(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	_, ok := mgr.Verify(t.Context(), "not-a-token")
	require.False(t, ok)
}

func TestManagerVerifyRejectsWrongAudience(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)
	ctx := t.Context()

	downloadIssuer, err := downloadtoken.GenerateIssuer(downloadtoken.TTL{Default: time.Hour, Min: time.Minute, Max: 24 * time.Hour}, downloadtoken.DownloadAudience)
	require.NoError(t, err)

	token, _, _, err := downloadIssuer.Issue("org_a", 0)
	require.NoError(t, err)

	_, ok := mgr.Verify(ctx, token)
	require.False(t, ok)
}

func TestManagerVerifyRejectsRevokedToken(t *testing.T) {
	t.Parallel()

	issuer := newTestIssuer(t)
	storage := newTestStorage(t, time.Millisecond)
	ctx := t.Context()

	mgr := nodetoken.NewManager(issuer, storage)

	id, token, err := mgr.CreateNodeToken(ctx, "org_a", "my-node")
	require.NoError(t, err)
	require.NoError(t, mgr.RevokeNodeToken(ctx, "org_a", id))

	require.Eventually(t, func() bool {
		_, ok := mgr.Verify(ctx, token)

		return !ok
	}, time.Second, time.Millisecond)
}
