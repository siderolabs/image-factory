// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

const checksumTestTalosVersion = "v1.12.4"

func downloadChecksum(ctx context.Context, t *testing.T, baseURL, schematicID, version, path, suffix, method string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/image/"+schematicID+"/"+version+"/"+path+suffix, nil)
	require.NoError(t, err)

	addTestAuth(req)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

// checksumAlgo holds the parameters needed to test a single checksum algorithm.
type checksumAlgo struct {
	suffix    string
	digestLen int
	newHasher func() hash.Hash
}

var checksumAlgos = []checksumAlgo{
	{".sha512", 128, sha512.New},
	{".sha256", 64, sha256.New},
}

func testChecksumFrontend(ctx context.Context, t *testing.T, baseURL string) {
	if !enterprise.Enabled() {
		t.Run("checksum not available", func(t *testing.T) {
			t.Parallel()

			resp := downloadChecksum(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, "kernel-amd64", ".sha512", http.MethodGet)
			assert.Equal(t, http.StatusPaymentRequired, resp.StatusCode)
		})

		return
	}

	for _, algo := range checksumAlgos {
		t.Run("GET empty schematic"+algo.suffix, func(t *testing.T) {
			t.Parallel()

			assetPath := "kernel-amd64"

			resp := downloadChecksum(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath, algo.suffix, http.MethodGet)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
			assert.Equal(t,
				`attachment; filename="`+assetPath+algo.suffix+`"`,
				resp.Header.Get("Content-Disposition"),
			)

			contentLength := resp.Header.Get("Content-Length")
			require.NotEmpty(t, contentLength)

			clValue, err := strconv.ParseInt(contentLength, 10, 64)
			require.NoError(t, err)
			assert.Greater(t, clValue, int64(0))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			line := string(body)
			parts := strings.SplitN(strings.TrimSuffix(line, "\n"), "  ", 2)
			require.Len(t, parts, 2, "expected '<hash>  <filename>' format, got: %q", line)
			assert.Len(t, parts[0], algo.digestLen, "hex digest should be %d characters for %s", algo.digestLen, algo.suffix)
			assert.Equal(t, assetPath, parts[1])
		})

		t.Run("HEAD empty schematic"+algo.suffix, func(t *testing.T) {
			t.Parallel()

			assetPath := "kernel-amd64"

			resp := downloadChecksum(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath, algo.suffix, http.MethodHead)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
			assert.Equal(t,
				`attachment; filename="`+assetPath+algo.suffix+`"`,
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

		t.Run("validate checksum against asset"+algo.suffix, func(t *testing.T) {
			t.Parallel()

			assetPath := "kernel-amd64"

			checksumResp := downloadChecksum(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath, algo.suffix, http.MethodGet)
			require.Equal(t, http.StatusOK, checksumResp.StatusCode)

			checksumBody, err := io.ReadAll(checksumResp.Body)
			require.NoError(t, err)

			parts := strings.SplitN(strings.TrimSuffix(string(checksumBody), "\n"), "  ", 2)
			require.Len(t, parts, 2)

			expectedHash := parts[0]

			assetResp := downloadAsset(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath)
			require.Equal(t, http.StatusOK, assetResp.StatusCode)

			hasher := algo.newHasher()
			_, err = io.Copy(hasher, assetResp.Body)
			require.NoError(t, err)

			actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
			assert.Equal(t, expectedHash, actualHash, "checksum from checksum endpoint should match %s of downloaded asset", algo.suffix)
		})
	}

	t.Run("reproducibility", func(t *testing.T) {
		t.Parallel()

		assetPath := "kernel-amd64"

		assetResp1 := downloadAsset(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath)
		require.Equal(t, http.StatusOK, assetResp1.StatusCode)

		hasher1 := sha512.New()
		_, err := io.Copy(hasher1, assetResp1.Body)
		require.NoError(t, err)

		hash1 := fmt.Sprintf("%x", hasher1.Sum(nil))

		assetResp2 := downloadAsset(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath)
		require.Equal(t, http.StatusOK, assetResp2.StatusCode)

		hasher2 := sha512.New()
		_, err = io.Copy(hasher2, assetResp2.Body)
		require.NoError(t, err)

		hash2 := fmt.Sprintf("%x", hasher2.Sum(nil))

		assert.Equal(t, hash1, hash2, "asset should be reproducible: two downloads should produce identical SHA-512 hashes")

		checksumResp := downloadChecksum(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, assetPath, ".sha512", http.MethodGet)
		require.Equal(t, http.StatusOK, checksumResp.StatusCode)

		checksumBody, err := io.ReadAll(checksumResp.Body)
		require.NoError(t, err)

		parts := strings.SplitN(strings.TrimSuffix(string(checksumBody), "\n"), "  ", 2)
		require.Len(t, parts, 2)

		assert.Equal(t, hash1, parts[0], "checksum endpoint should match the reproducible asset hash")
	})

	t.Run("non-existent schematic", func(t *testing.T) {
		t.Parallel()

		resp := downloadChecksum(ctx, t, baseURL, nonexistentSchematicID, checksumTestTalosVersion, "kernel-amd64", ".sha512", http.MethodGet)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()

		resp := downloadChecksum(ctx, t, baseURL, emptySchematicID, checksumTestTalosVersion, "invalid-path.xyz", ".sha512", http.MethodGet)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
