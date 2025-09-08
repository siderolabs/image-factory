// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func downloadPXE(ctx context.Context, t *testing.T, baseURL string, schematicID, talosVersion, path string) string {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/pxe/"+schematicID+"/"+talosVersion+"/"+path, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func checkPXENoRedirect(ctx context.Context, t *testing.T, pxe string, files ...string) {
	t.Helper()

	scanner := bufio.NewScanner(strings.NewReader(pxe))

	var urls []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		for _, file := range files {
			if strings.HasPrefix(line, file+" ") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					urls = append(urls, fields[1])
				}
			}
		}
	}

	require.NoError(t, scanner.Err())

	for _, testURL := range urls {
		downloadNoRedirect(ctx, t, testURL)
	}
}

func fixupCmdline(cmdline string, talosVersion string) string {
	if quirks.New(talosVersion).SupportsSELinux() {
		cmdline = strings.ReplaceAll(cmdline, "sha512\n", "sha512 selinux=1\n")
		cmdline = strings.ReplaceAll(cmdline, " lockdown=confidentiality\n", " selinux=1 lockdown=confidentiality\n")
	}

	if !quirks.New(talosVersion).SupportsMetalPlatformConsoleTTYS0() {
		cmdline = strings.ReplaceAll(cmdline, " console=ttyS0", "")
	}

	if !quirks.New(talosVersion).SupportsIMA() {
		cmdline = strings.ReplaceAll(cmdline, " ima_template=ima-ng ima_appraise=fix ima_hash=sha512", "")
	}

	return cmdline
}

func testPXEFrontend(ctx context.Context, t *testing.T, baseURL, pxeURL string) {
	talosVersions := []string{
		"v1.5.0",
		"v1.11.0",
	}

	const (
		metalInsecureExpected   = "#!ipxe\n\nimgfree\nkernel ENDPOINT/image/CONFIG/VERSION/kernel-amd64 talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512\ninitrd ENDPOINT/image/CONFIG/VERSION/initramfs-amd64.xz\nboot\n"    //nolint:lll
		equinixInsecureExpected = "#!ipxe\n\nimgfree\nkernel ENDPOINT/image/CONFIG/VERSION/kernel-amd64 talos.platform=equinixMetal console=ttyS1,115200n8 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512\ninitrd ENDPOINT/image/CONFIG/VERSION/initramfs-amd64.xz\nboot\n" //nolint:lll
		securebootExpected      = "#!ipxe\n\nimgfree\nkernel ENDPOINT/image/CONFIG/VERSION/metal-amd64-secureboot-uki.efi talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 lockdown=confidentiality\nboot\n"
	)

	for _, talosVersion := range talosVersions {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			t.Run("metal-amd64", func(t *testing.T) {
				t.Parallel()

				pxe := downloadPXE(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64")

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(fixupCmdline(metalInsecureExpected, talosVersion), "ENDPOINT", pxeURL),
							"CONFIG", emptySchematicID,
						),
						"VERSION", talosVersion,
					),
					pxe,
				)

				checkPXENoRedirect(ctx, t, pxe, "kernel", "initrd")
			})

			t.Run("metal-x86_64", func(t *testing.T) {
				t.Parallel()

				pxe := downloadPXE(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-x86_64")

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(fixupCmdline(metalInsecureExpected, talosVersion), "ENDPOINT", pxeURL),
							"CONFIG", emptySchematicID,
						),
						"VERSION", talosVersion,
					),
					pxe,
				)

				checkPXENoRedirect(ctx, t, pxe, "kernel", "initrd")
			})

			t.Run("equinix-arm64", func(t *testing.T) {
				t.Parallel()

				pxe := downloadPXE(ctx, t, baseURL, emptySchematicID, talosVersion, "equinixMetal-amd64")

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(fixupCmdline(equinixInsecureExpected, talosVersion), "ENDPOINT", pxeURL),
							"CONFIG", emptySchematicID,
						),
						"VERSION", talosVersion,
					),
					pxe,
				)

				checkPXENoRedirect(ctx, t, pxe, "kernel", "initrd")
			})

			t.Run("secureboot-amd64", func(t *testing.T) {
				t.Parallel()

				pxe := downloadPXE(ctx, t, baseURL, emptySchematicID, talosVersion, "metal-amd64-secureboot")

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(fixupCmdline(securebootExpected, talosVersion), "ENDPOINT", pxeURL),
							"CONFIG", emptySchematicID,
						),
						"VERSION", talosVersion,
					),
					pxe,
				)

				checkPXENoRedirect(ctx, t, pxe, "kernel")
			})
		})
	}
}
