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

	"github.com/siderolabs/image-factory/pkg/schematic"
)

// well known schematic IDs, they will be created with the test run
const (
	emptySchematicID            = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
	extraArgsSchematicID        = "e0fb1129bbbdfb5d002e94af4cdce712a8370e850950a33a242d4c3f178c532d"
	systemExtensionsSchematicID = "51ff3e49313773332729a5c04e57af0dbe2e6d3f65ff638e6d4c3a05065fefff"
)

func createSchematic(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/schematics", bytes.NewReader(marshalled))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func createSchematicGetID(ctx context.Context, t *testing.T, baseURL string, schematic schematic.Schematic) string {
	t.Helper()

	marshalled, err := schematic.Marshal()
	require.NoError(t, err)

	resp := createSchematic(ctx, t, baseURL, marshalled)

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var respBody struct {
		ID string `json:"id"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))

	return respBody.ID
}

func createSchematicInvalid(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) string {
	t.Helper()

	resp := createSchematic(ctx, t, baseURL, marshalled)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func testSchematic(ctx context.Context, t *testing.T, baseURL string) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, emptySchematicID, createSchematicGetID(ctx, t, baseURL, schematic.Schematic{}))
	})

	t.Run("kernel args", func(t *testing.T) {
		assert.Equal(t, extraArgsSchematicID, createSchematicGetID(ctx, t, baseURL,
			schematic.Schematic{
				Customization: schematic.Customization{
					ExtraKernelArgs: []string{"nolapic", "nomodeset"},
				},
			},
		))
	})

	t.Run("system extensions", func(t *testing.T) {
		assert.Equal(t, systemExtensionsSchematicID, createSchematicGetID(ctx, t, baseURL,
			schematic.Schematic{
				Customization: schematic.Customization{
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/amd-ucode",
							"siderolabs/gvisor",
							"siderolabs/gasket-driver",
						},
					},
				},
			},
		))
	})

	t.Run("empty once again", func(t *testing.T) {
		assert.Equal(t, emptySchematicID, createSchematicGetID(ctx, t, baseURL, schematic.Schematic{}))
	})

	t.Run("invalid", func(t *testing.T) {
		assert.Equal(t, "yaml: unmarshal errors:\n  line 1: field something not found in type schematic.Schematic\n", createSchematicInvalid(ctx, t, baseURL, []byte(`something:`)))
	})

	t.Run("new schematic", func(t *testing.T) {
		// create a new random schematic, as the schematic is persisted, and we want to test uploading new config
		randomKernelArg := hex.EncodeToString(randomBytes(t, 32))

		assert.Len(t, createSchematicGetID(ctx, t, baseURL,
			schematic.Schematic{
				Customization: schematic.Customization{
					ExtraKernelArgs: []string{randomKernelArg},
					SystemExtensions: schematic.SystemExtensions{
						OfficialExtensions: []string{
							"siderolabs/amd-ucode",
						},
					},
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
