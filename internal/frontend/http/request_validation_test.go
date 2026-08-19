// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	factoryhttp "github.com/siderolabs/image-factory/internal/frontend/http"
)

func TestWrapperLeavesSchematicBodyValidationToHandler(t *testing.T) {
	t.Parallel()

	frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
	called := false
	handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		called = true

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/schematics", strings.NewReader(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	handler(response, request, nil)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestWrapperPreservesValidatedRequestBody(t *testing.T) {
	t.Parallel()

	frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))

	var body string

	handler := frontend.WrapHandler(func(_ context.Context, _ http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)

		body = string(data)

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/schematics", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	handler(response, request, nil)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, `{}`, body)
}

func TestWrapperRejectsUndocumentedFrontendRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/future-route", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/v2/example/unknown", status: http.StatusNotFound},
		{method: http.MethodDelete, path: "/image/example/v1.11.0/metal-amd64.raw", status: http.StatusMethodNotAllowed},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()

			frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
			called := false
			handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
				called = true

				return nil
			})

			request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			response := httptest.NewRecorder()

			handler(response, request, nil)

			assert.False(t, called)
			assert.Equal(t, test.status, response.Code)
		})
	}
}

func TestWrapperPreservesHeaderIndependentSchematicDecoding(t *testing.T) {
	t.Parallel()

	for _, contentType := range []string{
		"application/x-yaml",
		"text/yaml",
		"application/x-www-form-urlencoded",
		"application/vnd.example+yaml",
	} {
		t.Run(contentType, func(t *testing.T) {
			t.Parallel()

			frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
			called := false
			handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
				called = true

				return nil
			})

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/schematics", strings.NewReader(`{}`))
			request.Header.Set("Content-Type", contentType)

			response := httptest.NewRecorder()
			handler(response, request, nil)

			assert.True(t, called)
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, contentType, request.Header.Get("Content-Type"))
		})
	}
}

func TestWrapperLeavesPathValidationToHandlers(t *testing.T) {
	t.Parallel()

	frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
	called := false
	handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		called = true

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/schematics/not-a-digest", nil)
	response := httptest.NewRecorder()

	handler(response, request, nil)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestWrapperValidatesImageArtifactPath(t *testing.T) {
	t.Parallel()

	const prefix = "/image/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/v1.12.0/"

	for _, test := range []struct {
		name       string
		method     string
		path       string
		wantCalled bool
		wantStatus int
	}{
		{name: "supported GET", method: http.MethodGet, path: "metal-amd64.raw.xz", wantCalled: true, wantStatus: http.StatusOK},
		{name: "supported HEAD", method: http.MethodHead, path: "metal-amd64.raw.xz", wantCalled: true, wantStatus: http.StatusOK},
		{name: "checksum", method: http.MethodGet, path: "metal-amd64.iso.sha256", wantCalled: true, wantStatus: http.StatusOK},
		{name: "unsupported GET", method: http.MethodGet, path: "metal-amd64.zip", wantCalled: false, wantStatus: http.StatusBadRequest},
		{name: "unsupported HEAD", method: http.MethodHead, path: "metal-amd64.zip", wantCalled: false, wantStatus: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
			called := false
			handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
				called = true

				return nil
			})

			request := httptest.NewRequestWithContext(t.Context(), test.method, prefix+test.path, nil)
			response := httptest.NewRecorder()

			handler(response, request, nil)

			assert.Equal(t, test.wantCalled, called)
			assert.Equal(t, test.wantStatus, response.Code)
		})
	}
}

func TestWrapperPreservesUIFormBody(t *testing.T) {
	t.Parallel()

	frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))

	var body string

	handler := frontend.WrapHandler(func(_ context.Context, _ http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)

		body = string(data)

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/ui/wizard", strings.NewReader("version=v1.12.0"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response := httptest.NewRecorder()

	handler(response, request, nil)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "version=v1.12.0", body)
}

func TestWrapperLeavesSchematicBodyValidationToHandlerWithArbitraryContentType(t *testing.T) {
	t.Parallel()

	frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
	called := false
	handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		called = true

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/schematics", strings.NewReader("unknown: true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response := httptest.NewRecorder()
	handler(response, request, nil)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/x-www-form-urlencoded", request.Header.Get("Content-Type"))
}
