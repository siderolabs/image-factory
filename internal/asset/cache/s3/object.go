// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3

import (
	"io"

	"github.com/siderolabs/image-factory/internal/asset/cache"
)

type objectAsset struct {
	obj          io.ReadCloser
	size         int64
	presignedUrl string
}

// Check interface.
var (
	_ cache.BootAsset         = (*objectAsset)(nil)
	_ cache.RedirectableAsset = (*objectAsset)(nil)
)

// Redirect returns the presigned URL for the boot asset.
// If the URL is empty, it returns error.
func (r *objectAsset) Redirect() (string, error) {
	if r.presignedUrl == "" {
		return "", cache.ErrNoRedirect
	}
	return r.presignedUrl, nil
}

// Size returns the size of the boot asset.
func (r *objectAsset) Size() int64 {
	return r.size
}

// Reader returns a reader for the boot asset.
func (r *objectAsset) Reader() (io.ReadCloser, error) {
	return r.obj, nil
}
