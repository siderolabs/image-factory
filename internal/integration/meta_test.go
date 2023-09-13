// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getVersions(ctx context.Context, t *testing.T, baseURL string) []string {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/versions", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var versions []string

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&versions))

	return versions
}

type extensionInfo struct {
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

func getExtensions(ctx context.Context, t *testing.T, baseURL, talosVersion string) []extensionInfo {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/version/"+talosVersion+"/extensions/official", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var extensions []extensionInfo

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&extensions))

	return extensions
}

func testMetaFrontend(ctx context.Context, t *testing.T, baseURL string) {
	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		versions := getVersions(ctx, t, baseURL)

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

				extensions := getExtensions(ctx, t, baseURL, talosVersion)

				names := xslices.Map(extensions, func(ext extensionInfo) string {
					return ext.Name
				})

				assert.Contains(t, names, "siderolabs/amd-ucode")
				assert.Contains(t, names, "siderolabs/gvisor")
				assert.Contains(t, names, "siderolabs/nvidia-open-gpu-kernel-modules")
			})
		}
	})
}
