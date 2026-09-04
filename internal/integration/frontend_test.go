// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func testFrontend(ctx context.Context, baseURL string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		t.Run("Pipeline", testFrontendPipeline(ctx, baseURL))

		t.Run("Server Header", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL+"/", nil)
			require.NoError(t, err)

			addTestAuth(req)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, resp.Body.Close())
			})

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			server := resp.Header.Get("Server")

			if enterprise.Enabled() {
				assert.Contains(t, server, "Image Factory Enterprise")
			} else {
				assert.Contains(t, server, "Image Factory")
				assert.NotContains(t, server, "Enterprise")
			}
		})

		t.Run("Public HTTP behavior", func(t *testing.T) {
			t.Parallel()

			for _, test := range []struct {
				name         string
				method       string
				path         string
				status       int
				contentType  string
				bodyContains string
			}{
				{name: "OpenAPI", method: http.MethodGet, path: "/openapi.yaml", status: http.StatusOK, contentType: "application/yaml", bodyContains: "openapi: 3.1.0"},
				{name: "nested static asset", method: http.MethodGet, path: "/css/output.css", status: http.StatusOK, contentType: "text/css"},
				{name: "JavaScript asset", method: http.MethodGet, path: "/js/clipboard.js", status: http.StatusOK},
				{name: "favicon asset", method: http.MethodGet, path: "/favicons/favicon.ico", status: http.StatusOK},
				{name: "Apple touch icon", method: http.MethodGet, path: "/favicons/apple-touch-icon.png", status: http.StatusOK},
				{name: "missing static asset", method: http.MethodGet, path: "/js/missing.js", status: http.StatusNotFound},
				{name: "unknown route", method: http.MethodGet, path: "/future-route", status: http.StatusNotFound},
				{name: "unknown OCI route", method: http.MethodGet, path: "/v2/example/unknown", status: http.StatusNotFound},
				{name: "unsupported method", method: http.MethodDelete, path: "/image/example/v1.11.0/metal-amd64.raw", status: http.StatusMethodNotAllowed},
			} {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()

					req, err := http.NewRequestWithContext(ctx, test.method, baseURL+test.path, nil)
					require.NoError(t, err)

					addTestAuth(req)

					resp, err := http.DefaultClient.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close() //nolint:errcheck

					assert.Equal(t, test.status, resp.StatusCode)

					if test.contentType != "" {
						assert.Contains(t, resp.Header.Get("Content-Type"), test.contentType)
					}

					if test.bodyContains != "" {
						body, readErr := io.ReadAll(resp.Body)
						require.NoError(t, readErr)
						assert.Contains(t, string(body), test.bodyContains)
					}
				})
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

			// POST /schematics requires auth; verify correct credentials allow access.
			c, err := client.New(baseURL, clientAuthCredentials()...)
			require.NoError(t, err)

			_, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
			require.NoError(t, err)
		})

		t.Run("Incorrect Credentials", func(t *testing.T) {
			t.Parallel()

			username, password := authCredentials()
			password += "x"

			// /versions is now public; use SchematicCreate (POST /schematics) which requires auth.
			c, err := client.New(baseURL, client.WithBasicAuth(username, password))
			require.NoError(t, err)

			_, _, err = c.SchematicCreate(ctx, schematicpkg.Schematic{})
			require.Error(t, err)
			require.ErrorContains(t, err, "HTTP 401: authentication required")
		})
	}
}
