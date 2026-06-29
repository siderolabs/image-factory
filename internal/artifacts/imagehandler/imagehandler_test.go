// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagehandler_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/internal/artifacts/imagehandler"
)

// TestOCIRecordsPlatform asserts that a single-arch image is written to the OCI layout with a
// platform descriptor, so the Talos imager can match it to a target architecture.
func TestOCIRecordsPlatform(t *testing.T) {
	t.Parallel()

	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	// Single-arch images (e.g. custom-built extensions) carry their arch only in the config.
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{OS: "linux", Architecture: "arm64"})
	require.NoError(t, err)

	path := t.TempDir() + "/oci"

	require.NoError(t, imagehandler.OCI(path)(t.Context(), zaptest.NewLogger(t), img))

	idx, err := layout.ImageIndexFromPath(path)
	require.NoError(t, err)

	manifest, err := idx.IndexManifest()
	require.NoError(t, err)

	require.Len(t, manifest.Manifests, 1)
	require.NotNil(t, manifest.Manifests[0].Platform, "platform descriptor must be recorded")
	require.Equal(t, "linux", manifest.Manifests[0].Platform.OS)
	require.Equal(t, "arm64", manifest.Manifests[0].Platform.Architecture)
}
