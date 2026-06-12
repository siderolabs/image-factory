// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/cache"
)

func TestVersionedCache(t *testing.T) {
	t.Parallel()

	t.Run("fetches once and caches the result", func(t *testing.T) {
		t.Parallel()

		var calls int

		c := cache.NewSingleFlightCache(func(tag string) (string, error) {
			calls++

			return tag, nil
		})

		got, err := c.Get(t.Context(), "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", got)

		got, err = c.Get(t.Context(), "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", got)

		assert.Equal(t, 1, calls, "second get should be served from cache")
	})

	t.Run("propagates fetch errors without caching", func(t *testing.T) {
		t.Parallel()

		fetchErr := errors.New("boom")

		var calls int

		c := cache.NewSingleFlightCache(func(string) (struct{}, error) {
			calls++

			return struct{}{}, fetchErr
		})

		_, err := c.Get(context.Background(), "v1.0.0")
		require.ErrorIs(t, err, fetchErr)

		_, err = c.Get(context.Background(), "v1.0.0")
		require.ErrorIs(t, err, fetchErr)

		assert.Equal(t, 2, calls, "failed fetches should not be cached")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		c := cache.NewSingleFlightCache(func(tag string) ([]string, error) {
			return []string{tag}, nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := c.Get(ctx, "v1.0.0")
		require.ErrorIs(t, err, context.Canceled)
	})
}
