// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry implements a cache using an OCI registry.
package registry

import (
	"context"
	"crypto"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Options contains options for the registry cache.
type Options struct {
	CacheRepository         name.Repository
	CacheSigningKey         crypto.PrivateKey
	RemoteOptions           []remote.Option
	RegistryRefreshInterval time.Duration
}

// Cache is using OCI registry to cache assets.
type Cache struct {
	puller          remotewrap.Puller
	pusher          remotewrap.Pusher
	imageSigner     *signer.Signer
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

	c.puller, err = remotewrap.NewPuller(options.RegistryRefreshInterval, options.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating puller: %w", err)
	}

	c.pusher, err = remotewrap.NewPusher(options.RegistryRefreshInterval, options.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating pusher: %w", err)
	}

	c.imageSigner, err = signer.NewSigner(options.CacheSigningKey)
	if err != nil {
		return nil, fmt.Errorf("error creating signer: %w", err)
	}

	return c, nil
}

// Get returns the boot asset from the cache.
func (c *Cache) Get(ctx context.Context, profileID string) (cache.BootAsset, error) {
	taggedRef := c.cacheRepository.Tag(profileID)

	c.logger.Debug("heading cached image", zap.Stringer("ref", taggedRef))

	desc, err := c.puller.Head(ctx, taggedRef)
	if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
		// ignore 404/403, it means the image hasn't been pushed yet
		return nil, cache.ErrCacheNotFound
	}

	if err != nil {
		// something is wrong
		return nil, fmt.Errorf("failed to head cache image: %w", err)
	}

	digestRef := c.cacheRepository.Digest(desc.Digest.String())

	_, _, err = cosign.VerifyImageSignatures(
		ctx,
		digestRef,
		c.imageSigner.GetCheckOpts(),
	)
	if err != nil {
		// signature doesn't validate, skip the cache, but keep building
		c.logger.Info("cache image signature doesn't validate", zap.Error(err), zap.Stringer("ref", taggedRef))

		return nil, cache.ErrCacheNotFound
	}

	c.logger.Info("using cached image", zap.Stringer("ref", taggedRef))

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
func (c *Cache) Put(ctx context.Context, profileID string, asset cache.BootAsset) error {
	taggedRef := c.cacheRepository.Tag(profileID)

	c.logger.Info("pushing cached image", zap.Stringer("ref", taggedRef))

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

	if err = c.pusher.Push(ctx, taggedRef, img); err != nil {
		return fmt.Errorf("failed to push cache image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("failed to get cache image digest: %w", err)
	}

	digestRef := c.cacheRepository.Digest(digest.String())

	c.logger.Info("signing cache image", zap.Stringer("ref", digestRef))

	if err := c.imageSigner.SignImage(
		ctx,
		digestRef,
		c.pusher,
	); err != nil {
		return fmt.Errorf("error signing cached image: %w", err)
	}

	return nil
}
