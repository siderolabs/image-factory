// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/enterprise/tokens"
)

// newTestStorage stands up an in-process OCI registry and a Storage pointed at it.
func newTestStorage(t *testing.T, refreshInterval time.Duration) *tokens.Storage {
	t.Helper()

	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	return newTestStorageAt(t, strings.TrimPrefix(srv.URL, "http://"), refreshInterval)
}

// newTestStorageAt creates a Storage pointed at an already-running registry host, so multiple
// instances (simulating multiple factory replicas) can share one backing registry.
func newTestStorageAt(t *testing.T, host string, refreshInterval time.Duration) *tokens.Storage {
	t.Helper()

	repo, err := name.NewRepository(host+"/tokens", name.Insecure)
	require.NoError(t, err)

	storage, err := tokens.NewStorage(repo, refreshInterval, []remote.Option{})
	require.NoError(t, err)

	return storage
}

func TestStorageCreateListRevoke(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	records, err := s.List(ctx, "org_a")
	require.NoError(t, err)
	require.Empty(t, records)

	err = s.Create(ctx, "org_a", tokens.Record{ID: "jti-1", Name: "node-1"})
	require.NoError(t, err)

	err = s.Create(ctx, "org_a", tokens.Record{ID: "jti-2", Name: "node-2"})
	require.NoError(t, err)

	records, err = s.List(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, records, 2)

	require.NoError(t, s.Revoke(ctx, "org_a", "jti-1"))

	records, err = s.List(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "jti-2", records[0].ID)
}

func TestStorageConcurrentCreatesAreNotLost(t *testing.T) {
	t.Parallel()

	const count = 8

	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")
	writers := []*tokens.Storage{
		newTestStorageAt(t, host, time.Minute),
		newTestStorageAt(t, host, time.Minute),
	}
	ctx := t.Context()
	start := make(chan struct{})
	errs := make(chan error, count)

	var wg sync.WaitGroup

	for i := range count {
		wg.Go(func() {
			<-start

			errs <- writers[i%len(writers)].Create(ctx, "org_a", tokens.Record{ID: fmt.Sprintf("jti-%d", i), Name: fmt.Sprintf("node-%d", i)})
		})
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	records, err := newTestStorageAt(t, host, time.Minute).List(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, records, count)
}

func TestStorageRevokeMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", tokens.Record{ID: "jti-1", Name: "node-1"})
	require.NoError(t, err)

	err = s.Revoke(ctx, "org_a", "does-not-exist")
	require.ErrorIs(t, err, tokens.ErrNotFound)
}

func TestStorageOrgsAreIsolated(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", tokens.Record{ID: "jti-1", Name: "node-1"})
	require.NoError(t, err)

	records, err := s.List(ctx, "org_b")
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestStorageValid(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", tokens.Record{ID: "jti-1", Name: "node-1"})
	require.NoError(t, err)

	valid, err := s.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.True(t, valid)

	valid, err = s.Valid(ctx, "org_a", "jti-unknown")
	require.NoError(t, err)
	require.False(t, valid)
}

// TestStorageValidCacheDelaysRevocation checks that a revoke's effect on Valid is bounded by
// refreshInterval, not instant, on a replica other than the one that performed the revoke —
// the same instance sees its own write immediately (mutate refreshes its own cache), so this
// uses two Storage instances against the same registry to simulate two factory replicas.
func TestStorageValidCacheDelaysRevocation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")

	writer := newTestStorageAt(t, host, time.Hour)
	reader := newTestStorageAt(t, host, time.Hour)

	ctx := t.Context()

	err := writer.Create(ctx, "org_a", tokens.Record{ID: "jti-1", Name: "node-1"})
	require.NoError(t, err)

	valid, err := reader.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.True(t, valid)

	require.NoError(t, writer.Revoke(ctx, "org_a", "jti-1"))

	// reader's cache was warmed above and refreshInterval is an hour, so it still reports the
	// token as valid immediately after the other replica's revoke.
	valid, err = reader.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.True(t, valid)
}

func TestStorageValidRefreshesAfterInterval(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Millisecond)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", tokens.Record{ID: "jti-1", Name: "node-1"})
	require.NoError(t, err)

	valid, err := s.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.True(t, valid)

	require.NoError(t, s.Revoke(ctx, "org_a", "jti-1"))

	require.Eventually(t, func() bool {
		valid, err := s.Valid(ctx, "org_a", "jti-1")

		return err == nil && !valid
	}, time.Second, time.Millisecond)
}
