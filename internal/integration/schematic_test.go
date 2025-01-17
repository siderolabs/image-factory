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
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skyssolutions/siderolabs-image-factory/pkg/client"
	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

// well known schematic IDs, they will be created with the test run
const (
	emptySchematicID               = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
	extraArgsSchematicID           = "e0fb1129bbbdfb5d002e94af4cdce712a8370e850950a33a242d4c3f178c532d"
	systemExtensionsSchematicID    = "51ff3e49313773332729a5c04e57af0dbe2e6d3f65ff638e6d4c3a05065fefff"
	metaSchematicID                = "fe866116408a5a13dab7d5003eb57a00954ea81ebeec3fbbcd1a6d4462a00036"
	rpiGenericOverlaySchematicID   = "ee21ef4a5ef808a9b7484cc0dda0f25075021691c8c09a276591eedb638ea1f9"
	securebootWellKnownSchematicID = "fa8e05f142a851d3ee568eb0a8e5841eaf6b0ebc8df9a63df16ac5ed2c04f3e6"
)

var testSchematics = map[string]*schematic.Schematic{
	emptySchematicID: {},
	extraArgsSchematicID: {
		Customization: schematic.Customization{
			ExtraKernelArgs: []string{"nolapic", "nomodeset"},
		},
	},
	systemExtensionsSchematicID: {
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
	metaSchematicID: {
		Customization: schematic.Customization{
			Meta: []schematic.MetaValue{
				{
					Key:   0xa,
					Value: `{"externalIPs":["1.2.3.4"]}`,
				},
			},
		},
	},
	rpiGenericOverlaySchematicID: {
		Overlay: schematic.Overlay{
			Name:  "rpi_generic",
			Image: "siderolabs/sbc-raspberrypi",
		},
	},
	securebootWellKnownSchematicID: {
		Customization: schematic.Customization{
			SecureBoot: schematic.SecureBootCustomization{
				IncludeWellKnownCertificates: true,
			},
		},
	},
}

func createSchematicGetID(ctx context.Context, t *testing.T, c *client.Client, schematic schematic.Schematic) string {
	t.Helper()

	id, err := c.SchematicCreate(ctx, schematic)
	require.NoError(t, err)

	return id
}

// not using the client here as we need to submit invalid yaml.
func createSchematicInvalid(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) string {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/schematics", bytes.NewReader(marshalled))
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

func testSchematic(ctx context.Context, t *testing.T, baseURL string) {
	c, err := client.New(baseURL)
	require.NoError(t, err)

	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, emptySchematicID, createSchematicGetID(ctx, t, c, *testSchematics[emptySchematicID]))
	})

	t.Run("kernel args", func(t *testing.T) {
		assert.Equal(t, extraArgsSchematicID, createSchematicGetID(ctx, t, c, *testSchematics[extraArgsSchematicID]))
	})

	t.Run("system extensions", func(t *testing.T) {
		assert.Equal(t, systemExtensionsSchematicID, createSchematicGetID(ctx, t, c, *testSchematics[systemExtensionsSchematicID]))
	})

	t.Run("meta", func(t *testing.T) {
		assert.Equal(t, metaSchematicID, createSchematicGetID(ctx, t, c, *testSchematics[metaSchematicID]))
	})

	t.Run("secureboot well-known certs", func(t *testing.T) {
		assert.Equal(t, securebootWellKnownSchematicID, createSchematicGetID(ctx, t, c, *testSchematics[securebootWellKnownSchematicID]))
	})

	t.Run("rpi generic overlay", func(t *testing.T) {
		assert.Equal(t, rpiGenericOverlaySchematicID, createSchematicGetID(ctx, t, c, *testSchematics[rpiGenericOverlaySchematicID]))
	})

	t.Run("empty once again", func(t *testing.T) {
		assert.Equal(t, emptySchematicID, createSchematicGetID(ctx, t, c, *testSchematics[emptySchematicID]))
	})

	t.Run("invalid", func(t *testing.T) {
		assert.Equal(t, "yaml: unmarshal errors:\n  line 1: field something not found in type schematic.Schematic\n", createSchematicInvalid(ctx, t, baseURL, []byte(`something:`)))
	})

	t.Run("new schematic", func(t *testing.T) {
		// create a new random schematic, as the schematic is persisted, and we want to test uploading new config
		randomKernelArg := hex.EncodeToString(randomBytes(t, 32))

		assert.Len(t, createSchematicGetID(ctx, t, c,
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
