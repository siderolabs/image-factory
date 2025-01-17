// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache implements an in-memory cache over schematic storage.
package cache

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"golang.org/x/sync/singleflight"

	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic/storage"
)

// Storage is a schematic storage in-memory cache.
type Storage struct {
	underlying storage.Storage

	metricCacheSize prometheus.Gauge

	g  singleflight.Group
	m  map[string]optional.Optional[[]byte]
	mu sync.Mutex
}

// NewCache returns a new cache storage.
func NewCache(underlying storage.Storage) *Storage {
	return &Storage{
		underlying: underlying,
		m:          map[string]optional.Optional[[]byte]{},
		metricCacheSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "image_factory_schematic_cache_size",
			Help: "Number of schematics in in-memory cache.",
		}),
	}
}

// Check interface.
var _ storage.Storage = (*Storage)(nil)

// Head checks if the schematic exists.
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

		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	// cache entry is not there, use .Get to populate it
	_, err := s.Get(ctx, id)

	return err
}

// Get returns the schematic.
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

		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
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

		return r.Val.([]byte), nil //nolint:forcetypeassert,errcheck
	}
}

// Put stores the schematic.
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

// Describe implements prom.Collector interface.
func (s *Storage) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(s, ch)
}

// Collect implements prom.Collector interface.
func (s *Storage) Collect(ch chan<- prometheus.Metric) {
	s.mu.Lock()
	s.metricCacheSize.Set(float64(len(s.m)))
	s.mu.Unlock()

	s.metricCacheSize.Collect(ch)
}

var _ prometheus.Collector = &Storage{}
