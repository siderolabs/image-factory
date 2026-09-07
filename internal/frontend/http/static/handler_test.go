// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package static_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	staticfiles "github.com/siderolabs/image-factory/internal/frontend/http/static"
)

func TestHandlerPreservesFileServerBehavior(t *testing.T) {
	t.Parallel()

	modified := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	filesystem := http.FS(fstest.MapFS{
		"asset.txt":         {Data: []byte("0123456789"), ModTime: modified},
		"nested/index.html": {Data: []byte("nested index"), ModTime: modified},
	})

	handler := staticfiles.New(filesystem)
	router := httprouter.New()
	router.GET("/static/*filepath", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		require.NoError(t, handler.Serve(request.Context(), writer, request, params))
	})

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	client := server.Client()

	t.Run("nested index", func(t *testing.T) {
		t.Parallel()

		response := get(t, client, server.URL+"/static/nested/")
		defer func() {
			require.NoError(t, response.Body.Close())
		}()

		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "nested index", string(body))
	})

	t.Run("directory redirect", func(t *testing.T) {
		t.Parallel()

		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/static/nested", nil)
		require.NoError(t, err)

		redirectClient := *client
		redirectClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

		response, err := redirectClient.Do(request)
		require.NoError(t, err)

		defer func() {
			require.NoError(t, response.Body.Close())
		}()

		require.Equal(t, http.StatusMovedPermanently, response.StatusCode)
		require.Equal(t, "nested/", response.Header.Get("Location"))
	})

	t.Run("range request", func(t *testing.T) {
		t.Parallel()

		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/static/asset.txt", nil)
		require.NoError(t, err)
		request.Header.Set("Range", "bytes=2-5")

		response, err := client.Do(request)
		require.NoError(t, err)

		defer func() {
			require.NoError(t, response.Body.Close())
		}()

		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusPartialContent, response.StatusCode)
		require.Equal(t, "2345", string(body))
		require.Equal(t, "bytes 2-5/10", response.Header.Get("Content-Range"))
	})

	t.Run("conditional request", func(t *testing.T) {
		t.Parallel()

		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/static/asset.txt", nil)
		require.NoError(t, err)
		request.Header.Set("If-Modified-Since", modified.Format(http.TimeFormat))

		response, err := client.Do(request)
		require.NoError(t, err)

		defer func() {
			require.NoError(t, response.Body.Close())
		}()

		require.Equal(t, http.StatusNotModified, response.StatusCode)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		response := get(t, client, server.URL+"/static/missing.txt")
		defer func() {
			require.NoError(t, response.Body.Close())
		}()

		require.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func get(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)

	response, err := client.Do(request)
	require.NoError(t, err)

	return response
}
