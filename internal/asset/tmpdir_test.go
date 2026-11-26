// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/asset"
)

func TestTmpDirSeekableReaderReadsFromOffset(t *testing.T) {
	t.Parallel()

	built, err := asset.NewTmpDirForTesting([]byte("asset-bytes"))
	require.NoError(t, err)

	seekable, ok := built.(asset.SeekableBootAsset)
	require.True(t, ok, "an asset built on demand is kept in a file, so it can be read from any offset")

	reader, err := seekable.SeekableReader()
	require.NoError(t, err)

	defer reader.Close() //nolint:errcheck

	_, err = reader.Seek(2, io.SeekStart)
	require.NoError(t, err)

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "set-bytes", string(got))
}
