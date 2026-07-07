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

func TestHandleLLMsTxt(t *testing.T) {
	frontend := frontendhttp.NewTestFrontend(zap.NewNop())

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/llms.txt", nil)

	frontend.WrapHandler(frontend.HandleLLMsTxt())(w, r, nil)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
	assert.Contains(t, w.Body.String(), "factory.talos.dev")
}
