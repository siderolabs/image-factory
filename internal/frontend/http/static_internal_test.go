// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
)

func TestServeFiles(t *testing.T) {
	t.Parallel()

	filesystem := http.FS(fstest.MapFS{
		"styles/main.css": &fstest.MapFile{Data: []byte("body {}")},
	})
	handler := serveFiles(filesystem)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/css/styles/main.css", nil)
	response := httptest.NewRecorder()

	handler(response, request, httprouter.Params{{Key: "filepath", Value: "/styles/main.css"}})

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "body {}", response.Body.String())
}

func TestServeFilesReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := serveFiles(http.FS(fstest.MapFS{}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/js/missing.js", nil)
	response := httptest.NewRecorder()

	handler(response, request, httprouter.Params{{Key: "filepath", Value: "/missing.js"}})

	require.Equal(t, http.StatusNotFound, response.Code)
}
