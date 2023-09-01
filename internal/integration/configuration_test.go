// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-service/pkg/configuration"
)

// well known configuration IDs, they will be created with the test run
const (
	emptyConfigurationID     = "f655a707a7522c4bb0bd753833b288aa426c487ad44dab7cbb08d911a1ab33e5"
	extraArgsConfigurationID = "69cbf7e068a698c5d3c72fbea15817bd1c8f2a6d9fc1bee5cf1e3a00bbe1f326"
)

func createConfiguration(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/configuration", bytes.NewReader(marshalled))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	return resp
}

func createConfigurationGetID(ctx context.Context, t *testing.T, baseURL string, config configuration.Configuration) string {
	t.Helper()

	marshalled, err := config.Marshal()
	require.NoError(t, err)

	resp := createConfiguration(ctx, t, baseURL, marshalled)

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var respBody struct {
		ID string `json:"id"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))

	return respBody.ID
}

func createConfigurationInvalid(ctx context.Context, t *testing.T, baseURL string, marshalled []byte) string {
	t.Helper()

	resp := createConfiguration(ctx, t, baseURL, marshalled)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func testConfiguration(ctx context.Context, t *testing.T, baseURL string) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, emptyConfigurationID, createConfigurationGetID(ctx, t, baseURL, configuration.Configuration{}))
	})

	t.Run("kernel args", func(t *testing.T) {
		assert.Equal(t, extraArgsConfigurationID, createConfigurationGetID(ctx, t, baseURL,
			configuration.Configuration{
				Customization: configuration.Customization{
					ExtraKernelArgs: []string{"nolapic", "nomodeset"},
				},
			},
		))
	})

	t.Run("empty repeat", func(t *testing.T) {
		assert.Equal(t, emptyConfigurationID, createConfigurationGetID(ctx, t, baseURL, configuration.Configuration{}))
	})

	t.Run("invalid", func(t *testing.T) {
		assert.Equal(t, "yaml: unmarshal errors:\n  line 1: field something not found in type configuration.Configuration\n", createConfigurationInvalid(ctx, t, baseURL, []byte(`something:`)))
	})
}
