// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

const vexTestTalosVersion = "v1.13.0"

func downloadVEX(ctx context.Context, t *testing.T, baseURL, version, method string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/vex/"+version+"/vex.json", nil)
	require.NoError(t, err)

	addTestAuth(req)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func testVEXFrontend(ctx context.Context, t *testing.T, baseURL string) {
	if !enterprise.Enabled() {
		t.Run("endpoint not registered", func(t *testing.T) {
			t.Parallel()

			resp := downloadVEX(ctx, t, baseURL, vexTestTalosVersion, http.MethodGet)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		return
	}

	t.Run("GET valid version", func(t *testing.T) {
		t.Parallel()

		resp := downloadVEX(ctx, t, baseURL, vexTestTalosVersion, http.MethodGet)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Equal(t,
			`attachment; filename="`+vexTestTalosVersion+`.vex.json"`,
			resp.Header.Get("Content-Disposition"),
		)

		contentLength := resp.Header.Get("Content-Length")
		require.NotEmpty(t, contentLength)

		clValue, err := strconv.ParseInt(contentLength, 10, 64)
		require.NoError(t, err)
		assert.Greater(t, clValue, int64(0))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.True(t, json.Valid(body))
	})

	t.Run("HEAD valid version", func(t *testing.T) {
		t.Parallel()

		resp := downloadVEX(ctx, t, baseURL, vexTestTalosVersion, http.MethodHead)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Equal(t,
			`attachment; filename="`+vexTestTalosVersion+`.vex.json"`,
			resp.Header.Get("Content-Disposition"),
		)

		contentLength := resp.Header.Get("Content-Length")
		require.NotEmpty(t, contentLength)

		clValue, err := strconv.ParseInt(contentLength, 10, 64)
		require.NoError(t, err)
		assert.Greater(t, clValue, int64(0))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Empty(t, body)
	})

	t.Run("version without v prefix", func(t *testing.T) {
		t.Parallel()

		resp := downloadVEX(ctx, t, baseURL, "1.13.0", http.MethodGet)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// handler adds the v prefix
		assert.Equal(t,
			`attachment; filename="v1.13.0.vex.json"`,
			resp.Header.Get("Content-Disposition"),
		)
	})

	t.Run("invalid version", func(t *testing.T) {
		t.Parallel()

		for _, version := range []string{
			"not-a-version",
			"v1.12.0", // below availableFrom (1.13.0)
		} {
			t.Run(version, func(t *testing.T) {
				t.Parallel()

				resp := downloadVEX(ctx, t, baseURL, version, http.MethodGet)
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			})
		}
	})

	t.Run("cached response", func(t *testing.T) {
		t.Parallel()

		// first request: fetch from OCI registry (may be slow)
		resp1 := downloadVEX(ctx, t, baseURL, vexTestTalosVersion, http.MethodGet)
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		firstSize, err := io.Copy(io.Discard, resp1.Body)
		require.NoError(t, err)

		// second request: served from in-memory cache
		start := time.Now()

		resp2 := downloadVEX(ctx, t, baseURL, vexTestTalosVersion, http.MethodGet)
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		secondSize, err := io.Copy(io.Discard, resp2.Body)
		require.NoError(t, err)

		assert.Equal(t, firstSize, secondSize)
		assert.Less(t, time.Since(start), 5*time.Second)
	})
}
