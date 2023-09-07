// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache implements an in-memory cache over flavor storage.
package cache

import (
	"context"
	"sync"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-service/internal/flavor/storage"
)

// Storage is a flavor storage in-memory cache.
type Storage struct {
	underlying storage.Storage

	g  singleflight.Group
	m  map[string]optional.Optional[[]byte]
	mu sync.Mutex
}

// NewCache returns a new cache storage.
func NewCache(underlying storage.Storage) *Storage {
	return &Storage{
		underlying: underlying,
		m:          map[string]optional.Optional[[]byte]{},
	}
}

// Check interface.
var _ storage.Storage = (*Storage)(nil)

// Head checks if the flavor exists.
func (s *Storage) Head(ctx context.Context, id string) error {
	// check cache
	s.mu.Lock()
	v, ok := s.m[id]
	s.mu.Unlock()

	// cache entry is there, return immediate response
	if ok {
		if v.IsPresent() {
			return nil
		}

		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("flavor ID %q not found", id)
	}

	// cache entry is not there, use .Get to populate it
	_, err := s.Get(ctx, id)

	return err
}

// Get returns the flavor.
func (s *Storage) Get(ctx context.Context, id string) ([]byte, error) {
	// check cache
	s.mu.Lock()
	v, ok := s.m[id]
	s.mu.Unlock()

	// cache entry is there, return immediate response
	if ok {
		if v.IsPresent() {
			return v.ValueOrZero(), nil
		}

		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("flavor ID %q not found", id)
	}

	ch := s.g.DoChan(id, func() (any, error) {
		data, err := s.underlying.Get(ctx, id)
		if err != nil {
			if xerrors.TagIs[storage.ErrNotFoundTag](err) {
				s.mu.Lock()

				// never overwrite a present value, as Put might have been called
				if _, ok := s.m[id]; !ok {
					s.m[id] = optional.None[[]byte]()
				}

				s.mu.Unlock()
			}

			return nil, err
		}

		s.mu.Lock()

		// never overwrite a present value, as Put might have been called
		if _, ok := s.m[id]; !ok {
			s.m[id] = optional.Some(data)
		}

		s.mu.Unlock()

		return data, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.Err != nil {
			return nil, r.Err
		}

		return r.Val.([]byte), nil //nolint:forcetypeassert
	}
}

// Put stores the flavor.
func (s *Storage) Put(ctx context.Context, id string, data []byte) error {
	err := s.underlying.Put(ctx, id, data)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.m[id] = optional.Some(data)
	s.mu.Unlock()

	return nil
}
