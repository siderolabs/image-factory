// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts_test

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
)

func buildExtensionsManifestArchive(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	for name, contents := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(contents)),
		}))

		_, err := tw.Write([]byte(contents))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())

	return &buf
}

func TestExtractExtensionList(t *testing.T) {
	t.Parallel()

	registry, err := name.NewRegistry("harbor.example.com")
	require.NoError(t, err)

	const (
		gvisorLine = "ghcr.io/siderolabs/gvisor:20231214.0@sha256:5ab365f2b98ab885b1d9a6ebb2e2b06d0a7887d2c173a2b7d3f9e0e4f2f4f1cb"
		zfsLine    = "ghcr.io/siderolabs/zfs:2.2.2@sha256:6f9e2d0c8a9d18b9a54c3d0f7c6b3a2f4e5d6c7b8a9f0e1d2c3b4a5f6e7d8c9b"
	)

	archive := map[string]string{
		"image-digests": gvisorLine + "\n" + zfsLine + "\n",
		"descriptions.yaml": `"` + gvisorLine + `":
  author: Sidero Labs
  description: gVisor container runtime
`,
	}

	for _, test := range []struct {
		name      string
		namespace string

		expectedGvisorPullRef string
	}{
		{
			name: "no namespace",

			expectedGvisorPullRef: "harbor.example.com/siderolabs/gvisor:20231214.0",
		},
		{
			name:      "with namespace",
			namespace: "ghcrio",

			expectedGvisorPullRef: "harbor.example.com/ghcrio/siderolabs/gvisor:20231214.0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			extensions, err := artifacts.ExtractExtensionList(
				buildExtensionsManifestArchive(t, archive),
				registry,
				test.namespace,
			)
			require.NoError(t, err)
			require.Len(t, extensions, 2)

			gvisor := extensions[0]

			// TaggedReference is the extension identity (matched against schematics), so
			// the namespace must never leak into it.
			assert.Equal(t, "siderolabs/gvisor", gvisor.TaggedReference.RepositoryStr())
			assert.Equal(t, "harbor.example.com", gvisor.TaggedReference.RegistryStr())
			assert.Equal(t, test.expectedGvisorPullRef, gvisor.PullReference().String())
			assert.Equal(t, "sha256:5ab365f2b98ab885b1d9a6ebb2e2b06d0a7887d2c173a2b7d3f9e0e4f2f4f1cb", gvisor.Digest)
			assert.Equal(t, "Sidero Labs", gvisor.Author)
			assert.Equal(t, "gVisor container runtime", gvisor.Description)

			zfs := extensions[1]

			assert.Equal(t, "siderolabs/zfs", zfs.TaggedReference.RepositoryStr())
			assert.Empty(t, zfs.Description)
		})
	}
}

func TestRepoWithNamespace(t *testing.T) {
	t.Parallel()

	registry, err := name.NewRegistry("harbor.example.com")
	require.NoError(t, err)

	assert.Equal(t, "harbor.example.com/siderolabs/imager",
		artifacts.RepoWithNamespace(registry, "", "siderolabs/imager").String())
	assert.Equal(t, "harbor.example.com/ghcrio/siderolabs/imager",
		artifacts.RepoWithNamespace(registry, "ghcrio", "siderolabs/imager").String())
	assert.Equal(t, "harbor.example.com/ghcrio/siderolabs/imager",
		artifacts.RepoWithNamespace(registry, "ghcrio/", "siderolabs/imager").String())
}
