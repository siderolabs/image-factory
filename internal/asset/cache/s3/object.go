// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"

	"github.com/siderolabs/image-factory/internal/asset/cache"
)

type objectAsset struct {
	getter          func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	getPresignedURL func(context.Context, string) (string, error)
	bucket          string
	key             string
	etag            string
	size            int64
}

// Check interface.
var (
	_ cache.BootAsset         = (*objectAsset)(nil)
	_ cache.RedirectableAsset = (*objectAsset)(nil)
)

// Redirect returns the presigned URL for the boot asset.
// If the URL is empty, it returns error.
func (r *objectAsset) Redirect(ctx context.Context, filename string) (string, error) {
	return r.getPresignedURL(ctx, filename)
}

// Size returns the size of the boot asset.
func (r *objectAsset) Size() int64 {
	return r.size
}

// Reader returns a reader for the boot asset.
func (r *objectAsset) Reader() (io.ReadCloser, error) {
	opts := minio.GetObjectOptions{}

	err := opts.SetMatchETag(r.etag)
	if err != nil {
		return nil, err
	}

	return r.getter(context.Background(), r.bucket, r.key, opts)
}
