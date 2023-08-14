// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/h2non/filetype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func downloadAsset(ctx context.Context, t *testing.T, baseURL string, configurationID, talosVersion, path string) io.ReadCloser {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/image/"+configurationID+"/"+talosVersion+"/"+path, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)

	return resp.Body
}

func downloadAssetAndMatch(ctx context.Context, t *testing.T, baseURL string, configurationID, talosVersion, path string, fileType string, expectedSize int64) {
	t.Helper()

	body := downloadAsset(ctx, t, baseURL, configurationID, talosVersion, path)

	magic := make([]byte, 65536)

	n, err := io.ReadFull(body, magic)
	if err != io.EOF && err != io.ErrUnexpectedEOF {
		require.NoError(t, err)
	}

	magic = magic[:n]

	match, err := filetype.Match(magic)
	require.NoError(t, err)

	assert.Equal(t, fileType, match.MIME.Value)

	rest, err := io.Copy(io.Discard, body)
	require.NoError(t, err)

	assert.InDelta(t, expectedSize, rest+int64(n), float64(expectedSize)*0.1)
}

func downloadCmdlineAndMatch(ctx context.Context, t *testing.T, baseURL string, configurationID, talosVersion, path string, expectedSubstrings ...string) {
	t.Helper()

	body := downloadAsset(ctx, t, baseURL, configurationID, talosVersion, path)

	cmdlineBytes, err := io.ReadAll(body)
	require.NoError(t, err)

	cmdline := string(cmdlineBytes)

	for _, expectedSubstring := range expectedSubstrings {
		assert.Contains(t, cmdline, expectedSubstring)
	}
}

func testDownloadFrontend(ctx context.Context, t *testing.T, baseURL string) {
	const MiB = 1024 * 1024

	talosVersions := []string{
		"v1.5.0",
		"v1.5.1",
	}

	for _, talosVersion := range talosVersions {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			t.Run("iso", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image", 82724864)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image", 122007552)
			})

			t.Run("kernel", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "kernel-amd64", "application/vnd.microsoft.portable-executable", 16708992)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "kernel-arm64", "application/vnd.microsoft.portable-executable", 69356032)
			})

			t.Run("initramfs", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "initramfs-amd64.xz", "application/x-xz", 57.5*MiB)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "initramfs-arm64.xz", "application/x-xz", 42.5*MiB)
			})

			t.Run("installer image", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "installer-amd64.tar", "application/x-tar", 167482880)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "installer-arm64.tar", "application/x-tar", 163630080)
			})

			t.Run("metal image", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "metal-amd64.raw.xz", "application/x-xz", 78472708)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 66625420)
			})

			t.Run("aws image", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "aws-amd64.raw.xz", "application/x-xz", 78472708)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "aws-arm64.raw.xz", "application/x-xz", 66625420)
			})

			t.Run("gcp image", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "gcp-amd64.raw.tar.gz", "application/gzip", 78472708)
				downloadAssetAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "gcp-arm64.raw.tar.gz", "application/gzip", 70625420)
			})
		})
	}

	t.Run("cmdline", func(t *testing.T) {
		t.Parallel()

		talosVersion := talosVersions[0]

		t.Run("default metal", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "cmdline-metal-amd64", "talos.platform=metal")
		})

		t.Run("default aws", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, emptyConfigurationID, talosVersion, "cmdline-aws-arm64", "talos.platform=aws")
		})

		t.Run("extra metal", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, extraArgsConfigurationID, talosVersion, "cmdline-metal-amd64", "talos.platform=metal", "nolapic", "nomodeset")
		})
	})
}
