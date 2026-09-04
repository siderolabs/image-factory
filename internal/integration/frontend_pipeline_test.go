// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFrontendPipeline(ctx context.Context, baseURL string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		client := &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		for _, test := range []struct {
			name     string
			method   string
			path     string
			status   int
			wrapped  bool
			allow    string
			location string
		}{
			{
				name:    "protected route",
				method:  http.MethodHead,
				path:    "/",
				status:  http.StatusOK,
				wrapped: true,
			},
			{
				name:    "public route",
				method:  http.MethodGet,
				path:    "/openapi.yaml",
				status:  http.StatusOK,
				wrapped: true,
			},
			{
				name:    "static route",
				method:  http.MethodGet,
				path:    "/css/output.css",
				status:  http.StatusOK,
				wrapped: false,
			},
			{
				name:    "static route has no implicit HEAD",
				method:  http.MethodHead,
				path:    "/css/output.css",
				status:  http.StatusMethodNotAllowed,
				wrapped: false,
				allow:   "GET, OPTIONS",
			},
			{
				name:    "router not found",
				method:  http.MethodGet,
				path:    "/not-a-route",
				status:  http.StatusNotFound,
				wrapped: false,
			},
			{
				name:    "router method not allowed",
				method:  http.MethodDelete,
				path:    "/openapi.yaml",
				status:  http.StatusMethodNotAllowed,
				wrapped: false,
				allow:   "GET, OPTIONS",
			},
			{
				name:     "trailing slash redirect",
				method:   http.MethodGet,
				path:     "/openapi.yaml/",
				status:   http.StatusMovedPermanently,
				wrapped:  false,
				location: "/openapi.yaml",
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequestWithContext(ctx, test.method, baseURL+test.path, nil)
				require.NoError(t, err)

				addTestAuth(req)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close() //nolint:errcheck

				assert.Equal(t, test.status, resp.StatusCode)
				assert.Equal(t, test.allow, resp.Header.Get("Allow"))
				assert.Equal(t, test.location, resp.Header.Get("Location"))

				if test.wrapped {
					assert.NotEmpty(t, resp.Header.Get("Server"))
					assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
				} else {
					assert.Empty(t, resp.Header.Get("Server"))
					assert.Empty(t, resp.Header.Get("X-Request-ID"))
				}

				if test.method == http.MethodHead {
					body, readErr := io.ReadAll(resp.Body)
					require.NoError(t, readErr)
					assert.Empty(t, body)
				}
			})
		}

		t.Run("static byte range", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/css/output.css", nil)
			require.NoError(t, err)

			req.Header.Set("Range", "bytes=0-9")

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
			assert.Regexp(t, `^bytes 0-9/[1-9][0-9]*$`, resp.Header.Get("Content-Range"))
			assert.Equal(t, "10", resp.Header.Get("Content-Length"))
			assert.Empty(t, resp.Header.Get("Server"))
			assert.Empty(t, resp.Header.Get("X-Request-ID"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Len(t, body, 10)
		})

		t.Run("OCI ping GET and HEAD", func(t *testing.T) {
			t.Parallel()

			for _, method := range []string{http.MethodGet, http.MethodHead} {
				t.Run(method, func(t *testing.T) {
					t.Parallel()

					req, err := http.NewRequestWithContext(ctx, method, baseURL+"/v2/", nil)
					require.NoError(t, err)

					addTestAuth(req)

					resp, err := client.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close() //nolint:errcheck

					assert.Equal(t, http.StatusOK, resp.StatusCode)
					assert.NotEmpty(t, resp.Header.Get("Server"))
					assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
					assert.Empty(t, resp.Header.Get("Docker-Distribution-API-Version"))

					body, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					assert.Empty(t, body)
				})
			}
		})

		t.Run("CORS GET preflight", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodOptions, baseURL+"/versions", nil)
			require.NoError(t, err)

			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", http.MethodGet)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
			assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
			assert.Equal(t, http.MethodGet, resp.Header.Get("Access-Control-Allow-Methods"))
			assert.Empty(t, resp.Header.Get("Server"))
			assert.Empty(t, resp.Header.Get("X-Request-ID"))
		})

		t.Run("CORS omits POST", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/schematics", bytes.NewBufferString("{"))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "https://example.com")
			addTestAuth(req)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		})

		t.Run("CORS rejects POST preflight", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodOptions, baseURL+"/schematics", nil)
			require.NoError(t, err)

			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Origin", "https://example.com")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
			assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
			assert.Empty(t, resp.Header.Get("Access-Control-Allow-Methods"))
		})

		t.Run("HTMX wizard start", func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/ui/wizard", nil)
			require.NoError(t, err)

			req.Header.Set("Hx-Request", "true")
			addTestAuth(req)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "/", resp.Header.Get("Hx-Push-Url"))
			assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), `name="target"`)
		})
	}
}
