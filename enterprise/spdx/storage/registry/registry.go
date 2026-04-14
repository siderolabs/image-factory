// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package registry implements SPDX bundle storage in OCI registry.
package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
	"github.com/siderolabs/image-factory/enterprise/spdx/storage"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// SPDXBundleMediaType is the media type for SPDX bundles stored in the OCI registry.
const SPDXBundleMediaType types.MediaType = "application/vnd.sidero.dev-image.spdx-bundle+json"

// Options contains options for the registry storage.
type Options struct {
	CacheRepository         name.Repository
	CacheImageSigner        signer.Signer
	RemoteOptions           []remote.Option
	RegistryRefreshInterval time.Duration
}

// Storage implements SPDX bundle storage using OCI registry.
type Storage struct {
	puller          remotewrap.Puller
	pusher          remotewrap.Pusher
	imageSigner     signer.Signer
	logger          *zap.Logger
	cacheRepository name.Repository
}

// Check interface.
var _ storage.Storage = (*Storage)(nil)

// NewStorage creates a new registry storage.
func NewStorage(logger *zap.Logger, options Options) (*Storage, error) {
	s := &Storage{
		cacheRepository: options.CacheRepository,
		logger:          logger.With(zap.String("component", "spdx-storage-registry")),
	}

	var err error

	s.puller, err = remotewrap.NewPuller(options.RegistryRefreshInterval, options.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating puller: %w", err)
	}

	s.pusher, err = remotewrap.NewPusher(options.RegistryRefreshInterval, options.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating pusher: %w", err)
	}

	s.imageSigner = options.CacheImageSigner

	return s, nil
}

// Head checks if an SPDX bundle exists for the given schematic, version and architecture.
func (s *Storage) Head(ctx context.Context, schematicID, version, arch string) error {
	tag := builder.CacheTag(schematicID, version, arch)
	taggedRef := s.cacheRepository.Tag(tag)

	s.logger.Debug("heading SPDX bundle", zap.Stringer("ref", taggedRef))

	_, err := s.puller.Head(ctx, taggedRef)
	if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("SPDX bundle for schematic %q version %q arch %q not found", schematicID, version, arch)
	}

	if err != nil {
		return fmt.Errorf("failed to head SPDX bundle: %w", err)
	}

	return nil
}

// Get retrieves an SPDX bundle for the given schematic, version and architecture.
func (s *Storage) Get(ctx context.Context, schematicID, version, arch string) (storage.Bundle, error) {
	tag := builder.CacheTag(schematicID, version, arch)
	taggedRef := s.cacheRepository.Tag(tag)

	s.logger.Debug("getting SPDX bundle", zap.Stringer("ref", taggedRef))

	desc, err := s.puller.Head(ctx, taggedRef)
	if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("SPDX bundle for schematic %q version %q arch %q not found", schematicID, version, arch)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to head SPDX bundle: %w", err)
	}

	digestRef := s.cacheRepository.Digest(desc.Digest.String())

	// Verify signature
	err = s.imageSigner.VerifyImage(ctx, digestRef)
	if err != nil {
		s.logger.Warn("SPDX bundle signature doesn't validate", zap.Error(err), zap.Stringer("ref", taggedRef))

		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("SPDX bundle signature verification failed")
	}

	s.logger.Info("using cached SPDX bundle", zap.Stringer("ref", taggedRef))

	imgDesc, err := s.puller.Get(ctx, digestRef)
	if err != nil {
		return nil, fmt.Errorf("failed to pull SPDX bundle: %w", err)
	}

	img, err := imgDesc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to create image from descriptor: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get image layers: %w", err)
	}

	if len(layers) != 1 {
		return nil, fmt.Errorf("unexpected number of layers: %d", len(layers))
	}

	layer := layers[0]

	size, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer size: %w", err)
	}

	return &remoteBundle{
		layer: layer,
		size:  size,
	}, nil
}

// Put stores an SPDX bundle.
func (s *Storage) Put(ctx context.Context, schematicID, version, arch string, data io.Reader, size int64) error {
	tag := builder.CacheTag(schematicID, version, arch)
	taggedRef := s.cacheRepository.Tag(tag)

	s.logger.Info("pushing SPDX bundle", zap.Stringer("ref", taggedRef))

	// Read all data into memory for the layer
	content, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read SPDX bundle data: %w", err)
	}

	layer, err := partial.CompressedToLayer(&layerWrapper{
		content: content,
	})
	if err != nil {
		return fmt.Errorf("failed to create layer: %w", err)
	}

	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return fmt.Errorf("failed to append layer: %w", err)
	}

	if err = s.pusher.Push(ctx, taggedRef, img); err != nil {
		return fmt.Errorf("failed to push SPDX bundle: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("failed to get image digest: %w", err)
	}

	digestRef := s.cacheRepository.Digest(digest.String())

	s.logger.Info("signing SPDX bundle", zap.Stringer("ref", digestRef))

	if err := s.imageSigner.SignImage(ctx, digestRef, s.pusher); err != nil {
		return fmt.Errorf("error signing SPDX bundle: %w", err)
	}

	return nil
}

// layerWrapper wraps content to implement the v1.Layer interface.
type layerWrapper struct {
	content []byte
}

// Digest returns the hash of the compressed layer.
func (w *layerWrapper) Digest() (v1.Hash, error) {
	hash, _, err := v1.SHA256(bytes.NewReader(w.content))

	return hash, err
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (w *layerWrapper) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(w.content)), nil
}

// Size returns the compressed size of the layer.
func (w *layerWrapper) Size() (int64, error) {
	return int64(len(w.content)), nil
}

// MediaType returns the media type for the layer.
func (w *layerWrapper) MediaType() (types.MediaType, error) {
	return SPDXBundleMediaType, nil
}

// remoteBundle implements storage.Bundle for a remote layer.
type remoteBundle struct {
	layer v1.Layer
	size  int64
}

// Reader returns a reader for the bundle content.
func (b *remoteBundle) Reader() (io.ReadCloser, error) {
	return b.layer.Compressed()
}

// Size returns the size of the bundle in bytes.
func (b *remoteBundle) Size() int64 {
	return b.size
}
