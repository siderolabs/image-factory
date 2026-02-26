// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/internal/schematic/storage"
	"github.com/siderolabs/image-factory/internal/schematic/storage/cache"
)

type mockStorage struct {
	counter atomic.Int64
}

type toggleMockStorage struct {
	counter  atomic.Int64
	notFound atomic.Bool
}

func (s *toggleMockStorage) Head(_ context.Context, _ string) error {
	panic("should never be called")
}

func (s *toggleMockStorage) Get(_ context.Context, id string) ([]byte, error) {
	counter := s.counter.Add(1)

	if s.notFound.Load() {
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	return fmt.Appendf(nil, "%s-%d", id, counter), nil
}

func (s *toggleMockStorage) Put(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (s *toggleMockStorage) Describe(chan<- *prometheus.Desc) {}
func (s *toggleMockStorage) Collect(chan<- prometheus.Metric) {}

func (s *mockStorage) Head(_ context.Context, _ string) error {
	panic("should never be called")
}

func (s *mockStorage) Get(_ context.Context, id string) ([]byte, error) {
	counter := s.counter.Add(1)

	switch id {
	case "not-found":
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	case "failing":
		return nil, fmt.Errorf("failing")
	default:
		return fmt.Appendf(nil, "%s-%d", id, counter), nil
	}
}

func (s *mockStorage) Put(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (s *mockStorage) Describe(chan<- *prometheus.Desc) {
}

func (s *mockStorage) Collect(chan<- prometheus.Metric) {
}

func TestStorage(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	underlying := &mockStorage{}
	strg := cache.NewCache(underlying, cache.Options{
		CacheCapacity: 100,
		NegativeTTL:   time.Second,
	})

	var eg errgroup.Group

	eg.Go(func() error {
		return strg.Start()
	})

	t.Cleanup(func() {
		// there is a race that strg.Start() might have not been started yet
		// by the time we call strg.Stop(), so we need to do a bit of a dance
		time.Sleep(100 * time.Millisecond) // wait for strg.Start() to actually stop

		strg.Stop()

		require.NoError(t, eg.Wait())
	})

	v, err := strg.Get(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "foo-1", string(v))

	v, err = strg.Get(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "foo-1", string(v)) // cached value

	v, err = strg.Get(ctx, "bar")
	require.NoError(t, err)
	assert.Equal(t, "bar-2", string(v)) // cached value

	err = strg.Head(ctx, "bar")
	require.NoError(t, err)

	err = strg.Head(ctx, "baz")
	require.NoError(t, err)

	v, err = strg.Get(ctx, "baz")
	require.NoError(t, err)
	assert.Equal(t, "baz-3", string(v)) // cached value

	err = strg.Put(ctx, "foo", []byte("newvalue"))
	require.NoError(t, err)

	v, err = strg.Get(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "newvalue", string(v)) // write-through cached value

	_, err = strg.Get(ctx, "not-found")
	require.Error(t, err)
	require.True(t, xerrors.TagIs[storage.ErrNotFoundTag](err))

	err = strg.Head(ctx, "not-found")
	require.Error(t, err)
	require.True(t, xerrors.TagIs[storage.ErrNotFoundTag](err)) // should be cached

	_, err = strg.Get(ctx, "not-found")
	require.Error(t, err)
	require.True(t, xerrors.TagIs[storage.ErrNotFoundTag](err)) // should be cached

	v, err = strg.Get(ctx, "foobar")
	require.NoError(t, err)
	assert.Equal(t, "foobar-5", string(v)) // counter was incremented once on 'not-found' and once on 'foobar'

	_, err = strg.Get(ctx, "failing")
	require.Error(t, err)
	require.ErrorContains(t, err, "failing") // should be not cached

	_, err = strg.Get(ctx, "failing")
	require.Error(t, err)
	require.ErrorContains(t, err, "failing") // should be not cached

	v, err = strg.Get(ctx, "lastone")
	require.NoError(t, err)
	assert.Equal(t, "lastone-8", string(v)) // counter was incremented twice on 'failing' and once on 'lastone'
}

func TestStorageNegativeCacheTTL(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mock := &toggleMockStorage{}
	mock.notFound.Store(true)

	const negativeTTL = 50 * time.Millisecond

	strg := cache.NewCache(mock, cache.Options{
		CacheCapacity: 100,
		NegativeTTL:   negativeTTL,
	})

	var eg errgroup.Group

	eg.Go(func() error {
		return strg.Start()
	})

	t.Cleanup(func() {
		strg.Stop()

		require.NoError(t, eg.Wait())
	})

	// First Get: cache miss, underlying returns 404, result cached as negative.
	_, err := strg.Get(ctx, "item")
	require.Error(t, err)
	require.True(t, xerrors.TagIs[storage.ErrNotFoundTag](err))
	assert.Equal(t, int64(1), mock.counter.Load())

	// Second Get within TTL: 404 served from negative cache, underlying not called.
	_, err = strg.Get(ctx, "item")
	require.Error(t, err)
	require.True(t, xerrors.TagIs[storage.ErrNotFoundTag](err))
	assert.Equal(t, int64(1), mock.counter.Load())

	// Flip underlying so it now returns real data.
	mock.notFound.Store(false)

	// Wait for the negative cache entry to expire by polling until we see real data.
	var (
		v       []byte
		lastErr error
	)

	require.Eventually(t, func() bool {
		v, lastErr = strg.Get(ctx, "item")
		if lastErr != nil {
			return false
		}

		return string(v) == "item-2"
	}, 5*negativeTTL, negativeTTL/10)

	require.NoError(t, lastErr)
}

func TestStorageMetrics(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	underlying := &mockStorage{}
	strg := cache.NewCache(underlying, cache.Options{
		CacheCapacity: 100,
	})

	_, err := strg.Get(ctx, "foo")
	require.NoError(t, err)

	_, err = strg.Get(ctx, "bar")
	require.NoError(t, err)

	problems, err := testutil.CollectAndLint(strg)
	require.NoError(t, err)
	assert.Empty(t, problems)

	assert.Equal(t, 1, testutil.CollectAndCount(strg))
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
