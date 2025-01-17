// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic/storage"
	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic/storage/cache"
)

type mockStorage struct {
	counter atomic.Int64
}

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
		return []byte(fmt.Sprintf("%s-%d", id, counter)), nil
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	underlying := &mockStorage{}
	strg := cache.NewCache(underlying)

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
