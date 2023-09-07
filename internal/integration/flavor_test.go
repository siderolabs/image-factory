// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-service/pkg/flavor"
)

// well known flavor IDs, they will be created with the test run
const (
	emptyFlavorID     = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
	extraArgsFlavorID = "e0fb1129bbbdfb5d002e94af4cdce712a8370e850950a33a242d4c3f178c532d"
)

func createFlavor(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/flavor", bytes.NewReader(marshalled))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func createFlavorGetID(ctx context.Context, t *testing.T, baseURL string, flavor flavor.Flavor) string {
	t.Helper()

	marshalled, err := flavor.Marshal()
	require.NoError(t, err)

	resp := createFlavor(ctx, t, baseURL, marshalled)

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var respBody struct {
		ID string `json:"id"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))

	return respBody.ID
}

func createFlavorInvalid(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) string {
	t.Helper()

	resp := createFlavor(ctx, t, baseURL, marshalled)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func testFlavor(ctx context.Context, t *testing.T, baseURL string) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, emptyFlavorID, createFlavorGetID(ctx, t, baseURL, flavor.Flavor{}))
	})

	t.Run("kernel args", func(t *testing.T) {
		assert.Equal(t, extraArgsFlavorID, createFlavorGetID(ctx, t, baseURL,
			flavor.Flavor{
				Customization: flavor.Customization{
					ExtraKernelArgs: []string{"nolapic", "nomodeset"},
				},
			},
		))
	})

	t.Run("empty once again", func(t *testing.T) {
		assert.Equal(t, emptyFlavorID, createFlavorGetID(ctx, t, baseURL, flavor.Flavor{}))
	})

	t.Run("invalid", func(t *testing.T) {
		assert.Equal(t, "yaml: unmarshal errors:\n  line 1: field something not found in type flavor.Flavor\n", createFlavorInvalid(ctx, t, baseURL, []byte(`something:`)))
	})

	t.Run("new flavor", func(t *testing.T) {
		// create a new random flavor, as the flavor is persisted, and we want to test uploading new config
		randomKernelArg := hex.EncodeToString(randomBytes(t, 32))

		assert.Len(t, createFlavorGetID(ctx, t, baseURL,
			flavor.Flavor{
				Customization: flavor.Customization{
					ExtraKernelArgs: []string{randomKernelArg},
				},
			},
		), 64)
	})
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()

	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)

	return b
}
