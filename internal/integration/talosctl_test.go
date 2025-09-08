// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func downloadTalosctlInvalid(ctx context.Context, t *testing.T, baseURL string, talosVersion, path string, expectedCode int) string {
	t.Helper()

	resp := downloadTalosctl(ctx, t, baseURL, talosVersion, path)
	assert.Equal(t, expectedCode, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func downloadTalosctl(ctx context.Context, t *testing.T, baseURL string, talosVersion, path string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/talosctl/"+talosVersion+"/"+path, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

func testTalosctlFrontend(ctx context.Context, t *testing.T, baseURL string) {
	talosVersions := map[string][]string{
		"v1.11.0": {
			"talosctl-linux-amd64", "talosctl-linux-arm64", "talosctl-linux-armv7",
			"talosctl-darwin-amd64", "talosctl-darwin-arm64",
			"talosctl-freebsd-amd64", "talosctl-freebsd-arm64",
			"talosctl-windows-amd64.exe", "talosctl-windows-arm64.exe",
		},
	}

	for talosVersion, binaries := range talosVersions {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			for _, bin := range binaries {
				t.Run(bin, func(t *testing.T) {
					t.Parallel()

					res := downloadTalosctl(ctx, t, baseURL, talosVersion, bin)
					require.NotNil(t, res)
					assert.Equal(t, http.StatusOK, res.StatusCode)
				})
			}
		})
	}

	talosVersionsInvalid := map[string][]string{
		"v1.10.0": {"talosctl-linux-amd64"},
		"v1.8.0":  {"talosctl-linux-amd64"},
		"v1.3.0":  {"talosctl-linux-amd64"},
	}

	for talosVersion, binaries := range talosVersionsInvalid {
		t.Run(talosVersion, func(t *testing.T) {
			t.Parallel()

			for _, bin := range binaries {
				t.Run(bin, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, fmt.Sprintf("version %s is not available\n", strings.TrimPrefix(talosVersion, "v")),
						downloadTalosctlInvalid(ctx, t, baseURL, talosVersion, bin, http.StatusNotFound),
					)
				})
			}
		})
	}
}
