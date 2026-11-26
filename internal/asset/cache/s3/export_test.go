// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3

import (
	"context"

	"github.com/minio/minio-go/v7"

	"github.com/siderolabs/image-factory/internal/asset/cache"
)

// NewObjectAssetForTesting returns an S3 object asset with the given getter and metadata.
func NewObjectAssetForTesting(getter func(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error), bucket, key, etag string, size int64) cache.SeekableAsset {
	return &objectAsset{getter: getter, bucket: bucket, key: key, etag: etag, size: size}
}
