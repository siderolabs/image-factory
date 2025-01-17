// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skyssolutions/siderolabs-image-factory/pkg/client"
)

func testMetaFrontend(ctx context.Context, t *testing.T, baseURL string) {
	c, err := client.New(baseURL)
	require.NoError(t, err)

	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		versions, err := c.Versions(ctx)
		require.NoError(t, err)

		assert.Greater(t, len(versions), 10)
	})

	t.Run("extensions", func(t *testing.T) {
		t.Parallel()

		talosVersions := []string{
			"v1.5.0",
			"v1.5.1",
			"v1.6.0",
		}

		for _, talosVersion := range talosVersions {
			t.Run(talosVersion, func(t *testing.T) {
				t.Parallel()

				extensions, err := c.ExtensionsVersions(ctx, talosVersion)
				require.NoError(t, err)

				names := xslices.Map(extensions, func(ext client.ExtensionInfo) string {
					return ext.Name
				})

				assert.Contains(t, names, "siderolabs/amd-ucode")
				assert.Contains(t, names, "siderolabs/gvisor")
				assert.Contains(t, names, "siderolabs/nvidia-open-gpu-kernel-modules")
			})
		}

		t.Run("invalid version", func(t *testing.T) {
			t.Parallel()

			_, err := c.ExtensionsVersions(ctx, "v1.5.0-alpha.0")
			require.Error(t, err)

			var httpError *client.HTTPError
			require.ErrorAs(t, err, &httpError)

			assert.Equal(t, http.StatusNotFound, httpError.Code)
		})
	})

	t.Run("overlays", func(t *testing.T) {
		t.Parallel()

		testData := []struct {
			version  string
			expected []string
		}{
			{
				version:  "v1.5.0",
				expected: nil,
			},
			{
				version: "v1.7.0",
				expected: []string{
					"rpi_generic",
					"rockpi4",
					"rockpi4c",
					"rock4cplus",
					"nanopi-r4s",
					"rock64",
					"orangepi-r1-plus-lts",
					"jetson_nano",
					"bananapi_m64",
					"libretech_all_h3_cc_h5",
					"pine64",
				},
			},
		}

		for _, tt := range testData {
			t.Run(tt.version, func(t *testing.T) {
				t.Parallel()

				overlays, err := c.OverlaysVersions(ctx, tt.version)
				require.NoError(t, err)

				names := xslices.Map(overlays, func(overlay client.OverlayInfo) string {
					return overlay.Name
				})

				assert.Equal(t, tt.expected, names)
			})
		}
	})
}
