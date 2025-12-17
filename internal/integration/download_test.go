// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/h2non/filetype"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
	"go.uber.org/zap/zaptest"
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

func downloadAssetWithFilename(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path, filename string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/image/"+schematicID+"/"+talosVersion+"/"+path, nil)
	require.NoError(t, err)

	query := url.Values{}

	query.Add("filename", filename)

	req.URL.RawQuery = query.Encode()

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func downloadNoRedirect(ctx context.Context, t *testing.T, url string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err, url)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// If this is called, it means a redirect was attempted
			assert.Failf(t,
				"unexpected redirect",
				"attempted to redirect to %s (via: %v)",
				req.URL.String(), via)

			// Prevent following the redirect
			return http.ErrUseLastResponse
		},
	}

	resp, err := noRedirectClient.Do(req)
	require.NoError(t, err, url)

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

func matchSizeAndType(t *testing.T, body io.Reader, fileType string, expectedSize int64) int64 {
	t.Helper()

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

	return rest + int64(n)
}

func downloadAssetAndMatchSize(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, fileType string, expectedSize int64) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header, "Content-Disposition")

	size := matchSizeAndType(t, body, fileType, expectedSize)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, size)
}

func checkCors(ctx context.Context, t *testing.T, baseURL, schematicID, talosVersion, path string) {
	doRequest := func(method string) *http.Response {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+"/image/"+schematicID+"/"+talosVersion+"/"+path, nil)
		require.NoError(t, err)

		if req.Method == http.MethodOptions {
			req.Header.Set("Access-Control-Request-Method", http.MethodGet)
			req.Header.Set("Access-Control-Request-Headers", "authentication")
		}

		req.Header.Set("Origin", "https://foo.com")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		t.Cleanup(func() {
			resp.Body.Close()
		})

		return resp
	}

	for _, method := range []string{
		http.MethodOptions, http.MethodGet, http.MethodHead,
	} {
		t.Run(method, func(t *testing.T) {
			resp := doRequest(method)

			allowedHost := resp.Header.Get("Access-Control-Allow-Origin")

			assert.Contains(t, []string{"*", "https://foo.com"}, allowedHost)

			if method == http.MethodOptions {
				assert.Equal(t, "GET", resp.Header.Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "authentication", resp.Header.Get("Access-Control-Allow-Headers"))
			} else {
				headers := xslices.Map(strings.Split(resp.Header.Get("Access-Control-Expose-Headers"), ","), func(s string) string {
					return strings.TrimSpace(s)
				})

				for _, header := range []string{"Content-Disposition", "Content-Length", "Content-Type"} {
					assert.Contains(t, headers, header)
				}
			}
		})
	}
}

func downloadDiskImageMatchSizeAndPartitions(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, fileType string, expectedSize int64, expectedPartitions []partitionAssertion) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)

	d := t.TempDir()
	compressedPath := filepath.Join(d, "disk-image.compressed")

	out, err := os.Create(compressedPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	body := io.TeeReader(resp.Body, out)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header, "Content-Disposition")

	size := matchSizeAndType(t, body, fileType, expectedSize)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, size)

	// now decompress
	_, err = out.Seek(0, io.SeekStart)
	require.NoError(t, err)

	decompressedPath := filepath.Join(d, "disk-image.decompressed")
	decompressedFile, err := os.Create(decompressedPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, decompressedFile.Close())
	})

	var in io.Reader

	switch fileType {
	case "application/x-xz":
		in, err = xz.NewReader(out)
		require.NoError(t, err)
	case "application/zstd":
		in, err = zstd.NewReader(out)
		require.NoError(t, err)
	default:
		t.Fatalf("unsupported file type: %s", fileType)
	}

	_, err = io.Copy(decompressedFile, in)
	require.NoError(t, err)

	info, err := blkid.ProbePath(decompressedPath, blkid.WithProbeLogger(zaptest.NewLogger(t)))
	require.NoError(t, err)

	assert.Equal(t, "gpt", info.Name)

	assert.Equal(t,
		xslices.Map(expectedPartitions, func(p partitionAssertion) string { return p.label }),
		xslices.Map(info.Parts, func(p blkid.NestedProbeResult) string { return pointer.SafeDeref(p.PartitionLabel) }),
	)

	if len(expectedPartitions) == len(info.Parts) {
		// if the length is not the same, the assertion above will fail anyways
		for i, p := range expectedPartitions {
			if p.size > 0 {
				assert.Equal(t, p.size, info.Parts[i].PartitionSize, "partition %s size mismatch", p.label)
			}
		}
	}
}

func downloadAssetAndValidateInitramfs(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, initramfsSpec initramfsSpec) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header, "Content-Disposition")

	d := t.TempDir()
	initramfsPath := filepath.Join(d, "initramfs.xz")

	out, err := os.Create(initramfsPath)
	require.NoError(t, err)

	size, err := io.Copy(out, body)
	require.NoError(t, err)

	require.NoError(t, out.Close())

	assertInitramfs(t, initramfsPath, talosVersion, initramfsSpec)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, size)
}

func downloadAssetAndValidateUKI(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, ukiSpec ukiSpec) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header, "Content-Disposition")

	d := t.TempDir()
	ukiPath := filepath.Join(d, "uki.efi")

	out, err := os.Create(ukiPath)
	require.NoError(t, err)

	size, err := io.Copy(out, body)
	require.NoError(t, err)

	require.NoError(t, out.Close())

	assertUKI(t, ukiPath, ukiSpec)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, size)
}

func downloadInstallerAndValidateUKI(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path, arch string, ukiSpec ukiSpec) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header, "Content-Disposition")

	d := t.TempDir()
	installerPath := filepath.Join(d, "installer.tar")

	out, err := os.Create(installerPath)
	require.NoError(t, err)

	size, err := io.Copy(out, body)
	require.NoError(t, err)

	require.NoError(t, out.Close())

	assertInstallerTarUKIArtifact(t, installerPath, arch, ukiSpec)

	downloadAssetAssertCached(ctx, t, baseURL, schematicID, talosVersion, path, size)
}

func downloadCmdlineAndMatch(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string, expected string) {
	t.Helper()

	resp := downloadAsset(ctx, t, baseURL, schematicID, talosVersion, path)
	body := resp.Body

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header, "Content-Disposition")

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

type partitionAssertion struct {
	label string
	size  uint64 // if zero, size is not checked
}

func sdBootPartitions(talosVersion string) []partitionAssertion {
	partitions := []partitionAssertion{
		{"EFI", quirks.New(talosVersion).PartitionSizes().UKIEFISize()},
		{"META", quirks.New(talosVersion).PartitionSizes().METASize()},
	}

	if !quirks.New(talosVersion).SkipDataPartitions() {
		partitions = append(partitions,
			partitionAssertion{
				"STATE", quirks.New(talosVersion).PartitionSizes().StateSize(),
			},
			partitionAssertion{
				"EPHEMERAL", 0,
			},
		)
	}

	return partitions
}

func dualBootPartitions(talosVersion string) []partitionAssertion {
	partitions := []partitionAssertion{
		{"EFI", quirks.New(talosVersion).PartitionSizes().UKIEFISize()},
		{"BIOS", quirks.New(talosVersion).PartitionSizes().GrubBIOSSize()},
		{"BOOT", quirks.New(talosVersion).PartitionSizes().GrubBootSize()},
		{"META", quirks.New(talosVersion).PartitionSizes().METASize()},
	}

	if !quirks.New(talosVersion).SkipDataPartitions() {
		partitions = append(partitions,
			partitionAssertion{
				"STATE", quirks.New(talosVersion).PartitionSizes().StateSize(),
			},
			partitionAssertion{
				"EPHEMERAL", 0,
			},
		)
	}

	return partitions
}

func grubPartitions(talosVersion string) []partitionAssertion {
	dbPartitions := dualBootPartitions(talosVersion)

	dbPartitions[0].size = quirks.New(talosVersion).PartitionSizes().GrubEFISize()

	return dbPartitions
}

func defaultPartitions(talosVersion, arch string) []partitionAssertion {
	switch arch {
	case "amd64":
		if quirks.New(talosVersion).UseSDBootForUEFI() {
			return dualBootPartitions(talosVersion)
		} else {
			return grubPartitions(talosVersion)
		}
	case "arm64":
		if quirks.New(talosVersion).UseSDBootForUEFI() {
			return sdBootPartitions(talosVersion)
		} else {
			return grubPartitions(talosVersion)
		}
	default:
		panic("defaultPartitions: unsupported architecture: " + arch)
	}
}

func testDownloadFrontend(ctx context.Context, t *testing.T, baseURL string) {
	const MiB = 1024 * 1024

	talosVersions := []string{
		"v1.11.0",
		"v1.10.2",
		"v1.9.4",
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

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 82724864, "1.8", 106475520, "1.9", 106475520, "1.10", 301*MiB, "1.11", 301*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 122007552, "1.8", 90738688, "1.9", 90738688, "1.10", 274*MiB, "1.11", 274*MiB),
					)
				})

				t.Run("custom filename", func(t *testing.T) {
					t.Parallel()

					response := downloadAssetWithFilename(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.iso", "custom-filename.iso")

					disposition := response.Header.Get("Content-Disposition")

					require.Equal(t, `attachment; filename="custom-filename.iso"`, disposition)
				})

				t.Run("secureboot iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 162*MiB, "1.8", 198*MiB, "1.9", 198*MiB, "1.10", 204*MiB, "1.11", 204*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-secureboot.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 232*MiB, "1.8", 169*MiB, "1.9", 169*MiB, "1.10", 186*MiB, "1.11", 186*MiB),
					)
				})

				t.Run("kernel", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-amd64", "application/vnd.microsoft.portable-executable",
						sizePicker(talosVersion, "1.5", 16708992, "1.8", 18727936, "1.9", 18727936, "1.10", 18727936, "1.11", 18727936),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "kernel-arm64", "application/vnd.microsoft.portable-executable",
						sizePicker(talosVersion, "1.5", 69356032, "1.8", 21787136, "1.9", 21787136, "1.10", 21787136, "1.11", 19407360),
					)
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

				t.Run("regular UKI", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-uki.efi", "application/vnd.microsoft.portable-executable",
						sizePicker(talosVersion, "1.5", 77691056, "1.8", 98469552, "1.9", 98469552, "1.10", 95*MiB, "1.11", 95*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-uki.efi", "application/vnd.microsoft.portable-executable",
						sizePicker(talosVersion, "1.5", 114564272, "1.8", 82733744, "1.9", 82733744, "1.10", 86*MiB, "1.11", 86*MiB),
					)
				})

				t.Run("SecureBoot UKI", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot-uki.efi", "application/vnd.microsoft.portable-executable",
						sizePicker(talosVersion, "1.5", 77691056, "1.8", 98469552, "1.9", 98469552, "1.10", 95*MiB, "1.11", 95*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-secureboot-uki.efi", "application/vnd.microsoft.portable-executable",
						sizePicker(talosVersion, "1.5", 114564272, "1.8", 82733744, "1.9", 82733744, "1.10", 86*MiB, "1.11", 86*MiB),
					)
				})

				t.Run("nocloud UKI", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=nocloud console=tty1 console=ttyS0 net.ifnames=0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

					if quirks.New(talosVersion).SupportsSELinux() {
						expected += " selinux=1"
					}

					if !quirks.New(talosVersion).SupportsIMA() {
						expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
					}

					downloadAssetAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "nocloud-amd64-uki.efi", ukiSpec{
						expectedCmdline: expected,
					})
				})

				t.Run("legacy installer image", func(t *testing.T) {
					if quirks.New(talosVersion).SupportsUnifiedInstaller() {
						t.Skip()
					}

					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-amd64.tar", "application/x-tar",
						sizePicker(talosVersion, "1.5", 167482880, "1.8", 185155584, "1.9", 136*MiB, "1.10", 127*MiB, "1.11", 127*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar",
						sizePicker(talosVersion, "1.5", 157*MiB, "1.8", 170119168, "1.9", 126*MiB, "1.10", 116*MiB, "1.11", 116*MiB),
					)
				})

				t.Run("installer image", func(t *testing.T) {
					if !quirks.New(talosVersion).SupportsUnifiedInstaller() {
						t.Skip()
					}

					t.Parallel()

					expectedMetalCmdlineAMD64 := "talos.platform=metal console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 selinux=1"
					expectedMetalCmdlineARM64 := "talos.platform=metal console=ttyAMA0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 selinux=1"

					expectedAWSCmdline := "talos.platform=aws console=tty1 console=ttyS0 net.ifnames=0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 selinux=1"
					expectedNoCloudCmdline := "talos.platform=nocloud console=tty1 console=ttyS0 net.ifnames=0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 selinux=1"

					if !quirks.New(talosVersion).SupportsIMA() {
						expectedMetalCmdlineAMD64 = strings.ReplaceAll(expectedMetalCmdlineAMD64, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
						expectedMetalCmdlineARM64 = strings.ReplaceAll(expectedMetalCmdlineARM64, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
						expectedAWSCmdline = strings.ReplaceAll(expectedAWSCmdline, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
						expectedNoCloudCmdline = strings.ReplaceAll(expectedNoCloudCmdline, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
					}

					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-amd64.tar", "amd64", ukiSpec{
						expectedCmdline: expectedMetalCmdlineAMD64,
					})
					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-installer-amd64.tar", "amd64", ukiSpec{
						expectedCmdline: expectedMetalCmdlineAMD64,
					})

					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-arm64.tar", "arm64", ukiSpec{
						expectedCmdline: expectedMetalCmdlineARM64,
					})
					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-installer-arm64.tar", "arm64", ukiSpec{
						expectedCmdline: expectedMetalCmdlineARM64,
					})

					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-installer-amd64.tar", "amd64", ukiSpec{
						expectedCmdline: expectedAWSCmdline,
					})

					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-installer-arm64.tar", "arm64", ukiSpec{
						expectedCmdline: expectedAWSCmdline,
					})

					downloadInstallerAndValidateUKI(ctx, t, baseURL, emptySchematicID, talosVersion, "nocloud-installer-amd64.tar", "amd64", ukiSpec{
						expectedCmdline: expectedNoCloudCmdline,
					})
				})

				t.Run("metal image", func(t *testing.T) {
					t.Parallel()

					checkCors(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.xz")

					downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 78472708, "1.8", 101464300, "1.9", 101464300, "1.10", 192*MiB, "1.11", 192*MiB),
						defaultPartitions(talosVersion, "amd64"),
					)
					downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 66625420, "1.8", 83998408, "1.9", 83998408, "1.10", 86*MiB, "1.11", 86*MiB),
						defaultPartitions(talosVersion, "arm64"),
					)
				})

				t.Run("metal zstd image", func(t *testing.T) {
					t.Parallel()

					downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.raw.zst", "application/zstd",
						sizePicker(talosVersion, "1.5", 78472708, "1.8", 100120864, "1.9", 100120864, "1.10", 191*MiB, "1.11", 191*MiB),
						defaultPartitions(talosVersion, "amd64"),
					)
					downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.raw.zst", "application/zstd",
						sizePicker(talosVersion, "1.5", 66_625_420, "1.8", 83_651_316, "1.9", 83_651_316, "1.10", 86*MiB, "1.11", 86*MiB),
						defaultPartitions(talosVersion, "arm64"),
					)
				})

				t.Run("metal qcow2 image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64.qcow2", "",
						sizePicker(talosVersion, "1.5", 75*MiB, "1.8", 94*MiB, "1.9", 93*MiB, "1.10", 191*MiB, "1.11", 191*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64.qcow2", "",
						sizePicker(talosVersion, "1.5", 108*MiB, "1.8", 79*MiB, "1.9", 85*MiB, "1.10", 86*MiB, "1.11", 86*MiB),
					)
				})

				t.Run("metal secureboot image", func(t *testing.T) {
					t.Parallel()

					downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 78472708, "1.8", 97975380, "1.9", 97975380, "1.10", 95*MiB, "1.11", 95*MiB),
						sdBootPartitions(talosVersion),
					)
					downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-arm64-secureboot.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 66625420, "1.8", 82420728, "1.9", 82420728, "1.10", 86*MiB, "1.11", 86*MiB),
						sdBootPartitions(talosVersion),
					)
				})

				t.Run("aws image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-amd64.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 78472708, "1.8", 103249176, "1.9", 103249176, "1.10", 193*MiB, "1.11", 193*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "aws-arm64.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 66625420, "1.8", 85783432, "1.9", 85783432, "1.10", 88*MiB, "1.11", 88*MiB),
					)
				})

				t.Run("gcp image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "gcp-amd64.raw.tar.gz", "application/gzip",
						sizePicker(talosVersion, "1.5", 78472708, "1.8", 102107964, "1.9", 102107964, "1.10", 192*MiB, "1.11", 192*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "gcp-arm64.raw.tar.gz", "application/gzip",
						sizePicker(talosVersion, "1.5", 70625420, "1.8", 84214192, "1.9", 84214192, "1.10", 95*MiB, "1.11", 95*MiB),
					)
				})

				t.Run("vmware image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "vmware-amd64.ova", "application/x-tar",
						sizePicker(talosVersion, "1.5", 79*MiB, "1.8", 98*MiB, "1.9", 98*MiB, "1.10", 197*MiB, "1.11", 197*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "vmware-arm64.ova", "application/x-tar",
						sizePicker(talosVersion, "1.5", 69*MiB, "1.8", 81*MiB, "1.9", 87*MiB, "1.10", 89*MiB, "1.11", 89*MiB),
					)
				})

				t.Run("rpi image", func(t *testing.T) {
					t.Parallel()

					if quirks.New(talosVersion).SupportsOverlay() {
						downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 136632380, grubPartitions(talosVersion))
					}
				})
			})

			t.Run("extensions schematic", func(t *testing.T) {
				t.Parallel()

				t.Run("iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 112222208, "1.8", 133283840, "1.9", 133283840, "1.10", 381*MiB, "1.11", 381*MiB))
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 150120448, "1.8", 115824640, "1.9", 115824640, "1.10", 349*MiB, "1.11", 349*MiB),
					)
				})

				t.Run("secureboot iso", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64-secureboot.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 214*MiB, "1.8", 250*MiB, "1.9", 250*MiB, "1.10", 257*MiB, "1.11", 257*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64-secureboot.iso", "application/x-iso9660-image",
						sizePicker(talosVersion, "1.5", 280*MiB, "1.8", 218*MiB, "1.9", 218*MiB, "1.10", 235*MiB, "1.11", 235*MiB),
					)
				})

				t.Run("metal image", func(t *testing.T) {
					t.Parallel()

					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-amd64.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 108049020, "1.8", 128244948, "1.9", 128244948, "1.10", 245*MiB, "1.11", 245*MiB),
					)
					downloadAssetAndMatchSize(ctx, t, baseURL, systemExtensionsSchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz",
						sizePicker(talosVersion, "1.5", 91484764, "1.8", 109057716, "1.9", 109057716, "1.10", 111*MiB, "1.11", 111*MiB),
					)
				})

				t.Run("rpi image", func(t *testing.T) {
					t.Parallel()

					if quirks.New(talosVersion).SupportsOverlay() {
						downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 136632380)
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
			downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar", 209*MiB)
		})

		t.Run("metal image", func(t *testing.T) {
			t.Parallel()

			// curl the image and `du -sh` on the image
			downloadAssetAndMatchSize(ctx, t, baseURL, rpiGenericOverlaySchematicID, talosVersion, "metal-arm64.raw.xz", "application/x-xz", 117*MiB)
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
				downloadAssetAndMatchSize(ctx, t, baseURL, emptySchematicID, talosVersion, "installer-arm64.tar", "application/x-tar", 214*MiB)
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

	t.Run("uki extra args", func(t *testing.T) {
		t.Parallel()

		for _, talosVersion := range talosVersions {
			t.Run(talosVersion, func(t *testing.T) {
				if !quirks.New(talosVersion).SupportsUnifiedInstaller() {
					t.Skip()
				}

				t.Parallel()

				expected := "talos.platform=metal console=tty0 console=ttyS0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

				if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
					expected = strings.ReplaceAll(expected, " console=ttyS0", "")
				}

				if quirks.New(talosVersion).SupportsSELinux() {
					expected += " selinux=1"
				}

				if !quirks.New(talosVersion).SupportsIMA() {
					expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
				}

				expected += " nolapic nomodeset"

				downloadAssetAndValidateUKI(ctx, t, baseURL, extraArgsSchematicID, talosVersion, "metal-amd64-uki.efi", ukiSpec{
					expectedCmdline: expected,
				})
			})
		}
	})

	t.Run("installer extra args", func(t *testing.T) {
		t.Parallel()

		for _, talosVersion := range talosVersions {
			t.Run(talosVersion, func(t *testing.T) {
				if !quirks.New(talosVersion).SupportsUnifiedInstaller() {
					t.Skip()
				}

				t.Parallel()

				expected := "talos.platform=metal console=tty0 console=ttyS0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

				if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
					expected = strings.ReplaceAll(expected, " console=ttyS0", "")
				}

				if quirks.New(talosVersion).SupportsSELinux() {
					expected += " selinux=1"
				}

				if !quirks.New(talosVersion).SupportsIMA() {
					expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
				}

				expected += " nolapic nomodeset"

				downloadInstallerAndValidateUKI(ctx, t, baseURL, extraArgsSchematicID, talosVersion, "installer-amd64.tar", "amd64", ukiSpec{
					expectedCmdline: expected,
				})

				downloadInstallerAndValidateUKI(ctx, t, baseURL, extraArgsSchematicID, talosVersion, "metal-installer-amd64.tar", "amd64", ukiSpec{
					expectedCmdline: expected,
				})
			})
		}
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

					if quirks.New(talosVersion).SupportsSELinux() {
						expected += " selinux=1"
					}

					if !quirks.New(talosVersion).SupportsIMA() {
						expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
					}

					downloadCmdlineAndMatch(ctx, t, baseURL, emptySchematicID, talosVersion, "cmdline-metal-amd64", expected)
				})

				t.Run("default aws", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=aws console=tty1 console=ttyS0 net.ifnames=0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

					if quirks.New(talosVersion).SupportsSELinux() {
						expected += " selinux=1"
					}

					if !quirks.New(talosVersion).SupportsIMA() {
						expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
					}

					downloadCmdlineAndMatch(ctx, t, baseURL, emptySchematicID, talosVersion, "cmdline-aws-arm64", expected)
				})

				t.Run("extra metal", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

					if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
						expected = strings.ReplaceAll(expected, " console=ttyS0", "")
					}

					if quirks.New(talosVersion).SupportsSELinux() {
						expected += " selinux=1"
					}

					if !quirks.New(talosVersion).SupportsIMA() {
						expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
					}

					expected += " nolapic nomodeset"

					downloadCmdlineAndMatch(ctx, t, baseURL, extraArgsSchematicID, talosVersion, "cmdline-metal-amd64", expected)
				})

				t.Run("meta contents", func(t *testing.T) {
					t.Parallel()

					expected := "talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512"

					if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
						expected = strings.ReplaceAll(expected, " console=ttyS0", "")
					}

					if quirks.New(talosVersion).SupportsSELinux() {
						expected += " selinux=1"
					}

					if !quirks.New(talosVersion).SupportsIMA() {
						expected = strings.ReplaceAll(expected, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
					}

					expected += " talos.environment=INSTALLER_META_BASE64=MHhhPXsiZXh0ZXJuYWxJUHMiOlsiMS4yLjMuNCJdfQ=="

					downloadCmdlineAndMatch(ctx, t, baseURL, metaSchematicID, talosVersion, "cmdline-metal-amd64", expected)
				})
			})
		}
	})

	t.Run("bootloader_override", func(t *testing.T) {
		t.Parallel()

		t.Run("grub", func(t *testing.T) {
			t.Parallel()

			downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, grubBootloaderOverrideSchematicID, "v1.12.0", "metal-amd64.raw.zst", "application/zstd", 95*MiB, grubPartitions("v1.12.0"))
		})

		t.Run("sd-boot", func(t *testing.T) {
			t.Parallel()

			downloadDiskImageMatchSizeAndPartitions(ctx, t, baseURL, sdBootBootloaderOverrideSchematicID, "v1.12.0", "metal-amd64.raw.zst", "application/zstd", 93*MiB, sdBootPartitions("v1.12.0"))
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
