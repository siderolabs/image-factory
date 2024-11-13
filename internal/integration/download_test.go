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
	"strings"
	"testing"
	"time"

	"github.com/h2non/filetype"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
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

func downloadAssetAssertCached(ctx context.Context, t *testing.T, baseURL, schematicID, talosVersion, path string, expectedSize int64) {
	t.Helper()

	start := time.Now()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	size, err := io.Copy(io.Discard, resp.Body)

	require.NoError(t, err)

	assert.Equal(t, expectedSize, size)
	assert.Less(t, time.Since(start), 60*time.Second) // images take some time to download, even from the cache, so give it some time
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

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, rest+int64(n))
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

	size, err := io.Copy(out, body)
	require.NoError(t, err)

	require.NoError(t, out.Close())

	assertInitramfs(t, initramfsPath, initramfsSpec)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, size)
}

func downloadCmdlineAndMatch(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, expected string) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)

	cmdlineBytes, err := io.ReadAll(body)
	require.NoError(t, err)

	cmdline := string(cmdlineBytes)

	assert.Equal(t, expected, cmdline)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, int64(len(cmdlineBytes)))
}

func schematicExtraInfo(t *testing.T, schematicID string, talosVersion string) string {
	t.Helper()

	if !quirks.New(talosVersion).SupportsOverlay() {
		return ""
	}

	schematic := must.Value(testSchematics[schematicID].Marshal())(t)

	return string(schematic)
}

func sizePicker(talosVersion string, v ...any) int64 {
	if len(v)%2 != 0 {
		panic("sizePicker: odd number of arguments")
	}

	talosVersion = strings.TrimPrefix(talosVersion, "v")

	for i := 0; i < len(v); i += 2 {
		k := v[i].(string)

		if strings.HasPrefix(talosVersion, k) {
			return int64(v[i+1].(int))
		}
	}

	panic("sizePicker: no match")
}

func testDownloadFrontend(ctx context.Context, t *testing.T, baseURL string) {
	const MiB = 1024 * 1024

	talosVersions := []string{
		"v1.8.2",
		"v1.5.1",
	}

	for _, talosVersion := range talosVersions {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			t.Run("empty schematic", func(t *testing.T) {
				t.Parallel()

				t.Run("iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 82724864, "1.8", 106475520))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 122007552, "1.8", 90738688))
				})

				t.Run("secureboot iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 162*MiB, "1.8", 198*MiB))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-secureboot.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 232*MiB, "1.8", 169*MiB))
				})

				t.Run("kernel", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-amd64", "application/vnd.microsoft.portable-executable", sizePicker(talosVersion, "1.5", 16708992, "1.8", 18727936))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-arm64", "application/vnd.microsoft.portable-executable", sizePicker(talosVersion, "1.5", 69356032, "1.8", 21787136))
				})

				t.Run("initramfs", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndValidateInitramfs(ctx, t, baseURL, emptySchematicID, talosVersion, "initramfs-amd64.xz",
						initramfsSpec{
							schematicID:        emptySchematicID,
							schematicExtraInfo: schematicExtraInfo(t, emptySchematicID, talosVersion),
						},
					)
					downloadAssetAndValidateInitramfs(ctx, t, baseURL, emptySchematicID, talosVersion, "initramfs-arm64.xz",
						initramfsSpec{
							schematicID:        emptySchematicID,
							schematicExtraInfo: schematicExtraInfo(t, emptySchematicID, talosVersion),
						},
					)
				})

				t.Run("UKI", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot-uki.efi", "application/vnd.microsoft.portable-executable", sizePicker(talosVersion, "1.5", 77691056, "1.8", 98469552))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-secureboot-uki.efi", "application/vnd.microsoft.portable-executable", sizePicker(talosVersion, "1.5", 114564272, "1.8", 82733744))
				})

				t.Run("installer image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-amd64.tar", "application/x-tar", sizePicker(talosVersion, "1.5", 167482880, "1.8", 185155584))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar", sizePicker(talosVersion, "1.5", 222793728, "1.8", 170119168))
				})

				t.Run("metal image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 78472708, "1.8", 101464300))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 66625420, "1.8", 83998408))
				})

				t.Run("metal zstd image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.zst", "application/zstd", sizePicker(talosVersion, "1.5", 78472708, "1.8", 100120864))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.raw.zst", "application/zstd", sizePicker(talosVersion, "1.5", 66_625_420, "1.8", 83_651_316))
				})

				t.Run("metal qcow2 image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.qcow2", "", sizePicker(talosVersion, "1.5", 92176384, "1.8", 119808000))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.qcow2", "", sizePicker(talosVersion, "1.5", 119808000, "1.8", 90415104))
				})

				t.Run("metal secureboot image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 78472708, "1.8", 97975380))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-secureboot.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 66625420, "1.8", 82420728))
				})

				t.Run("aws image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-amd64.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 78472708, "1.8", 103249176))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-arm64.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 66625420, "1.8", 85783432))
				})

				t.Run("gcp image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "gcp-amd64.raw.tar.gz", "application/gzip", sizePicker(talosVersion, "1.5", 78472708, "1.8", 102107964))
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "gcp-arm64.raw.tar.gz", "application/gzip", sizePicker(talosVersion, "1.5", 70625420, "1.8", 84214192))
				})

				t.Run("rpi image", func(t *testing.T) {
					t.Parallel()

					if quirks.New(talosVersion).SupportsOverlay() {
						downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 136632380)
					} else {
						downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-rpi_generic-arm64.raw.xz", "application/x-xz", 107183936)
					}
				})
			})

			t.Run("extensions schematic", func(t *testing.T) {
				t.Parallel()

				t.Run("iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 112222208, "1.8", 133283840))
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 150120448, "1.8", 115824640))
				})

				t.Run("secureboot iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64-secureboot.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 214*MiB, "1.8", 250*MiB))
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64-secureboot.iso", "application/x-iso9660-image", sizePicker(talosVersion, "1.5", 280*MiB, "1.8", 218*MiB))
				})

				t.Run("metal image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 108049020, "1.8", 128244948))
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", sizePicker(talosVersion, "1.5", 91484764, "1.8", 109057716))
				})

				t.Run("rpi image", func(t *testing.T) {
					t.Parallel()

					if quirks.New(talosVersion).SupportsOverlay() {
						downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 136632380)
					} else {
						downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-rpi_generic-arm64.raw.xz", "application/x-xz", 132095368)
					}
				})

				t.Run("initramfs", func(t *testing.T) {
					t.Parallel()

					gasketName := "gasket"

					if quirks.New(talosVersion).SupportsOverlay() {
						gasketName = "gasket-driver"
					}

					downloadAssetAndValidateInitramfs(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "initramfs-amd64.xz",
						initramfsSpec{
							earlyPaths: []string{
								"kernel/x86/microcode/AuthenticAMD.bin",
							},
							extensions: []string{
								"amd-ucode",
								"gvisor",
								gasketName,
							},
							modulesDepMatch:    optional.Some("gasket"),
							schematicID:        systemExtensionsSchematicID,
							schematicExtraInfo: schematicExtraInfo(t, systemExtensionsSchematicID, talosVersion),
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
								gasketName,
							},
							modulesDepMatch:    optional.Some("gasket"),
							schematicID:        systemExtensionsSchematicID,
							schematicExtraInfo: schematicExtraInfo(t, systemExtensionsSchematicID, talosVersion),
						},
					)
				})
			})
		})
	}

	// test for v1.7.0 which supports overlays
	t.Run("v1.7.0", func(t *testing.T) {
		t.Parallel()

		talosVersion := "v1.7.0"

		t.Run("installer image", func(t *testing.T) {
			t.Parallel()

			// curl the image and `du -sh` on the tarball
			downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar", 209*1024*1024)
		})

		t.Run("metal image", func(t *testing.T) {
			t.Parallel()

			// curl the image and `du -sh` on the image
			downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 117*1024*1024)
		})

		t.Run("initramfs", func(t *testing.T) {
			t.Parallel()

			downloadAssetAndValidateInitramfs(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "initramfs-amd64.xz",
				initramfsSpec{
					schematicID:        rpiGenericOverlaySchematicID,
					skipMlxfw:          true,
					schematicExtraInfo: string(must.Value(testSchematics[rpiGenericOverlaySchematicID].Marshal())(t)),
				},
			)
		})
	})

	// special test for v1.3.7 which supports less features
	t.Run("v1.3.7", func(t *testing.T) {
		t.Parallel()

		talosVersion := "v1.3.7"

		t.Run("empty schematic", func(t *testing.T) {
			t.Parallel()

			t.Run("iso", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image", 82724864)
				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image", 122007552)
			})

			t.Run("kernel", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-amd64", "application/vnd.microsoft.portable-executable", 18956032)
				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-arm64", "application/vnd.microsoft.portable-executable", 69356032)
			})

			t.Run("initramfs", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndValidateInitramfs(ctx, t, baseURL, emptySchematicID, talosVersion, "initramfs-amd64.xz",
					initramfsSpec{
						schematicID:        emptySchematicID,
						schematicExtraInfo: schematicExtraInfo(t, emptySchematicID, talosVersion),
					},
				)
				downloadAssetAndValidateInitramfs(ctx, t, baseURL, emptySchematicID, talosVersion, "initramfs-arm64.xz",
					initramfsSpec{
						schematicID:        emptySchematicID,
						schematicExtraInfo: schematicExtraInfo(t, emptySchematicID, talosVersion),
					},
				)
			})

			t.Run("installer image", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-amd64.tar", "application/x-tar", 188085248)
				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar", 275915264)
			})

			t.Run("metal image", func(t *testing.T) {
				t.Parallel()

				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz", 78472708)
				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 66625420)
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
						modulesDepMatch:    optional.Some("gasket"),
						schematicID:        systemExtensionsSchematicID,
						skipMlxfw:          true,
						schematicExtraInfo: schematicExtraInfo(t, systemExtensionsSchematicID, talosVersion),
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
						modulesDepMatch:    optional.Some("gasket"),
						schematicID:        systemExtensionsSchematicID,
						skipMlxfw:          true,
						schematicExtraInfo: schematicExtraInfo(t, systemExtensionsSchematicID, talosVersion),
					},
				)
			})
		})
	})

	t.Run("cmdline", func(t *testing.T) {
		t.Parallel()

		for _, talosVersion := range talosVersions {
			t.Run(talosVersion, func(t *testing.T) {
				t.Parallel()

				t.Run("default metal", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

					if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
						expected = strings.ReplaceAll(expected, " console=ttyS0", "")
					}

					downloadCmdlineAndMatch(ctx, t, baseURL, emptySchematicID, talosVersion, "cmdline-metal-amd64", expected)
				})

				t.Run("default aws", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=aws console=tty1 console=ttyS0 net.ifnames=0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

					downloadCmdlineAndMatch(ctx, t, baseURL, emptySchematicID, talosVersion, "cmdline-aws-arm64", expected)
				})

				t.Run("extra metal", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 nolapic nomodeset"

					if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
						expected = strings.ReplaceAll(expected, " console=ttyS0", "")
					}

					downloadCmdlineAndMatch(ctx, t, baseURL, extraArgsSchematicID, talosVersion, "cmdline-metal-amd64", expected)
				})

				t.Run("meta contents", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 talos.environment=INSTALLER_META_BASE64=MHhhPXsiZXh0ZXJuYWxJUHMiOlsiMS4yLjMuNCJdfQ=="

					if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
						expected = strings.ReplaceAll(expected, " console=ttyS0", "")
					}

					downloadCmdlineAndMatch(ctx, t, baseURL, metaSchematicID, talosVersion, "cmdline-metal-amd64", expected)
				})
			})
		}
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
