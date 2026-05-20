// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache implements an in-memory cache over schematic storage.
package cache

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"

	commoncache "github.com/siderolabs/image-factory/internal/cache"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
)

// Options configures the storage.
type Options struct {
	MetricsNamespace string
	CacheCapacity    uint64
	NegativeTTL      time.Duration
}

// Storage is a schematic storage in-memory cache.
type Storage struct {
	c          *commoncache.Cache[string, optional.Optional[[]byte]]
	underlying storage.Storage

	negativeTTL time.Duration
}

// NewCache returns a new cache storage.
func NewCache(underlying storage.Storage, options Options) *Storage {
	return &Storage{
		underlying:  underlying,
		negativeTTL: options.NegativeTTL,
		c: commoncache.New[string, optional.Optional[[]byte]](commoncache.Options{
			MetricsNamespace: options.MetricsNamespace,
			MetricsName:      "image_factory_schematic_cache_size",
			MetricsHelp:      "Number of schematics in in-memory cache.",
			Capacity:         options.CacheCapacity,
		}),
	}
}

// Start the cache invalidation, should be run in a goroutine.
func (s *Storage) Start() error {
	return s.c.Start()
}

// Stop the cache invalidation background goroutine.
func (s *Storage) Stop() {
	s.c.Stop()
}

// Check interface.
var _ storage.Storage = (*Storage)(nil)

// Head checks if the schematic exists.
func (s *Storage) Head(ctx context.Context, id string) error {
	// check cache
	item := s.c.TTL.Get(id)

	// cache entry is there, return immediate response
	if item != nil && !item.IsExpired() {
		if item.Value().IsPresent() {
			return nil
		}

		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	// cache entry is not there, use .Get to populate it
	_, err := s.Get(ctx, id)

	return err
}

// Get returns the schematic.
func (s *Storage) Get(ctx context.Context, id string) ([]byte, error) {
	// check cache
	item := s.c.TTL.Get(id)

	// cache entry is there, return immediate response
	if item != nil && !item.IsExpired() {
		if item.Value().IsPresent() {
			return item.Value().ValueOrZero(), nil
		}

		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	ch := s.c.SF.DoChan(id, func() (any, error) {
		data, err := s.underlying.Get(ctx, id)
		if err != nil {
			if xerrors.TagIs[storage.ErrNotFoundTag](err) {
				// never overwrite a present value, as Put might have been called
				s.c.TTL.GetOrSet(
					id, optional.None[[]byte](),
					ttlcache.WithTTL[string, optional.Optional[[]byte]](s.negativeTTL),
				)
			}

			return nil, err
		}

		// never overwrite a present value, as Put might have been called
		item, _ := s.c.TTL.GetOrSet(id, optional.Some(data))

		return item.Value().ValueOr(data), nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.Err != nil {
			return nil, r.Err
		}

		return r.Val.([]byte), nil //nolint:forcetypeassert,errcheck
	}
}

// Put stores the schematic.
func (s *Storage) Put(ctx context.Context, id string, data []byte) error {
	err := s.underlying.Put(ctx, id, data)
	if err != nil {
		return err
	}

	s.c.TTL.Set(id, optional.Some(data), -1) // never expire, schematics are immutable

	return nil
}

// Describe implements prom.Collector interface.
func (s *Storage) Describe(ch chan<- *prometheus.Desc) {
	s.c.Describe(ch)
}

// Collect implements prom.Collector interface.
func (s *Storage) Collect(ch chan<- prometheus.Metric) {
	s.c.Collect(ch)
}

var _ prometheus.Collector = &Storage{}
