// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package s3_test

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/asset/cache/s3"
)

func TestObjectAssetSeekableReaderReturnsStoredObject(t *testing.T) {
	t.Parallel()

	var stored *minio.Object

	var gotBucket, gotKey string

	object := s3.NewObjectAssetForTesting(
		func(_ context.Context, bucket, key string, _ minio.GetObjectOptions) (*minio.Object, error) {
			stored = &minio.Object{}
			gotBucket, gotKey = bucket, key

			return stored, nil
		},
		"bucket", "key", "cached-etag", 11,
	)

	reader, err := object.SeekableReader()
	require.NoError(t, err)
	require.Same(t, stored, reader, "the seekable reader must be the object of the stored asset")
	require.Equal(t, "bucket", gotBucket)
	require.Equal(t, "key", gotKey)
	require.Equal(t, int64(11), object.Size())
}
