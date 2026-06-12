// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache provides reusable in-memory caching primitives.
//
// Cache is a capacity-bounded TTL cache with Prometheus metrics and
// singleflight de-duplication; consumers compose their own value semantics
// (negative caching, optional wrapping, etc.) on top of it.
//
// VersionedCache lazily fetches and caches lists of values keyed by a version
// tag, de-duplicating concurrent loads and respecting context cancellation.
package cache

import (
	"github.com/jellydator/ttlcache/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/panicsafe"
	"golang.org/x/sync/singleflight"
)

// Options configures a Cache.
type Options struct {
	// MetricsNamespace is prepended to MetricsName in the exported gauge.
	MetricsNamespace string

	// MetricsName is the Prometheus gauge name for the cache size.
	MetricsName string

	// MetricsHelp is the Prometheus gauge help string.
	MetricsHelp string

	// Capacity caps the number of entries; LRU eviction kicks in at the limit.
	Capacity uint64
}

// Cache is a capacity-bounded TTL cache with a Prometheus size gauge and a
// singleflight group for de-duplicating concurrent loads.
//
// TTL and SF are exposed so consumers can use the underlying ttlcache and
// singleflight APIs directly without thin re-wrapping.
type Cache[K comparable, V any] struct {
	TTL *ttlcache.Cache[K, V]
	SF  singleflight.Group

	metricSize prometheus.Gauge
}

// New constructs a Cache with WithDisableTouchOnHit (pure TTL, no LRU touch).
func New[K comparable, V any](options Options) *Cache[K, V] {
	return &Cache[K, V]{
		TTL: ttlcache.New[K, V](
			ttlcache.WithDisableTouchOnHit[K, V](),
			ttlcache.WithCapacity[K, V](options.Capacity),
		),
		metricSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      options.MetricsName,
			Help:      options.MetricsHelp,
			Namespace: options.MetricsNamespace,
		}),
	}
}

// Start runs the cache eviction goroutine; should be invoked in a goroutine.
func (c *Cache[K, V]) Start() error {
	return panicsafe.Run(c.TTL.Start)
}

// Stop halts the eviction goroutine.
func (c *Cache[K, V]) Stop() {
	c.TTL.Stop()
}

// Describe implements prom.Collector.
func (c *Cache[K, V]) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect implements prom.Collector.
func (c *Cache[K, V]) Collect(ch chan<- prometheus.Metric) {
	c.metricSize.Set(float64(c.TTL.Len()))

	c.metricSize.Collect(ch)
}

var _ prometheus.Collector = (*Cache[string, struct{}])(nil)
