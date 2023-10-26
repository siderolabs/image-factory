// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
)

// remoteAsset holds a cached image layer which contains the asset.
type remoteAsset struct {
	layer v1.Layer
	size  int64
}

// Check interface.
var _ BootAsset = (*remoteAsset)(nil)

// Size returns the size of the boot asset.
func (r *remoteAsset) Size() int64 {
	return r.size
}

// Reader returns a reader for the boot asset.
func (r *remoteAsset) Reader() (io.ReadCloser, error) {
	return r.layer.Compressed()
}

// layerWrapper adapts to the expected v1.Layer interface.
type layerWrapper struct {
	src    BootAsset
	digest v1.Hash
}

// Digest returns the Hash of the compressed layer.
func (w *layerWrapper) Digest() (v1.Hash, error) {
	if w.digest != (v1.Hash{}) {
		return w.digest, nil
	}

	in, err := w.src.Reader()
	if err != nil {
		return v1.Hash{}, err
	}

	defer in.Close() //nolint:errcheck

	digester := digest.Canonical.Digester()

	if _, err := io.Copy(digester.Hash(), in); err != nil {
		return v1.Hash{}, err
	}

	w.digest = v1.Hash{
		Algorithm: digest.Canonical.String(),
		Hex:       digester.Digest().Hex(),
	}

	return w.digest, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
//
// In fact, it returns raw contents.
func (w *layerWrapper) Compressed() (io.ReadCloser, error) {
	return w.src.Reader()
}

// Size returns the compressed size of the Layer.
func (w *layerWrapper) Size() (int64, error) {
	return w.src.Size(), nil
}

// Returns the mediaType for the compressed Layer.
func (w *layerWrapper) MediaType() (types.MediaType, error) {
	return "application/data", nil
}
