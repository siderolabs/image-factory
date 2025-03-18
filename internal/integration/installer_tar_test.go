// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/require"
)

func assertInstallerTarUKIArtifact(t *testing.T, path, arch string, ukiSpec ukiSpec) {
	t.Helper()

	img, err := crane.Load(path)
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)

	require.Len(t, layers, 2)

	artifactsLayer := layers[1]

	reader, err := artifactsLayer.Uncompressed()
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, reader.Close())
	})

	tarReader := tar.NewReader(reader)

	ukiExtractDir := t.TempDir()
	ukiFile := filepath.Join(ukiExtractDir, fmt.Sprintf("vmlinuz-%s.efi", arch))

	for {
		header, err := tarReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			require.NoError(t, err)
		}

		switch header.Name {
		case fmt.Sprintf("usr/install/%s/vmlinuz.efi", arch):
			file, err := os.Create(ukiFile)
			require.NoError(t, err)

			_, err = io.Copy(file, tarReader)
			require.NoError(t, err)

			assertUKI(t, ukiFile, ukiSpec)
		// nothing to do here
		case fmt.Sprintf("usr/install/%s/systemd-boot.efi", arch):
		default:
			t.Fatalf("unexpected file in installer tar: %s", header.Name)
		}
	}
}
