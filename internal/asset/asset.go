// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package asset implements generation of Talos build assets.
package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/reporter"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset/cache"
	factoryprofile "github.com/siderolabs/image-factory/internal/profile"
)

// BootAsset is an interface to access a boot asset.
//
// It is used to abstract the access to the boot asset, so that it can be
// implemented in different ways, such as a local file, a remote file.
type BootAsset interface {
	Size() int64
	Reader() (io.ReadCloser, error)
}

// Builder is the asset builder.
type Builder struct {
	metricConcurrencyLatency prometheus.Histogram
	cache                    cache.Cache
	metricBuildLatency       prometheus.Histogram
	sf                       singleflight.Group
	metricAssetsCached       *prometheus.CounterVec
	logger                   *zap.Logger
	metricAssetsBuilt        *prometheus.CounterVec
	metricAssetBytesCached   *prometheus.CounterVec
	metricAssetBytesBuilt    *prometheus.CounterVec
	metricAssetCachedErrors  *prometheus.CounterVec
	semaphore                chan struct{}
	artifactsManager         *artifacts.Manager
	cacheMightFail           bool
}

// Options configures the asset builder.
type Options struct {
	MetricsNamespace   string
	AllowedConcurrency int
	GetAfterPut        bool
}

// NewBuilder creates a new asset builder.
func NewBuilder(logger *zap.Logger, artifactsManager *artifacts.Manager, cache cache.Cache, options Options) (*Builder, error) {
	return &Builder{
		logger:           logger.With(zap.String("component", "asset-builder")),
		cache:            cache,
		artifactsManager: artifactsManager,
		semaphore:        make(chan struct{}, options.AllowedConcurrency),
		cacheMightFail:   options.GetAfterPut,

		metricAssetsCached: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "image_factory_assets_delivered_cached_total",
				Help:      "Number of assets retrieved from cache.",
				Namespace: options.MetricsNamespace,
			},
			[]string{"talos_version", "output_kind", "arch"},
		),
		metricAssetsBuilt: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "image_factory_assets_built_total",
				Help:      "Number of assets built (missing in the cache).",
				Namespace: options.MetricsNamespace,
			},
			[]string{"talos_version", "output_kind", "arch"},
		),
		metricAssetBytesCached: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "image_factory_assets_cached_bytes_total",
				Help:      "Number of bytes delivered with cached assets.",
				Namespace: options.MetricsNamespace,
			},
			[]string{"talos_version", "output_kind", "arch"},
		),
		metricAssetBytesBuilt: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "image_factory_assets_built_bytes_total",
				Help:      "Number of bytes delivered with assets built (missing in the cache).",
				Namespace: options.MetricsNamespace,
			},
			[]string{"talos_version", "output_kind", "arch"},
		),
		metricAssetCachedErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "image_factory_assets_cached_errors_total",
				Help:      "Number of errors retrieving assets from cache.",
				Namespace: options.MetricsNamespace,
			},
			[]string{"talos_version", "output_kind", "arch"},
		),
		metricConcurrencyLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:      "image_factory_assets_concurrency_latency_seconds",
				Help:      "Latency of asset build related to the concurrency limit (waiting for available workers).",
				Buckets:   []float64{1, 10, 60, 180, 600},
				Namespace: options.MetricsNamespace,
			},
		),
		metricBuildLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:      "image_factory_assets_build_latency_seconds",
				Help:      "Latency of asset build for the build itself (excluding waiting for available workers).",
				Buckets:   []float64{1, 10, 60, 180, 600},
				Namespace: options.MetricsNamespace,
			},
		),
	}, nil
}

func (b *Builder) getBuildAsset(ctx context.Context, versionString, arch string, kind artifacts.Kind, out *profile.FileAsset) error {
	var err error

	out.Path, err = b.artifactsManager.Get(ctx, versionString, artifacts.Arch(arch), kind)

	return err
}

// Build the asset.
//
// First, check if the asset has already been built and cached then use the cached version.
// If the asset hasn't been built yet, build it and cache it honoring the concurrency limit, and push it to the cache.
func (b *Builder) Build(ctx context.Context, prof profile.Profile, versionString, filename, filenameOverride string) (BootAsset, error) {
	profileHash, err := factoryprofile.Hash(prof)
	if err != nil {
		return nil, err
	}

	asset, err := b.cache.Get(ctx, profileHash)
	if err == nil {
		b.metricAssetsCached.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Inc()
		b.metricAssetBytesCached.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Add(float64(asset.Size()))

		return asset, nil
	}

	b.metricAssetCachedErrors.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Inc()

	if b.cacheMightFail {
		b.logger.Warn("unable to reach cache, falling back to direct delivery", zap.Error(err))
	} else if !errors.Is(err, cache.ErrCacheNotFound) {
		return nil, fmt.Errorf("error getting asset from cache: %w", err)
	}

	// nothing in cache, so build the asset, but make sure we do it only once
	ch := b.sf.DoChan(profileHash, func() (any, error) { //nolint:contextcheck
		return b.buildAndCache(profileHash, prof, versionString, filename)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}

		var ok bool

		asset, ok = res.Val.(BootAsset)
		if !ok {
			// unexpected
			return nil, fmt.Errorf("unexpected result type: %T", res.Val)
		}

		return asset, nil
	}
}

// buildAndCache builds the asset and pushes it to the cache.
func (b *Builder) buildAndCache(profileHash string, prof profile.Profile, versionString, filename string) (BootAsset, error) {
	// detach the context to make sure the asset is built no matter if the request is canceled
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	asset, err := b.build(ctx, prof, versionString)
	if err != nil {
		return nil, err
	}

	b.metricAssetsBuilt.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Inc()
	b.metricAssetBytesBuilt.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Add(float64(asset.Size()))

	if err = b.cache.Put(ctx, profileHash, asset, filename); err != nil {
		b.logger.Error("error putting asset to cache", zap.Error(err), zap.String("profile_hash", profileHash))
	}

	// if the asset delivery requires a redirect to the cache, we need to return the cached asset
	// so that the client can download it directly from the cache.
	if b.cacheMightFail {
		cachedAsset, err := b.cache.Get(ctx, profileHash)
		if err == nil {
			b.metricAssetsCached.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Inc()
			b.metricAssetBytesCached.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Add(float64(asset.Size()))

			return cachedAsset, nil
		}

		// if we failed to get the asset from cache, return the built asset anyway
		b.logger.Warn("failed to get object from cache, falling back to direct delivery", zap.Error(err))

		b.metricAssetCachedErrors.WithLabelValues(versionString, prof.Output.Kind.String(), prof.Arch).Inc()
	}

	return asset, nil
}

// build the asset using Talos imager.
//
// A concurrency limit is enforced.
func (b *Builder) build(ctx context.Context, prof profile.Profile, versionString string) (BootAsset, error) {
	start := time.Now()

	// enforce concurrency limit
	select {
	case b.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	defer func() {
		<-b.semaphore
	}()

	concurrencyLatency := time.Since(start)
	b.logger.Info("building image asset", zap.Any("profile", prof), zap.String("version", versionString), zap.Duration("concurrency_latency", concurrencyLatency))
	b.metricConcurrencyLatency.Observe(concurrencyLatency.Seconds())

	if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindKernel, &prof.Input.Kernel); err != nil {
		return nil, fmt.Errorf("failed to get kernel: %w", err)
	}

	if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindInitramfs, &prof.Input.Initramfs); err != nil {
		return nil, fmt.Errorf("failed to get initramfs: %w", err)
	}

	if quirks.New(versionString).SupportsUKI() {
		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindSystemdBoot, &prof.Input.SDBoot); err != nil {
			return nil, fmt.Errorf("failed to get systemd-boot: %w", err)
		}

		if err := b.getBuildAsset(ctx, versionString, prof.Arch, artifacts.KindSystemdStub, &prof.Input.SDStub); err != nil {
			return nil, fmt.Errorf("failed to get systemd-stub: %w", err)
		}
	}

	imgr, err := imager.New(prof)
	if err != nil {
		return nil, err
	}

	tmpDir, err := newTmpDir()
	if err != nil {
		return nil, err
	}

	tmpDir.assetPath, err = imgr.Execute(ctx, tmpDir.directoryPath, reporter.New())
	if err != nil {
		return nil, fmt.Errorf("error generating asset: %w", err)
	}

	st, err := os.Stat(tmpDir.assetPath)
	if err != nil {
		return nil, fmt.Errorf("error getting asset size: %w", err)
	}

	tmpDir.size = st.Size()

	buildLatency := time.Since(start) - concurrencyLatency
	b.logger.Info("finished building image asset", zap.Any("profile", prof), zap.String("version", versionString), zap.Duration("build_latency", buildLatency))
	b.metricBuildLatency.Observe(buildLatency.Seconds())

	return tmpDir, nil
}

// Describe implements prom.Collector interface.
func (b *Builder) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(b, ch)
}

// Collect implements prom.Collector interface.
func (b *Builder) Collect(ch chan<- prometheus.Metric) {
	b.metricAssetsBuilt.Collect(ch)
	b.metricAssetsCached.Collect(ch)

	b.metricAssetBytesBuilt.Collect(ch)
	b.metricAssetBytesCached.Collect(ch)

	b.metricAssetCachedErrors.Collect(ch)

	b.metricBuildLatency.Collect(ch)
	b.metricConcurrencyLatency.Collect(ch)
}

var _ prometheus.Collector = (*Builder)(nil)
