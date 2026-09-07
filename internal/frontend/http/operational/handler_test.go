// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operational_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/operational"
)

func TestHandlerHealth(t *testing.T) {
	t.Parallel()

	handler := operational.New()
	response := httptest.NewRecorder()

	err := handler.Health(t.Context(), response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil), nil)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.Code)
	require.Empty(t, response.Body.String())
}

func TestHandlerReadiness(t *testing.T) {
	t.Parallel()

	t.Run("ready", func(t *testing.T) {
		t.Parallel()

		checker := &readinessChecker{}
		handler := operational.New(checker)
		response := httptest.NewRecorder()

		err := handler.Ready(t.Context(), response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil), nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.Code)
		require.Empty(t, response.Body.String())
		require.Equal(t, 1, checker.calls)
	})

	t.Run("not ready", func(t *testing.T) {
		t.Parallel()

		checker := &readinessChecker{err: errors.New("warming up")}
		handler := operational.New(checker)
		response := httptest.NewRecorder()

		err := handler.Ready(t.Context(), response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil), nil)

		require.NoError(t, err)
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.Equal(t, "not ready\n", response.Body.String())
		require.Equal(t, 1, checker.calls)
	})
}

func TestHandlerPreservesHeadSemanticsThroughNetHTTP(t *testing.T) {
	t.Parallel()

	handler := operational.New(&readinessChecker{err: errors.New("warming up")})
	router := httprouter.New()
	router.HEAD("/healthz", adapt(t, handler.Health))
	router.HEAD("/readyz", adapt(t, handler.Ready))

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	for _, test := range []struct {
		path   string
		status int
	}{
		{path: "/healthz", status: http.StatusOK},
		{path: "/readyz", status: http.StatusServiceUnavailable},
	} {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodHead, server.URL+test.path, nil)
		require.NoError(t, err)

		response, err := server.Client().Do(request)
		require.NoError(t, err)

		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		require.NoError(t, response.Body.Close())
		require.Equal(t, test.status, response.StatusCode)
		require.Empty(t, body)
	}
}

type readinessChecker struct {
	err   error
	calls int
}

func (checker *readinessChecker) Ready() error {
	checker.calls++

	return checker.err
}

func adapt(t *testing.T, endpoint func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error) httprouter.Handle {
	t.Helper()

	return func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		require.NoError(t, endpoint(request.Context(), writer, request, params))
	}
}
