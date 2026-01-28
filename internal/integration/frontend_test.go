// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFrontend(ctx context.Context, baseURL string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		t.Run("Server Header", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL+"/", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, resp.Body.Close())
			})

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			server := resp.Header.Get("Server")

			if enterprise.Enabled() {
				assert.Contains(t, server, "Enterprise Image Factory")
			} else {
				assert.Contains(t, server, "Image Factory")
				assert.NotContains(t, server, "Enterprise")
			}
		})

		t.Run("Auth", testFrontendAuth(ctx, baseURL))
	}
}

func testFrontendAuth(ctx context.Context, baseURL string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		if !enterprise.Enabled() {
			t.Skip("enterprise features are disabled")
		}

		t.Run("Correct Credentials", func(t *testing.T) {
			t.Parallel()

			client, err := client.New(baseURL, clientAuthCredentials()...)
			require.NoError(t, err)

			_, err = client.Versions(ctx)
			require.NoError(t, err)
		})

		t.Run("Incorrect Credentials", func(t *testing.T) {
			t.Parallel()

			username, password := authCredentials()
			password += "x"

			client, err := client.New(baseURL, client.WithBasicAuth(username, password))
			require.NoError(t, err)

			_, err = client.Versions(ctx)
			require.Error(t, err)
			require.ErrorContains(t, err, "HTTP 401: authentication required")
		})
	}
}
