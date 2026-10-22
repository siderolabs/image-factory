// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry implements a cache using an OCI registry.
package registry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Options contains options for the registry cache.
type Options struct {
	CacheRepository         name.Repository
	NameOptions             []name.Option
	CacheImageSigner        signer.Signer
	RemoteOptions           []remote.Option
	RegistryRefreshInterval time.Duration
}

// Cache is using OCI registry to cache assets.
type Cache struct {
	puller          remotewrap.Puller
	pusher          remotewrap.Pusher
	imageSigner     signer.Signer
	logger          *zap.Logger
	cacheRepository name.Repository
}

// Check interface.
var _ cache.Cache = (*Cache)(nil)

// New creates a new registry cache.
func New(logger *zap.Logger, options Options) (*Cache, error) {
	c := &Cache{
		cacheRepository: options.CacheRepository,
		logger:          logger.With(zap.String("component", "asset-cache-registry")),
	}

	var err error

	c.puller, err = remotewrap.NewPuller(options.RegistryRefreshInterval, options.NameOptions, options.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("error creating puller: %w", err)
	}

	c.pusher, err = remotewrap.NewPusher(options.RegistryRefreshInterval, options.NameOptions, options.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("error creating pusher: %w", err)
	}

	c.imageSigner = options.CacheImageSigner

	return c, nil
}

// Get returns the boot asset from the cache.
func (c *Cache) Get(ctx context.Context, profileID string) (cache.BootAsset, error) {
	taggedRef := c.cacheRepository.Tag(profileID)

	// A cache hit does Head plus a signature verification, and either can dominate the
	// request. Time them separately, and carry the request_id, so a slow hit is attributable
	// to a phase and a caller rather than showing up as an unexplained gap.
	logger := ctxlog.Logger(ctx, c.logger)

	logger.Debug("heading cached image", zap.Stringer("ref", taggedRef))

	headStart := time.Now()

	desc, err := c.puller.Head(ctx, taggedRef)

	headLatency := time.Since(headStart)

	switch {
	case regtransport.IsStatusCodeError(err, http.StatusNotFound):
		// the image hasn't been pushed yet
		return nil, cache.ErrCacheNotFound
	case regtransport.IsStatusCodeError(err, http.StatusForbidden):
		// A 403 can mean an absent tag or a bad credential, so it stays a cache miss -- but
		// log it, otherwise an unreadable cache looks exactly like a cold one.
		logger.Warn("cache image not readable, treating as a cache miss", zap.Stringer("ref", taggedRef))

		return nil, cache.ErrCacheNotFound
	case err != nil:
		// something is wrong
		return nil, fmt.Errorf("failed to head cache image: %w", err)
	}

	digestRef := c.cacheRepository.Digest(desc.Digest.String())

	verifyStart := time.Now()

	err = c.imageSigner.VerifyImage(ctx, digestRef, c.puller)

	verifyLatency := time.Since(verifyStart)

	if err != nil {
		// a verification cut short by cancellation says nothing about the signature: report
		// the cancellation rather than a miss, so the caller doesn't rebuild a valid entry.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		// signature doesn't validate, skip the cache, but keep building
		logger.Info("cache image signature doesn't validate", zap.Error(err), zap.Stringer("ref", taggedRef),
			zap.Duration("head_latency", headLatency), zap.Duration("verify_latency", verifyLatency))

		return nil, cache.ErrCacheNotFound
	}

	logger.Info("using cached image", zap.Stringer("ref", taggedRef),
		zap.Duration("head_latency", headLatency), zap.Duration("verify_latency", verifyLatency))

	imgDesc, err := c.puller.Get(ctx, digestRef)
	if err != nil {
		return nil, fmt.Errorf("failed to pull cache image: %w", err)
	}

	img, err := imgDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to create cache image from descriptor: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache image layers: %w", err)
	}

	if len(layers) != 1 {
		return nil, fmt.Errorf("unexpected number of cache image layers: %d", len(layers))
	}

	layer := layers[0]

	size, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache image layer size: %w", err)
	}

	return &remoteAsset{
		layer: layer,
		size:  size,
	}, nil
}

// Put uploads the boot asset to the registry.
func (c *Cache) Put(ctx context.Context, profileID string, asset cache.BootAsset, _ string) error {
	taggedRef := c.cacheRepository.Tag(profileID)

	logger := ctxlog.Logger(ctx, c.logger)

	logger.Info("pushing cached image", zap.Stringer("ref", taggedRef))

	layer, err := partial.CompressedToLayer(&layerWrapper{
		src: asset,
	})
	if err != nil {
		return err
	}

	// we don't need to push an image manifest, but we create it to make sure that a schematic blob (layer)
	// doesn't get GC'ed by the registry
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return err
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("failed to get cache image digest: %w", err)
	}

	digestRef := c.cacheRepository.Digest(digest.String())

	// Push by digest, sign, and only then move the tag. Get resolves the tag and rejects an
	// unsigned digest as a miss, so tagging first left a window -- the length of a keyless
	// signing, seconds -- in which every request for the asset missed and rebuilt it, each
	// rebuild re-opening the window for the next.
	if err = c.pusher.Push(ctx, digestRef, img); err != nil {
		return fmt.Errorf("failed to push cache image: %w", err)
	}

	if err = c.sign(ctx, digestRef); err != nil {
		return err
	}

	if err = c.pusher.Push(ctx, taggedRef, img); err != nil {
		return fmt.Errorf("failed to tag cache image: %w", err)
	}

	return nil
}

func (c *Cache) sign(ctx context.Context, digestRef name.Digest) error {
	logger := ctxlog.Logger(ctx, c.logger)

	// A cache manifest is content-addressed, so one signature over the digest stays valid for
	// the life of the entry, and a miss on an entry that is already in the registry (a new
	// Talos version whose asset is byte-identical, an object that lost its S3 metadata, a
	// transient Get error) must not sign it again. Keyless signing writes each signature as a
	// separate referrer, so re-signing grows a list that every later cache read has to walk:
	// the metal-amd64 cmdline entry reached 205 referrers at ~0.68s each, a 140s cache hit.
	//
	// Asking the signer, rather than looking for an attached artifact, is what makes skipping
	// safe: a referrer that is an SBOM, or a signature made under a since-rotated identity,
	// does not satisfy the same check Get performs, so it is signed again instead of leaving
	// an entry that never validates. Any error here means "not signed", so the failure
	// direction is always to sign.
	if err := c.imageSigner.VerifyImage(ctx, digestRef, c.puller); err == nil {
		logger.Debug("cache image is already signed", zap.Stringer("ref", digestRef))

		return nil
	}

	logger.Info("signing cache image", zap.Stringer("ref", digestRef))

	if err := c.imageSigner.SignImage(
		ctx,
		digestRef,
		c.pusher,
	); err != nil {
		return fmt.Errorf("error signing cached image: %w", err)
	}

	return nil
}
