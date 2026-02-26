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
	"github.com/siderolabs/gen/panicsafe"
	"github.com/siderolabs/gen/xerrors"
	"golang.org/x/sync/singleflight"

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
	c          *ttlcache.Cache[string, optional.Optional[[]byte]]
	underlying storage.Storage

	metricCacheSize prometheus.Gauge

	g singleflight.Group

	negativeTTL time.Duration
}

// NewCache returns a new cache storage.
func NewCache(underlying storage.Storage, options Options) *Storage {
	return &Storage{
		underlying:  underlying,
		negativeTTL: options.NegativeTTL,
		c: ttlcache.New[string, optional.Optional[[]byte]](
			ttlcache.WithDisableTouchOnHit[string, optional.Optional[[]byte]](),
			ttlcache.WithCapacity[string, optional.Optional[[]byte]](options.CacheCapacity),
		),
		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "image_factory_schematic_cache_size",
			Help:      "Number of schematics in in-memory cache.",
			Namespace: options.MetricsNamespace,
		}),
	}
}

// Start the cache invalidation, should be run in a goroutine.
func (s *Storage) Start() error {
	return panicsafe.Run(s.c.Start)
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
	item := s.c.Get(id)

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
	item := s.c.Get(id)

	// cache entry is there, return immediate response
	if item != nil && !item.IsExpired() {
		if item.Value().IsPresent() {
			return item.Value().ValueOrZero(), nil
		}

		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	ch := s.g.DoChan(id, func() (any, error) {
		data, err := s.underlying.Get(ctx, id)
		if err != nil {
			if xerrors.TagIs[storage.ErrNotFoundTag](err) {
				// never overwrite a present value, as Put might have been called
				s.c.GetOrSet(id, optional.None[[]byte](),
					ttlcache.WithTTL[string, optional.Optional[[]byte]](s.negativeTTL),
				)
			}

			return nil, err
		}

		// never overwrite a present value, as Put might have been called
		item, _ := s.c.GetOrSet(id, optional.Some(data))

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

	s.c.Set(id, optional.Some(data), -1) // never expire, schematics are immutable

	return nil
}

// Describe implements prom.Collector interface.
func (s *Storage) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(s, ch)
}

// Collect implements prom.Collector interface.
func (s *Storage) Collect(ch chan<- prometheus.Metric) {
	s.metricCacheSize.Set(float64(s.c.Len()))

	s.metricCacheSize.Collect(ch)
}

var _ prometheus.Collector = &Storage{}
