// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry implements a configuration storage in OCI registry.
package registry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-service/internal/configuration/storage"
)

// ConfigurationMediaType is a media type for the configuration stored in the OCI registry.
const ConfigurationMediaType types.MediaType = "application/vnd.sidero.dev-image.configuration"

// Storage is a config storage in a OCI Registry.
//
// Configuration ID is a sha256 of the contents, so it matches registry content-addressable storage.
type Storage struct {
	pusher     *remote.Pusher
	puller     *remote.Puller
	repository name.Repository
}

// Check interface.
var _ storage.Storage = (*Storage)(nil)

// digestPrefix is "sha256:".
var digestPrefix = digest.Canonical.String() + ":"

// NewStorage creates a new storage.
func NewStorage(repository name.Repository, remoteOpts []remote.Option) (*Storage, error) {
	s := &Storage{
		repository: repository,
	}

	var err error

	s.pusher, err = remote.NewPusher(append(remoteOpts, remote.WithNondistributable)...)
	if err != nil {
		return nil, err
	}

	s.puller, err = remote.NewPuller(remoteOpts...)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Head checks if the configuration exists.
func (s *Storage) Head(ctx context.Context, id string) error {
	// pre-validate the ID, so that invalid IDs return not found
	_, err := v1.NewHash(digestPrefix + id)
	if err != nil {
		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("configuration ID %q not found", id)
	}

	layer, err := s.puller.Layer(ctx, s.repository.Digest(digestPrefix+id))
	if err != nil {
		return err
	}

	// check if the layer exists
	// we're not interested in size, but it calls HEAD on the layer
	_, err = layer.Size()
	if err == nil {
		return nil
	}

	var transportError *transport.Error

	if errors.As(err, &transportError) && transportError.StatusCode == http.StatusNotFound {
		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("configuration ID %q not found", id)
	}

	return err
}

// Get returns the configuration.
func (s *Storage) Get(ctx context.Context, id string) ([]byte, error) {
	// pre-validate the ID
	_, err := v1.NewHash(digestPrefix + id)
	if err != nil {
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("configuration ID %q not found", id)
	}

	layer, err := s.puller.Layer(ctx, s.repository.Digest(digestPrefix+id))
	if err != nil {
		var transportError *transport.Error

		if errors.As(err, &transportError) && transportError.StatusCode == http.StatusNotFound {
			return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("configuration ID %q not found", id)
		}

		return nil, err
	}

	// pull the layer
	r, err := layer.Compressed()
	if err != nil {
		return nil, err
	}

	defer r.Close() //nolint:errcheck

	return io.ReadAll(r)
}

// layerWrapper adapts to the expected v1.Layer interface.
type layerWrapper struct {
	id   string
	data []byte
}

// Digest returns the Hash of the compressed layer.
func (w *layerWrapper) Digest() (v1.Hash, error) {
	return v1.Hash{
		Algorithm: digest.Canonical.String(),
		Hex:       w.id,
	}, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
//
// In fact, it returns raw contents.
func (w *layerWrapper) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(w.data)), nil
}

// Size returns the compressed size of the Layer.
func (w *layerWrapper) Size() (int64, error) {
	return int64(len(w.data)), nil
}

// Returns the mediaType for the compressed Layer.
func (w *layerWrapper) MediaType() (types.MediaType, error) {
	return ConfigurationMediaType, nil
}

// Put stores the configuration.
func (s *Storage) Put(ctx context.Context, id string, data []byte) error {
	layer, err := partial.CompressedToLayer(&layerWrapper{
		data: data,
		id:   id,
	})
	if err != nil {
		return err
	}

	return s.pusher.Upload(ctx, s.repository, layer)
}
