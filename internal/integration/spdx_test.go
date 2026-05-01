// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	spdxjson "github.com/spdx/tools-golang/json"
	"github.com/spdx/tools-golang/spdx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

const spdxTestTalosVersion = "v1.12.4"

func downloadSPDX(ctx context.Context, t *testing.T, baseURL, schematicID, version, arch, method string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/spdx/"+schematicID+"/"+version+"/"+arch, nil)
	require.NoError(t, err)

	addTestAuth(req)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func testSPDXFrontend(ctx context.Context, t *testing.T, baseURL string) {
	if !enterprise.Enabled() {
		t.Run("endpoint not registered", func(t *testing.T) {
			t.Parallel()

			resp := downloadSPDX(ctx, t, baseURL, emptySchematicID, spdxTestTalosVersion, "amd64", http.MethodGet)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		return
	}

	t.Run("GET empty schematic", func(t *testing.T) {
		t.Parallel()

		resp := downloadSPDX(ctx, t, baseURL, emptySchematicID, spdxTestTalosVersion, "amd64", http.MethodGet)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		assert.Equal(t, "application/spdx+json", resp.Header.Get("Content-Type"))
		assert.Equal(t,
			`attachment; filename="`+emptySchematicID+`-`+spdxTestTalosVersion+`-amd64.spdx.json"`,
			resp.Header.Get("Content-Disposition"),
		)

		contentLength := resp.Header.Get("Content-Length")
		require.NotEmpty(t, contentLength)

		clValue, err := strconv.ParseInt(contentLength, 10, 64)
		require.NoError(t, err)
		assert.Greater(t, clValue, int64(0))

		doc, err := spdxjson.Read(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, spdx.Version, doc.SPDXVersion)
		assert.Equal(t, spdx.DataLicense, doc.DataLicense)
		assert.Contains(t, doc.DocumentName, "talos-"+emptySchematicID)
		assert.Contains(t, doc.DocumentNamespace, "https://")
		assert.Contains(t, doc.DocumentNamespace, "/spdx/"+emptySchematicID)
		require.NotNil(t, doc.CreationInfo)
		assert.NotEmpty(t, doc.CreationInfo.Creators)
	})

	t.Run("HEAD empty schematic", func(t *testing.T) {
		t.Parallel()

		resp := downloadSPDX(ctx, t, baseURL, emptySchematicID, spdxTestTalosVersion, "amd64", http.MethodHead)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		assert.Equal(t, "application/spdx+json", resp.Header.Get("Content-Type"))
		assert.Equal(t,
			`attachment; filename="`+emptySchematicID+`-`+spdxTestTalosVersion+`-amd64.spdx.json"`,
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

	t.Run("GET system extensions schematic", func(t *testing.T) {
		t.Parallel()

		resp := downloadSPDX(ctx, t, baseURL, systemExtensionsSchematicID, spdxTestTalosVersion, "amd64", http.MethodGet)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		assert.Equal(t, "application/spdx+json", resp.Header.Get("Content-Type"))

		doc, err := spdxjson.Read(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, spdx.Version, doc.SPDXVersion)
		assert.Equal(t, spdx.DataLicense, doc.DataLicense)
		assert.Contains(t, doc.DocumentName, "talos-"+systemExtensionsSchematicID)

		// extensions schematic should have more packages than an empty schematic
		// as it merges SPDX documents from extensions as well
		assert.Greater(t, len(doc.Packages), 0)
	})

	t.Run("non-existent schematic", func(t *testing.T) {
		t.Parallel()

		resp := downloadSPDX(ctx, t, baseURL, nonexistentSchematicID, spdxTestTalosVersion, "amd64", http.MethodGet)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("invalid version", func(t *testing.T) {
		t.Parallel()

		for _, version := range []string{
			"not-a-version",
			"v1.9.4", // unsupported Talos version
		} {
			t.Run(version, func(t *testing.T) {
				t.Parallel()

				resp := downloadSPDX(ctx, t, baseURL, emptySchematicID, version, "amd64", http.MethodGet)
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			})
		}
	})

	t.Run("invalid architecture", func(t *testing.T) {
		t.Parallel()

		resp := downloadSPDX(ctx, t, baseURL, emptySchematicID, spdxTestTalosVersion, "invalid-arch", http.MethodGet)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("cached response", func(t *testing.T) {
		t.Parallel()

		// first request: build the SPDX bundle (may be slow)
		resp1 := downloadSPDX(ctx, t, baseURL, emptySchematicID, spdxTestTalosVersion, "amd64", http.MethodGet)
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		firstSize, err := io.Copy(io.Discard, resp1.Body)
		require.NoError(t, err)

		// second request: should be served from cache
		start := time.Now()

		resp2 := downloadSPDX(ctx, t, baseURL, emptySchematicID, spdxTestTalosVersion, "amd64", http.MethodGet)
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		secondSize, err := io.Copy(io.Discard, resp2.Body)
		require.NoError(t, err)

		assert.Equal(t, firstSize, secondSize)
		assert.Less(t, time.Since(start), 60*time.Second)
	})
}
