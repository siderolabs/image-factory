// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestFrontendConfiguredBrowserCallback(t *testing.T) {
	for _, callbackPath := range []string{"/callback", "/oauth/callback", "/oauth/callback/"} {
		t.Run(callbackPath, func(t *testing.T) {
			provider := &configuredCallbackProvider{path: callbackPath}
			frontend := newCatalogFrontend(t, provider)
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, callbackPath+"?code=original%2Bcode&state=original-state", nil)
			frontend.Handler().ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, callbackPath, provider.receivedPath)
			require.Equal(t, "code=original%2Bcode&state=original-state", provider.receivedQuery)
			require.Zero(t, provider.authenticationCalls)

			response = httptest.NewRecorder()
			frontend.Handler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/schematics/example", nil))
			require.Equal(t, http.StatusUnauthorized, response.Code)
			require.Equal(t, 1, provider.authenticationCalls)

			response = httptest.NewRecorder()
			frontend.Handler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, callbackPath, nil))
			require.Equal(t, http.StatusMethodNotAllowed, response.Code)
		})
	}
}

func TestFrontendRejectsInvalidBrowserCallback(t *testing.T) {
	for _, callbackPath := range []string{"", "/versions", "/oauth/:callback", "/css/callback"} {
		t.Run(callbackPath, func(t *testing.T) {
			frontend, err := httpfrontend.NewFrontend(t.Context(), zap.NewNop(), nil, nil, nil, nil, nil, nil, nil, httpfrontend.Options{
				AuthProvider: &configuredCallbackProvider{path: callbackPath},
			})
			require.ErrorContains(t, err, "browser callback path")
			require.Nil(t, frontend)
		})
	}
}

type configuredCallbackProvider struct {
	browserLoginProvider

	path                string
	receivedPath        string
	receivedQuery       string
	authenticationCalls int
}

func (provider *configuredCallbackProvider) CallbackPath() string { return provider.path }

func (provider *configuredCallbackProvider) CallbackHandler() enterprise.Handler {
	return func(_ context.Context, writer http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
		provider.receivedPath = request.URL.Path
		provider.receivedQuery = request.URL.RawQuery

		writer.WriteHeader(http.StatusNoContent)

		return nil
	}
}

func (provider *configuredCallbackProvider) Middleware(enterprise.Handler) enterprise.Handler {
	return func(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
		provider.authenticationCalls++

		writer.WriteHeader(http.StatusUnauthorized)

		return nil
	}
}
