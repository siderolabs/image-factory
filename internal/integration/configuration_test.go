// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func createConfiguration(ctx context.Context, t *testing.T, baseURL string, config configuration.Configuration) string {
	t.Helper()

	marshalled, err := config.Marshal()
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/configuration", bytes.NewReader(marshalled))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var respBody struct {
		ID string `json:"id"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))

	return respBody.ID
}

func testConfiguration(ctx context.Context, t *testing.T, baseURL string) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, emptyConfigurationID, createConfiguration(ctx, t, baseURL, configuration.Configuration{}))
	})

	t.Run("kernel args", func(t *testing.T) {
		assert.Equal(t, extraArgsConfigurationID, createConfiguration(ctx, t, baseURL,
			configuration.Configuration{
				Customization: configuration.Customization{
					ExtraKernelArgs: []string{"nolapic", "nomodeset"},
				},
			},
		))
	})

	t.Run("empty repeat", func(t *testing.T) {
		assert.Equal(t, emptyConfigurationID, createConfiguration(ctx, t, baseURL, configuration.Configuration{}))
	})
}
