// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func downloadPXE(ctx context.Context, t *testing.T, baseURL string, configurationID, talosVersion, path string) string {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/pxe/"+configurationID+"/"+talosVersion+"/"+path, nil)
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

func testPXEFrontend(ctx context.Context, t *testing.T, baseURL string) {
	talosVersions := []string{
		"v1.5.0",
		"v1.5.1",
	}

	const (
		metalInsecureExpected   = "#!ipxe\n\nkernel ENDPOINT/image/CONFIG/VERSION/kernel-amd64 talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512\ninitrd ENDPOINT/image/CONFIG/VERSION/initramfs-amd64.xz\nboot\n"    //nolint:lll
		equinixInsecureExpected = "#!ipxe\n\nkernel ENDPOINT/image/CONFIG/VERSION/kernel-amd64 talos.platform=equinixMetal console=ttyS1,115200n8 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512\ninitrd ENDPOINT/image/CONFIG/VERSION/initramfs-amd64.xz\nboot\n" //nolint:lll
		securebootExpected      = "#!ipxe\n\nkernel ENDPOINT/image/CONFIG/VERSION/metal-amd64-secureboot.uki.efi\nboot\n"
	)

	for _, talosVersion := range talosVersions {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			t.Run("metal-amd64", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(metalInsecureExpected, "ENDPOINT", baseURL),
							"CONFIG", emptyConfigurationID,
						),
						"VERSION", talosVersion,
					),
					downloadPXE(ctx, t, baseURL, emptyConfigurationID, talosVersion, "metal-amd64"),
				)
			})

			t.Run("equinix-arm64", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(equinixInsecureExpected, "ENDPOINT", baseURL),
							"CONFIG", emptyConfigurationID,
						),
						"VERSION", talosVersion,
					),
					downloadPXE(ctx, t, baseURL, emptyConfigurationID, talosVersion, "equinixMetal-amd64"),
				)
			})

			t.Run("secureboot-amd64", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t,
					strings.ReplaceAll(
						strings.ReplaceAll(
							strings.ReplaceAll(securebootExpected, "ENDPOINT", baseURL),
							"CONFIG", emptyConfigurationID,
						),
						"VERSION", talosVersion,
					),
					downloadPXE(ctx, t, baseURL, emptyConfigurationID, talosVersion, "metal-amd64-secureboot"),
				)
			})
		})
	}
}
