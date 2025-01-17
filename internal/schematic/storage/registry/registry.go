// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry implements a schematic storage in OCI registry.
package registry

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/xerrors"

	"github.com/skyssolutions/siderolabs-image-factory/internal/regtransport"
	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic/storage"
)

// SchematicMediaType is a media type for the schematic stored in the OCI registry.
const SchematicMediaType types.MediaType = "application/vnd.sidero.dev-image.schematic"

// Storage is a schematic storage in a OCI Registry.
//
// Schematic ID is a sha256 of the contents, so it matches registry content-addressable storage.
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

// Head checks if the schematic exists.
func (s *Storage) Head(ctx context.Context, id string) error {
	// pre-validate the ID, so that invalid IDs return not found
	_, err := v1.NewHash(digestPrefix + id)
	if err != nil {
		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
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

	if regtransport.IsStatusCodeError(err, http.StatusNotFound) {
		return xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	return err
}

// Get returns the schematic.
func (s *Storage) Get(ctx context.Context, id string) ([]byte, error) {
	// pre-validate the ID
	_, err := v1.NewHash(digestPrefix + id)
	if err != nil {
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
	}

	layer, err := s.puller.Layer(ctx, s.repository.Digest(digestPrefix+id))
	if err != nil {
		if regtransport.IsStatusCodeError(err, http.StatusNotFound) {
			return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
		}

		return nil, err
	}

	// pull the layer
	r, err := layer.Compressed()
	if err != nil {
		if regtransport.IsStatusCodeError(err, http.StatusNotFound) {
			return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("schematic ID %q not found", id)
		}

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
	return SchematicMediaType, nil
}

// Put stores the schematic.
func (s *Storage) Put(ctx context.Context, id string, data []byte) error {
	layer, err := partial.CompressedToLayer(&layerWrapper{
		data: data,
		id:   id,
	})
	if err != nil {
		return err
	}

	// we don't need to push an image manifest, but we create it to make sure that a schematic blob (layer)
	// doesn't get GC'ed by the registry
	img := empty.Image

	img, err = mutate.AppendLayers(img, layer)
	if err != nil {
		return err
	}

	return s.pusher.Push(ctx, s.repository.Tag(id), img)
}

// Describe implements prom.Collector interface.
func (s *Storage) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(s, ch)
}

// Collect implements prom.Collector interface.
func (s *Storage) Collect(chan<- prometheus.Metric) {
	// no metrics for now
}

var _ prometheus.Collector = &Storage{}
