// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts_test

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/artifacts"
)

func TestTalosVersionsCallerIsolation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)

	host := strings.TrimPrefix(server.URL, "http://")

	for _, tag := range []string{"v1.13.0", "v1.14.0"} {
		ref, err := name.NewTag(host+"/siderolabs/imager:"+tag, name.Insecure)
		require.NoError(t, err)
		require.NoError(t, remote.Write(ref, empty.Image))
	}

	archive := buildExtensionsManifestArchive(t, map[string]string{
		"image-digests": "ghcr.io/siderolabs/gvisor:20231214.0@sha256:5ab365f2b98ab885b1d9a6ebb2e2b06d0a7887d2c173a2b7d3f9e0e4f2f4f1cb\n",
	}).Bytes()

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(archive)), nil
	})
	require.NoError(t, err)

	manifest, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	ref, err := name.NewTag(host+"/siderolabs/extensions:v1.13.0", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, manifest))

	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "cached"}[cached], func(t *testing.T) {
			t.Parallel()

			manager, err := artifacts.NewManager(zap.NewNop(), artifacts.Options{
				ImageVerifyOptions:          artifacts.ImageVerifyOptions{Disabled: true},
				ImageRegistry:               host,
				InsecureImageRegistry:       true,
				ImagerImage:                 "siderolabs/imager",
				ExtensionManifestImage:      "siderolabs/extensions",
				TalosVersionRecheckInterval: time.Hour,
				RegistryRefreshInterval:     time.Hour,
			})
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, manager.Close()) })

			if cached {
				_, err = manager.GetTalosVersions(t.Context())
				require.NoError(t, err)
			}

			versions, err := manager.GetTalosVersions(t.Context())
			require.NoError(t, err)
			require.Len(t, versions, 2)

			// Pause a caller's reverse between its two writes. Other callers must
			// still see v1.13.0 while this caller temporarily has two v1.14.0s.
			versions[0] = versions[1]

			extensions, err := manager.GetOfficialExtensions(t.Context(), "1.13.0")
			require.NoError(t, err)
			assert.Len(t, extensions, 1)

			available, err := manager.GetTalosVersions(t.Context())
			require.NoError(t, err)
			assert.Equal(t, []semver.Version{semver.MustParse("1.13.0"), semver.MustParse("1.14.0")}, available)
		})
	}
}
