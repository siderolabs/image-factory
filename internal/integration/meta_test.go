// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/client"
)

func getVersions(ctx context.Context, t *testing.T, c *client.Client) []string {
	t.Helper()

	versions, err := c.Versions(ctx)
	require.NoError(t, err)

	return versions
}

func getExtensions(ctx context.Context, t *testing.T, c *client.Client, talosVersion string) []client.ExtensionInfo {
	t.Helper()

	versions, err := c.ExtensionsVersions(ctx, talosVersion)
	require.NoError(t, err)

	return versions
}

func testMetaFrontend(ctx context.Context, t *testing.T, baseURL string) {
	c, err := client.New(baseURL)
	require.NoError(t, err)

	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		versions := getVersions(ctx, t, c)

		assert.Greater(t, len(versions), 10)
	})

	t.Run("extensions", func(t *testing.T) {
		t.Parallel()

		talosVersions := []string{
			"v1.5.0",
			"v1.5.1",
		}

		for _, talosVersion := range talosVersions {
			t.Run(talosVersion, func(t *testing.T) {
				t.Parallel()

				extensions := getExtensions(ctx, t, c, talosVersion)

				names := xslices.Map(extensions, func(ext client.ExtensionInfo) string {
					return ext.Name
				})

				assert.Contains(t, names, "siderolabs/amd-ucode")
				assert.Contains(t, names, "siderolabs/gvisor")
				assert.Contains(t, names, "siderolabs/nvidia-open-gpu-kernel-modules")
			})
		}
	})
}
