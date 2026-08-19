// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	frontendhttp "github.com/siderolabs/image-factory/internal/frontend/http"
)

func TestHandleOpenAPI(t *testing.T) {
	t.Parallel()

	frontend := frontendhttp.NewTestFrontend(zap.NewNop())

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.yaml", nil)

	frontend.WrapHandler(frontend.HandleOpenAPI())(response, request, nil)

	result := response.Result()
	require.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, "application/yaml", result.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", result.Header.Get("Cache-Control"))
	assert.Contains(t, response.Body.String(), "openapi: 3.1.0")
	assert.Contains(t, response.Body.String(), "jsonSchemaDialect: https://spec.openapis.org/oas/3.1/dialect/2024-11-10")
	assert.Contains(t, response.Body.String(), "operationId: listVersions")
}
