// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package spa_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/fstest"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/spa"
)

const testIndex = `<html lang="en"><title>Image Factory{{ if .Config.Enterprise }} Enterprise{{ end }}</title>` +
	`<script id="app-config" type="application/json">{{ .Config }}</script></html>`

func indexData(*http.Request) spa.IndexData {
	return spa.IndexData{Config: spa.Config{Version: "v1.8.0</script>", Enterprise: true}}
}

func newRouter(t *testing.T, opts spa.Options) http.Handler {
	t.Helper()

	handler, err := spa.New(opts)
	require.NoError(t, err)

	router := httprouter.New()

	for _, route := range append(handler.Routes(), handler.AssetRoutes()...) {
		router.Handle(route.Method, route.Path, func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
			require.NoError(t, route.Handler(r.Context(), w, r, p))
		})
	}

	return handler.Middleware(router)
}

func get(t *testing.T, handler http.Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	maps.Copy(request.Header, header)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestIndex(t *testing.T) {
	t.Parallel()

	router := newRouter(t, spa.Options{
		Dist: fstest.MapFS{
			"index.html":           {Data: []byte(testIndex)},
			"assets/oindex-abc.js": {Data: []byte("console.log('hi')")},
		},
		IndexData: indexData,
	})

	for _, target := range []string{"/", "/?target=metal&version=1.13.0", "/ui/tokens"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			response := get(t, router, http.MethodGet, target, nil)

			require.Equal(t, http.StatusOK, response.Code)
			require.Equal(t, "text/html; charset=utf-8", response.Header().Get("Content-Type"))
			require.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
			require.Equal(t,
				`<html lang="en"><title>Image Factory Enterprise</title>`+
					`<script id="app-config" type="application/json">`+
					`{"version":"v1.8.0\u003c/script\u003e","enterprise":true,"tokensEnabled":false,"logoutEnabled":false,"authenticated":false}`+
					`</script></html>`,
				response.Body.String(),
			)
		})
	}

	t.Run("head", func(t *testing.T) {
		t.Parallel()

		response := get(t, router, http.MethodHead, "/", nil)

		require.Equal(t, http.StatusOK, response.Code)
		require.Empty(t, response.Body.String())
	})
}

func TestAssets(t *testing.T) {
	t.Parallel()

	router := newRouter(t, spa.Options{
		Dist: fstest.MapFS{
			"index.html":           {Data: []byte(testIndex)},
			"assets/oindex-abc.js": {Data: []byte("console.log('hi')")},
		},
		IndexData: indexData,
	})

	response := get(t, router, http.MethodGet, "/assets/oindex-abc.js", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "public, max-age=31536000, immutable", response.Header().Get("Cache-Control"))
	require.Equal(t, "text/javascript; charset=utf-8", response.Header().Get("Content-Type"))
	require.Equal(t, "console.log('hi')", response.Body.String())

	response = get(t, router, http.MethodGet, "/assets/oindex-abc.js", http.Header{"Range": {"bytes=0-6"}})

	require.Equal(t, http.StatusPartialContent, response.Code)
	require.Equal(t, "bytes 0-6/17", response.Header().Get("Content-Range"))
	require.Equal(t, "console", response.Body.String())

	for _, target := range []string{"/assets/", "/assets/missing.js"} {
		response = get(t, router, http.MethodGet, target, nil)

		require.Equal(t, http.StatusNotFound, response.Code, target)
		require.Empty(t, response.Header().Get("Cache-Control"), target)
	}
}

func TestFixedPathFiles(t *testing.T) {
	t.Parallel()

	router := newRouter(t, spa.Options{
		Dist: fstest.MapFS{
			"index.html":           {Data: []byte(testIndex)},
			"favicons/favicon.ico": {Data: []byte("icon")},
			"assets/oindex-abc.js": {Data: []byte("console.log('hi')")},
		},
		IndexData: indexData,
	})

	response := get(t, router, http.MethodGet, "/favicons/favicon.ico", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-cache", response.Header().Get("Cache-Control"), "the name stays the same when the file changes")
	require.Equal(t, "icon", response.Body.String())

	// The route serves only its own directory of the build.
	for _, target := range []string{"/favicons/", "/favicons/missing.png", "/favicons/../assets/oindex-abc.js"} {
		response := get(t, router, http.MethodGet, target, nil)

		require.Equal(t, http.StatusNotFound, response.Code, target)
	}
}

func TestMissingIndex(t *testing.T) {
	t.Parallel()

	_, err := spa.New(spa.Options{Dist: fstest.MapFS{}, IndexData: indexData})
	require.ErrorContains(t, err, "index.html")
}

func TestDevServer(t *testing.T) {
	t.Parallel()

	devServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><title>{{ .Config.Version }}</title><script type="module" src="/@vite/client"></script></html>`)) //nolint:errcheck
		default:
			w.Write([]byte("dev:" + r.URL.Path)) //nolint:errcheck
		}
	}))
	t.Cleanup(devServer.Close)

	devServerURL, err := url.Parse(devServer.URL)
	require.NoError(t, err)

	router := newRouter(t, spa.Options{DevServerURL: devServerURL, IndexData: indexData})

	response := get(t, router, http.MethodGet, "/ui/tokens", nil)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, `<html><title>v1.8.0&lt;/script&gt;</title><script type="module" src="/@vite/client"></script></html>`, response.Body.String())

	for _, target := range []string{"/@vite/client", "/src/main.ts", "/node_modules/.vite/deps/vue.js", "/favicons/favicon.ico"} {
		response = get(t, router, http.MethodGet, target, nil)

		require.Equal(t, http.StatusOK, response.Code, target)
		require.Equal(t, "dev:"+target, response.Body.String())
	}

	// The ping is answered by the dev server itself, so the template is left as is.
	response = get(t, router, http.MethodGet, "/", http.Header{"Accept": {"text/x-vite-ping"}})
	require.Contains(t, response.Body.String(), "{{ .Config.Version }}")
}
