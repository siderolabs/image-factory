// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package tokens_test

import (
	"fmt"
	"net/http/httptest"
	"slices"
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

func activeRecord(id, name string) tokens.Record {
	return tokens.Record{ID: id, Name: name, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestStorageCreateListRevoke(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	records, err := s.List(ctx, "org_a")
	require.NoError(t, err)
	require.Empty(t, records)

	err = s.Create(ctx, "org_a", activeRecord("jti-1", "node-1"))
	require.NoError(t, err)

	err = s.Create(ctx, "org_a", activeRecord("jti-2", "node-2"))
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

func TestStorageListOmitsExpiredTokens(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", tokens.Record{
		ID:        "expired",
		Name:      "expired",
		ExpiresAt: time.Now().Add(-time.Second),
	})
	require.NoError(t, err)

	err = s.Create(ctx, "org_a", tokens.Record{
		ID:        "active",
		Name:      "active",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	records, err := s.List(ctx, "org_a")
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "active", records[0].ID)
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

			errs <- writers[i%len(writers)].Create(ctx, "org_a", activeRecord(fmt.Sprintf("jti-%d", i), fmt.Sprintf("node-%d", i)))
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

	err := s.Create(ctx, "org_a", activeRecord("jti-1", "node-1"))
	require.NoError(t, err)

	err = s.Revoke(ctx, "org_a", "does-not-exist")
	require.ErrorIs(t, err, tokens.ErrNotFound)
}

func TestStorageOrgsAreIsolated(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", activeRecord("jti-1", "node-1"))
	require.NoError(t, err)

	records, err := s.List(ctx, "org_b")
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestStorageValid(t *testing.T) {
	t.Parallel()

	s := newTestStorage(t, time.Minute)
	ctx := t.Context()

	err := s.Create(ctx, "org_a", activeRecord("jti-1", "node-1"))
	require.NoError(t, err)

	valid, err := s.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.True(t, valid)

	valid, err = s.Valid(ctx, "org_a", "jti-unknown")
	require.NoError(t, err)
	require.False(t, valid)
}

func TestStorageValidReadsExactTokenAcrossReplicas(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")
	writer := newTestStorageAt(t, host, time.Hour)
	reader := newTestStorageAt(t, host, time.Hour)
	ctx := t.Context()

	err := writer.Create(ctx, "org_a", tokens.Record{
		ID:        "jti-1",
		Name:      "node-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	valid, err := reader.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.True(t, valid)

	require.NoError(t, writer.Revoke(ctx, "org_a", "jti-1"))

	valid, err = reader.Valid(ctx, "org_a", "jti-1")
	require.NoError(t, err)
	require.False(t, valid)
}

// TestStorageListScansOnlyOneOrgRepository pins the layout the listing cost depends on: each
// org's records live in their own repository, one tag per token named for its jti. A listing
// therefore reads one org's tokens rather than every token in the deployment.
func TestStorageListScansOnlyOneOrgRepository(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")
	s := newTestStorageAt(t, host, time.Minute)
	ctx := t.Context()

	require.NoError(t, s.Create(ctx, "org_a", activeRecord("jti-1", "node-1")))
	require.NoError(t, s.Create(ctx, "org_a", activeRecord("jti-2", "node-2")))
	require.NoError(t, s.Create(ctx, "org_b", activeRecord("jti-3", "node-3")))

	// Two repositories, one per org, and neither holds the other's tags.
	registryRef, err := name.NewRegistry(host, name.Insecure)
	require.NoError(t, err)

	repositories, err := remote.Catalog(ctx, registryRef)
	require.NoError(t, err)
	require.Len(t, repositories, 2)

	tagSets := make([][]string, 0, len(repositories))

	for _, repository := range repositories {
		require.True(t, strings.HasPrefix(repository, "tokens/"), "org repository %q is not under the base", repository)

		repo, err := name.NewRepository(host+"/"+repository, name.Insecure)
		require.NoError(t, err)

		tags, err := remote.List(repo)
		require.NoError(t, err)

		slices.Sort(tags)
		tagSets = append(tagSets, tags)
	}

	// The tag is the jti itself, so a lookup needs no listing and no hashing of the token ID.
	require.ElementsMatch(t, [][]string{{"jti-1", "jti-2"}, {"jti-3"}}, tagSets)
}
