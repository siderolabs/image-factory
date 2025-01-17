// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"go.uber.org/zap"

	"github.com/skyssolutions/siderolabs-image-factory/internal/image/signer"
	"github.com/skyssolutions/siderolabs-image-factory/internal/regtransport"
)

// registryCache is using OCI registry to cache assets.
type registryCache struct {
	puller          *remote.Puller
	pusher          *remote.Pusher
	imageSigner     *signer.Signer
	logger          *zap.Logger
	cacheRepository name.Repository
}

var errCacheNotFound = errors.New("not found in cache")

// Get returns the boot asset from the cache.
func (r *registryCache) Get(ctx context.Context, profileID string) (BootAsset, error) {
	taggedRef := r.cacheRepository.Tag(profileID)

	r.logger.Debug("heading cached image", zap.Stringer("ref", taggedRef))

	desc, err := r.puller.Head(ctx, taggedRef)

	if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
		// ignore 404/403, it means the image hasn't been pushed yet
		return nil, errCacheNotFound
	}

	if err != nil {
		// something is wrong
		return nil, fmt.Errorf("failed to head cache image: %w", err)
	}

	digestRef := r.cacheRepository.Digest(desc.Digest.String())

	_, _, err = cosign.VerifyImageSignatures(
		ctx,
		digestRef,
		r.imageSigner.GetCheckOpts(),
	)
	if err != nil {
		// signature doesn't validate, skip the cache, but keep building
		r.logger.Info("cache image signature doesn't validate", zap.Error(err), zap.Stringer("ref", taggedRef))

		return nil, errCacheNotFound
	}

	r.logger.Info("using cached image", zap.Stringer("ref", taggedRef))

	imgDesc, err := r.puller.Get(ctx, digestRef)
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
func (r *registryCache) Put(ctx context.Context, profileID string, asset BootAsset) error {
	taggedRef := r.cacheRepository.Tag(profileID)

	r.logger.Info("pushing cached image", zap.Stringer("ref", taggedRef))

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

	if err = r.pusher.Push(ctx, taggedRef, img); err != nil {
		return fmt.Errorf("failed to push cache image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("failed to get cache image digest: %w", err)
	}

	digestRef := r.cacheRepository.Digest(digest.String())

	r.logger.Info("signing cache image", zap.Stringer("ref", digestRef))

	if err := r.imageSigner.SignImage(
		ctx,
		digestRef,
		r.pusher,
	); err != nil {
		return fmt.Errorf("error signing cached image: %w", err)
	}

	return nil
}
