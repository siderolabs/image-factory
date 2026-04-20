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

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// nonexistentSchematicID is a fixed ID that will never be created.
const nonexistentSchematicID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// Schematic IDs are computed at init time because in enterprise builds the owner
// is embedded in the schematic YAML before hashing, changing the resulting ID.
var (
	emptySchematicID                    string
	extraArgsSchematicID                string
	systemExtensionsSchematicID         string
	metaSchematicID                     string
	rpiGenericOverlaySchematicID        string
	securebootWellKnownSchematicID      string
	grubBootloaderOverrideSchematicID   string
	sdBootBootloaderOverrideSchematicID string

	testSchematics map[string]*schematic.Schematic
)

func mustSchematicID(s *schematic.Schematic) string {
	id, err := s.ID()
	if err != nil {
		panic("computing schematic ID: " + err.Error())
	}

	return id
}

func init() {
	// In enterprise builds, schematics created via the authenticated API include the
	// owner in their YAML, which changes the SHA-256 ID. Pre-set the owner here so
	// the IDs computed in tests match what the server will return.
	owner, _ := authCredentials()

	raw := []*schematic.Schematic{
		{Owner: owner},
		{Owner: owner, Customization: schematic.Customization{ExtraKernelArgs: []string{"nolapic", "nomodeset"}}},
		{Owner: owner, Customization: schematic.Customization{SystemExtensions: schematic.SystemExtensions{OfficialExtensions: []string{"siderolabs/amd-ucode", "siderolabs/gvisor", "siderolabs/gasket-driver"}}}},
		{Owner: owner, Customization: schematic.Customization{Meta: []schematic.MetaValue{{Key: 0xa, Value: `{"externalIPs":["1.2.3.4"]}`}}}},
		{Owner: owner, Overlay: schematic.Overlay{Name: "rpi_generic", Image: "siderolabs/sbc-raspberrypi"}},
		{Owner: owner, Customization: schematic.Customization{SecureBoot: schematic.SecureBootCustomization{IncludeWellKnownCertificates: true}}},
		{Owner: owner, Customization: schematic.Customization{Bootloader: profile.BootLoaderKindGrub}},
		{Owner: owner, Customization: schematic.Customization{Bootloader: profile.BootLoaderKindSDBoot}},
	}

	testSchematics = make(map[string]*schematic.Schematic, len(raw))

	emptySchematicID = mustSchematicID(raw[0])
	extraArgsSchematicID = mustSchematicID(raw[1])
	systemExtensionsSchematicID = mustSchematicID(raw[2])
	metaSchematicID = mustSchematicID(raw[3])
	rpiGenericOverlaySchematicID = mustSchematicID(raw[4])
	securebootWellKnownSchematicID = mustSchematicID(raw[5])
	grubBootloaderOverrideSchematicID = mustSchematicID(raw[6])
	sdBootBootloaderOverrideSchematicID = mustSchematicID(raw[7])

	testSchematics[emptySchematicID] = raw[0]
	testSchematics[extraArgsSchematicID] = raw[1]
	testSchematics[systemExtensionsSchematicID] = raw[2]
	testSchematics[metaSchematicID] = raw[3]
	testSchematics[rpiGenericOverlaySchematicID] = raw[4]
	testSchematics[securebootWellKnownSchematicID] = raw[5]
	testSchematics[grubBootloaderOverrideSchematicID] = raw[6]
	testSchematics[sdBootBootloaderOverrideSchematicID] = raw[7]
}

func createSchematicGetID(ctx context.Context, t *testing.T, c *client.Client, schematic schematic.Schematic) string {
	t.Helper()

	id, err := c.SchematicCreate(ctx, schematic)
	require.NoError(t, err)

	// get the schematic back and compare
	retrieved, err := c.SchematicGet(ctx, id)
	require.NoError(t, err)
	schematic.Owner, _ = authCredentials()
	assert.Equal(t, &schematic, retrieved)

	return id
}

// not using the client here as we need to submit invalid yaml.
func createSchematicInvalid(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) string {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/schematics", bytes.NewReader(marshalled))
	require.NoError(t, err)

	addTestAuth(req)

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
	c, err := client.New(baseURL, clientAuthCredentials()...)
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

	t.Run("bootloader_override_grub", func(t *testing.T) {
		assert.Equal(t, grubBootloaderOverrideSchematicID, createSchematicGetID(ctx, t, c, *testSchematics[grubBootloaderOverrideSchematicID]))
	})

	t.Run("bootloader_override_sd-boot", func(t *testing.T) {
		assert.Equal(t, sdBootBootloaderOverrideSchematicID, createSchematicGetID(ctx, t, c, *testSchematics[sdBootBootloaderOverrideSchematicID]))
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
		assert.Equal(t, "yaml: construct errors:\n  line 1: field something not found in type schematic.Schematic\n", createSchematicInvalid(ctx, t, baseURL, []byte(`something:`)))
	})

	t.Run("nonexistent get", func(t *testing.T) {
		_, err := c.SchematicGet(ctx, nonexistentSchematicID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schematic not found")
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
