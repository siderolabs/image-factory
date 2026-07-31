// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
)

func TestDescriptorFromOCIPathSelectsExactPlatformManifest(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path, err := layout.Write(directory, empty.Index)
	require.NoError(t, err)
	require.NoError(t, path.AppendImage(empty.Image, layout.WithPlatform(v1.Platform{OS: "linux", Architecture: "amd64"})))

	descriptor, err := artifacts.DescriptorFromOCIPath(directory, artifacts.ArchAmd64)
	require.NoError(t, err)

	digest, err := empty.Image.Digest()
	require.NoError(t, err)
	require.Equal(t, digest, descriptor.Digest)
	require.Equal(t, "amd64", descriptor.Platform.Architecture)
	require.NotZero(t, descriptor.Size)
	require.NotEmpty(t, descriptor.MediaType)

	_, err = artifacts.DescriptorFromOCIPath(directory, artifacts.ArchArm64)
	require.ErrorContains(t, err, "arm64")
}
