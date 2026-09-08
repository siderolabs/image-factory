// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

func TestRegisteredRoutesUseProductionMiddlewareOrdering(t *testing.T) {
	provider := &rejectingAuthProvider{}

	router, err := httpfrontend.RegisterTestRoutesWithAuth(t.Context(), zap.NewNop(), provider)
	require.NoError(t, err)

	t.Run("public route skips authentication", func(t *testing.T) {
		response := serveGET(t, router, "/healthz")

		require.Equal(t, http.StatusOK, response.Code)
		require.NotEmpty(t, response.Header().Get(httpfrontend.RequestIDHeader))
		require.Zero(t, provider.calls)
	})

	t.Run("OpenAPI rejection occurs before authentication", func(t *testing.T) {
		response := serveGET(t, router, "/v2/not-a-registry-operation")

		require.Equal(t, http.StatusNotFound, response.Code)
		require.NotEmpty(t, response.Header().Get(httpfrontend.RequestIDHeader))
		require.Zero(t, provider.calls)
	})

	t.Run("OCI route authenticates and preserves registry challenge", func(t *testing.T) {
		response := serveGET(t, router, "/v2")

		require.Equal(t, http.StatusUnauthorized, response.Code)
		require.Equal(t, `Basic realm="Image Factory Enterprise", charset="UTF-8"`, response.Header().Get("WWW-Authenticate"))
		require.NotEmpty(t, response.Header().Get(httpfrontend.RequestIDHeader))
		require.Equal(t, 1, provider.calls)
	})

	t.Run("static route bypasses application middleware", func(t *testing.T) {
		response := serveGET(t, router, "/css/not-found.css")

		require.Equal(t, http.StatusNotFound, response.Code)
		require.Empty(t, response.Header().Get(httpfrontend.RequestIDHeader))
		require.Equal(t, 1, provider.calls)
	})
}

func serveGET(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	response := httptest.NewRecorder()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)

	router.ServeHTTP(response, request)

	return response
}

type rejectingAuthProvider struct {
	calls int
}

func (*rejectingAuthProvider) Run(context.Context) error { return nil }

func (provider *rejectingAuthProvider) Middleware(_ enterprise.Handler) enterprise.Handler {
	return func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		provider.calls++

		return xerrors.NewTagged[schematic.RequiresAuthenticationTag](errors.New("missing credentials"))
	}
}

func (*rejectingAuthProvider) UsernameFromContext(context.Context) (string, bool) { return "", false }

func (*rejectingAuthProvider) ContextWithUsername(ctx context.Context, _ string) context.Context {
	return ctx
}
