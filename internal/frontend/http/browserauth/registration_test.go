// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package browserauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestProviderCallbackRegistersWithRequestValidation(t *testing.T) {
	t.Parallel()

	for _, callbackPath := range []string{"/callback", "/oauth/callback"} {
		t.Run(callbackPath, func(t *testing.T) {
			t.Parallel()

			contract, err := api.NewContract(t.Context(), api.WithBrowserCallbackPath(callbackPath))
			require.NoError(t, err)

			provider := &callbackProvider{browserLoginProvider: browserLoginProvider{enabled: true, callbackPath: callbackPath}}
			middleware := httpfrontend.NewRequestMiddleware(zap.NewNop(), contract, provider, nil, nil)
			pipeline, err := transport.NewPipeline(middleware.Build, middleware.Build, middleware.Build)
			require.NoError(t, err)

			registrar, err := transport.NewRegistrar(contract, pipeline)
			require.NoError(t, err)
			require.NoError(t, registrar.Register(browserauth.New(provider).Routes()))

			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, callbackPath+"?code=test-code&state=test-state", nil)
			registrar.Handler().ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, callbackPath, provider.receivedPath)
			require.Equal(t, "test-code", provider.receivedCode)
			require.Equal(t, "code=test-code&state=test-state", provider.receivedQuery)

			// The callback schema intentionally permits provider-specific query data.
			// Add a test-only constraint to the matched operation to prove that the
			// real middleware validates this path rather than exempting callbacks.
			route, _, err := contract.Router.FindRoute(request)
			require.NoError(t, err)

			route.Operation.Parameters = append(route.Operation.Parameters, &openapi3.ParameterRef{
				Value: openapi3.NewQueryParameter("state").WithRequired(true).WithSchema(openapi3.NewStringSchema()),
			})
			provider.receivedPath = ""
			response = httptest.NewRecorder()
			registrar.Handler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, callbackPath+"?code=test-code", nil))
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Empty(t, provider.receivedPath)

			response = httptest.NewRecorder()
			registrar.Handler().ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, callbackPath, provider.receivedPath)
		})
	}
}

type callbackProvider struct {
	receivedPath  string
	receivedCode  string
	receivedQuery string

	browserLoginProvider
}

func (provider *callbackProvider) CallbackHandler() enterprise.Handler {
	return func(_ context.Context, writer http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
		provider.receivedPath = request.URL.Path
		provider.receivedCode = request.URL.Query().Get("code")
		provider.receivedQuery = request.URL.RawQuery

		writer.WriteHeader(http.StatusNoContent)

		return nil
	}
}
