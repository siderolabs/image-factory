// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cache

import (
	"context"
	"sync"

	"golang.org/x/sync/singleflight"
)

// VersionedCache lazily fetches and caches a list of values keyed by a version tag.
//
// Fetches are de-duplicated across concurrent callers via an internal
// singleflight group, and respect the caller's context for cancellation.
// Failed fetches are not cached.
type VersionedCache[T any] struct {
	fetch   func(tag string) ([]T, error)
	entries map[string][]T
	sf      singleflight.Group
	mu      sync.Mutex
}

// NewVersionedCache creates a cache that uses fetch to populate missing entries.
func NewVersionedCache[T any](fetch func(tag string) ([]T, error)) *VersionedCache[T] {
	return &VersionedCache[T]{
		entries: make(map[string][]T),
		fetch:   fetch,
	}
}

// Get returns the cached entry for tag, fetching it if absent.
func (c *VersionedCache[T]) Get(ctx context.Context, tag string) ([]T, error) {
	c.mu.Lock()
	entry, ok := c.entries[tag]
	c.mu.Unlock()

	if ok {
		return entry, nil
	}

	resultCh := c.sf.DoChan(tag, func() (any, error) { //nolint:contextcheck
		items, err := c.fetch(tag)
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.entries[tag] = items
		c.mu.Unlock()

		return nil, nil //nolint:nilnil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
	}

	c.mu.Lock()
	entry = c.entries[tag]
	c.mu.Unlock()

	return entry, nil
}
