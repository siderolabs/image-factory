// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/siderolabs/image-factory/internal/asset/cache"
)

type objectReference struct {
	ETag string `json:"etag"`
}

func newObjectReference(asset cache.BootAsset) (*objectReference, error) {
	r, err := asset.Reader()
	if err != nil {
		return nil, err
	}

	obj := &objectReference{}

	err = json.NewDecoder(r).Decode(obj)

	return obj, err
}

type referenceAsset struct {
	content []byte
}

func newReferenceAsset(v *objectReference) (*referenceAsset, error) {
	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(v)

	return &referenceAsset{
		content: buf.Bytes(),
	}, err
}

// Check interface.
var _ cache.BootAsset = (*referenceAsset)(nil)

// Size returns the size of the boot asset.
func (r *referenceAsset) Size() int64 {
	return int64(len(r.content))
}

// Reader returns a reader for the boot asset.
func (r *referenceAsset) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(r.content)), nil
}
