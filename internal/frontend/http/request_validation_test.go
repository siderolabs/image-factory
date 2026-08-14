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

func TestWrapperValidatesDocumentedRequests(t *testing.T) {
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

	assert.False(t, called)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), `property "unknown" is unsupported`)
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

func TestWrapperLeavesUndocumentedFrontendRoutesUntouched(t *testing.T) {
	t.Parallel()

	frontend := factoryhttp.NewTestFrontendWithContract(zaptest.NewLogger(t))
	called := false
	handler := frontend.WrapHandler(func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		called = true

		return nil
	})

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/ui/wizard", strings.NewReader(`not an API body`))
	response := httptest.NewRecorder()

	handler(response, request, nil)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, response.Code)
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
