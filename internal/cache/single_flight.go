// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache

import (
	"context"
	"sync"

	"golang.org/x/sync/singleflight"
)

// SingleFlightCache lazily fetches and caches a list of values keyed by a version tag.
//
// Fetches are de-duplicated across concurrent callers via an internal
// singleflight group, and respect the caller's context for cancellation.
// Failed fetches are not cached.
type SingleFlightCache[T any] struct {
	fetch   func(tag string) (T, error)
	entries map[string]T
	sf      singleflight.Group
	mu      sync.Mutex
}

// NewSingleFlightCache creates a cache that uses fetch to populate missing entries.
func NewSingleFlightCache[T any](fetch func(tag string) (T, error)) *SingleFlightCache[T] {
	return &SingleFlightCache[T]{
		entries: make(map[string]T),
		fetch:   fetch,
	}
}

// Get returns the cached entry for a tag, fetching it if absent.
func (c *SingleFlightCache[T]) Get(ctx context.Context, tag string) (T, error) {
	c.mu.Lock()
	entry, ok := c.entries[tag]
	c.mu.Unlock()

	if ok {
		return entry, nil
	}

	resultCh := c.sf.DoChan(tag, func() (any, error) { //nolint:contextcheck
		item, err := c.fetch(tag)
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.entries[tag] = item
		c.mu.Unlock()

		return item, nil //nolint:nilnil
	})

	var zero T

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return zero, result.Err
		}
	}

	c.mu.Lock()
	entry = c.entries[tag]
	c.mu.Unlock()

	return entry, nil
}
