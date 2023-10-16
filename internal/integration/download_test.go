// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/h2non/filetype"
	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func downloadAsset(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/image/"+schematicID+"/"+talosVersion+"/"+path, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func downloadAssetInvalid(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, expectedCode int) string {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	assert.Equal(t, expectedCode, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func downloadAssetAndMatchSize(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, fileType string, expectedSize int64) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)

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

func downloadAssetAndValidateInitramfs(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, initramfsSpec initramfsSpec) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)

	d := t.TempDir()
	initramfsPath := filepath.Join(d, "initramfs.xz")

	out, err := os.Create(initramfsPath)
	require.NoError(t, err)

	_, err = io.Copy(out, body)
	require.NoError(t, err)

	require.NoError(t, out.Close())

	assertInitramfs(t, initramfsPath, initramfsSpec)
}

func downloadCmdlineAndMatch(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, expectedSubstrings ...string) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)

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

			t.Run("empty schematic", func(t *testing.T) {
				t.Parallel()

				t.Run("iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image", 82724864)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image", 122007552)
				})

				t.Run("kernel", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-amd64", "application/vnd.microsoft.portable-executable", 16708992)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-arm64", "application/vnd.microsoft.portable-executable", 69356032)
				})

				t.Run("initramfs", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndValidateInitramfs(ctx, t, baseURL, emptySchematicID, talosVersion, "initramfs-amd64.xz",
						initramfsSpec{
							schematicID: emptySchematicID,
						},
					)
					downloadAssetAndValidateInitramfs(ctx, t, baseURL, emptySchematicID, talosVersion, "initramfs-arm64.xz",
						initramfsSpec{
							schematicID: emptySchematicID,
						},
					)
				})

				t.Run("installer image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-amd64.tar", "application/x-tar", 167482880)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar", 163630080)
				})

				t.Run("metal image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz", 78472708)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 66625420)
				})

				t.Run("aws image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-amd64.raw.xz", "application/x-xz", 78472708)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-arm64.raw.xz", "application/x-xz", 66625420)
				})

				t.Run("gcp image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "gcp-amd64.raw.tar.gz", "application/gzip", 78472708)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "gcp-arm64.raw.tar.gz", "application/gzip", 70625420)
				})
			})

			t.Run("extensions schematic", func(t *testing.T) {
				t.Parallel()

				t.Run("iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image", 112222208)
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image", 150120448)
				})

				t.Run("metal image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz", 108049020)
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 91484764)
				})

				t.Run("initramfs", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndValidateInitramfs(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "initramfs-amd64.xz",
						initramfsSpec{
							earlyPaths: []string{
								"kernel/x86/microcode/AuthenticAMD.bin",
							},
							extensions: []string{
								"amd-ucode",
								"gvisor",
								"gasket",
							},
							modulesDepMatch: optional.Some("gasket"),
							schematicID:     systemExtensionsSchematicID,
						},
					)
					downloadAssetAndValidateInitramfs(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "initramfs-arm64.xz",
						initramfsSpec{
							earlyPaths: []string{
								"kernel/x86/microcode/AuthenticAMD.bin",
							},
							extensions: []string{
								"amd-ucode",
								"gvisor",
								"gasket",
							},
							modulesDepMatch: optional.Some("gasket"),
							schematicID:     systemExtensionsSchematicID,
						},
					)
				})
			})
		})
	}

	t.Run("cmdline", func(t *testing.T) {
		t.Parallel()

		talosVersion := talosVersions[0]

		t.Run("default metal", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, emptySchematicID, talosVersion, "cmdline-metal-amd64", "talos.platform=metal")
		})

		t.Run("default aws", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, emptySchematicID, talosVersion, "cmdline-aws-arm64", "talos.platform=aws")
		})

		t.Run("extra metal", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, extraArgsSchematicID, talosVersion, "cmdline-metal-amd64", "talos.platform=metal", "nolapic", "nomodeset")
		})

		t.Run("meta contents", func(t *testing.T) {
			t.Parallel()

			downloadCmdlineAndMatch(ctx, t, baseURL, metaSchematicID, talosVersion, "cmdline-metal-amd64", "talos.environment=INSTALLER_META_BASE64=MHhhPXsiZXh0ZXJuYWxJUHMiOlsiMS4yLjMuNCJdfQ==")
		})
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		t.Run("schematic", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "schematic ID \"aaaaaaaaaaaa\" not found\n",
				downloadAssetInvalid(ctx, t, baseURL, "aaaaaaaaaaaa", "v1.5.0", "metal-amd64.iso", http.StatusNotFound),
			)
		})

		t.Run("profile", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "error parsing profile from path: invalid profile path: \"metal-amd64.ssd\"\n",
				downloadAssetInvalid(ctx, t, baseURL, emptySchematicID, "v1.5.0", "metal-amd64.ssd", http.StatusBadRequest),
			)
		})
	})
}
