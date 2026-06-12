// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package builder produces VEX documents from a signed OCI VEX data image.
package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/go-vex/pkg/types/v1alpha1"
	"github.com/siderolabs/go-vex/pkg/vexgen"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/cache"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/image/verify"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/pkg/constants"
)

// FetchTimeout caps an OCI fetch + signature verification.
const FetchTimeout = 5 * time.Minute

// DefaultDataTag is the OCI tag pulled when no override is configured.
const DefaultDataTag = "latest"

// Builder produces VEX documents for a Talos version, with TTL caching and singleflight.
type Builder struct {
	puller        remotewrap.Puller
	logger        *zap.Logger
	c             *cache.Cache[string, []byte]
	registry      string
	dataTag       string
	verifyOptions verify.VerifyOptions
	cacheTTL      time.Duration
	insecure      bool
}

// Options configures Builder.
type Options struct {
	Registry         string
	DataTag          string
	MetricsNamespace string
	RemoteOptions    []remote.Option
	VerifyOptions    verify.VerifyOptions
	RefreshInterval  time.Duration
	CacheTTL         time.Duration
	Capacity         uint64
	Insecure         bool
}

// NewBuilder constructs a Builder.
func NewBuilder(logger *zap.Logger, opts Options) (*Builder, error) {
	puller, err := remotewrap.NewPuller(opts.RefreshInterval, opts.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating puller: %w", err)
	}

	dataTag := opts.DataTag
	if dataTag == "" {
		dataTag = DefaultDataTag
	}

	return &Builder{
		puller:        puller,
		registry:      opts.Registry,
		dataTag:       dataTag,
		insecure:      opts.Insecure,
		verifyOptions: opts.VerifyOptions,
		cacheTTL:      opts.CacheTTL,
		c: cache.New[string, []byte](cache.Options{
			MetricsNamespace: opts.MetricsNamespace,
			MetricsName:      "image_factory_vex_cache_size",
			MetricsHelp:      "Number of VEX documents in in-memory cache.",
			Capacity:         opts.Capacity,
		}),
		logger: logger.With(zap.String("component", "vex-builder")),
	}, nil
}

// Start runs the cache eviction goroutine; should be invoked in a goroutine.
func (b *Builder) Start() error {
	return b.c.Start()
}

// Stop halts the cache eviction goroutine.
func (b *Builder) Stop() {
	b.c.Stop()
}

// Build returns a serialized VEX JSON document for the given Talos version tag.
//
// Cached per versionTag with TTL. Concurrent calls for the same tag share one OCI fetch
// via singleflight. The fetch runs under a detached context so request cancellations
// don't poison the shared work.
func (b *Builder) Build(ctx context.Context, versionTag string) ([]byte, error) {
	if item := b.c.TTL.Get(versionTag); item != nil && !item.IsExpired() {
		return item.Value(), nil
	}

	// carry the request ID into the detached build so its logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)

	resultCh := b.c.SF.DoChan(versionTag, func() (any, error) { //nolint:contextcheck
		return b.buildAndCache(reqID, versionTag)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		if res.Err != nil {
			return nil, res.Err
		}

		data, ok := res.Val.([]byte)
		if !ok {
			return nil, fmt.Errorf("unexpected result type: %T", res.Val)
		}

		return data, nil
	}
}

// buildAndCache runs under singleflight with a detached context.
//
// reqID is the request ID, carried into the detached context so the build logs
// keep the request_id.
func (b *Builder) buildAndCache(reqID, versionTag string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctxlog.WithRequestID(context.Background(), reqID), FetchTimeout)
	defer cancel()

	expData, err := b.fetchExploitabilityData(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching VEX data: %w", err)
	}

	now := time.Now()

	doc, err := vexgen.Populate(expData, versionTag, &now, "image-factory")
	if err != nil {
		return nil, fmt.Errorf("error generating VEX document: %w", err)
	}

	var buf bytes.Buffer
	if err = vexgen.Serialize(doc, &buf); err != nil {
		return nil, fmt.Errorf("error serializing VEX document: %w", err)
	}

	data := buf.Bytes()
	b.c.TTL.Set(versionTag, data, b.cacheTTL)

	return data, nil
}

// Describe implements prom.Collector interface.
func (b *Builder) Describe(ch chan<- *prometheus.Desc) {
	b.c.Describe(ch)
}

// Collect implements prom.Collector interface.
func (b *Builder) Collect(ch chan<- prometheus.Metric) {
	b.c.Collect(ch)
}

var _ prometheus.Collector = (*Builder)(nil)

// fetchExploitabilityData heads the configured OCI tag, verifies the signature on the resolved digest,
// pulls the image, and extracts the first regular file from the first layer.
func (b *Builder) fetchExploitabilityData(ctx context.Context) (*v1alpha1.ExploitabilityData, error) {
	var nameOpts []name.Option

	if b.insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}

	tagRef, err := name.NewTag(fmt.Sprintf("%s:%s", b.registry, b.dataTag), nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("error parsing reference: %w", err)
	}

	descriptor, err := b.puller.Head(ctx, tagRef)
	if err != nil {
		return nil, fmt.Errorf("error heading VEX image: %w", err)
	}

	digestRef := tagRef.Digest(descriptor.Digest.String())

	logger := ctxlog.Logger(ctx, b.logger).With(zap.Stringer("image", digestRef))

	logger.Debug("verifying VEX image signature")

	verifyResult, err := verify.VerifySignatures(ctx, digestRef, b.verifyOptions, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to verify VEX image signature for %s: %w", digestRef.Name(), err)
	}

	logger.Info("VEX image signature verified",
		zap.String("verification_method", verifyResult.Method),
		zap.Bool("bundle_verified", verifyResult.Verified))

	imgDesc, err := b.puller.Get(ctx, digestRef)
	if err != nil {
		return nil, fmt.Errorf("error fetching VEX image: %w", err)
	}

	img, err := imgDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("error reading VEX image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("error getting VEX layers: %w", err)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("no layers found in VEX data image")
	}

	reader, err := layers[0].Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("error getting layer content: %w", err)
	}
	defer reader.Close() //nolint:errcheck

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("error reading VEX data archive: %w", err)
		}

		if header.Typeflag == tar.TypeReg {
			return v1alpha1.LoadExploitabilityData(tarReader, v1alpha1.WithPURLOverride(constants.TalosPURL))
		}
	}

	return nil, fmt.Errorf("no data file found in VEX data image")
}
