// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	factoryapi "github.com/siderolabs/image-factory/api"
	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestServerBuildsContractBackedCORSHandler(t *testing.T) {
	t.Parallel()

	contract, err := factoryapi.NewContract(t.Context())
	require.NoError(t, err)

	server, err := httpfrontend.NewServer(
		contract,
		[]transport.Route{{
			Method:      http.MethodGet,
			Path:        "/healthz",
			OperationID: "getHealth",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolOperational,
			Handler: func(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
				writer.WriteHeader(http.StatusNoContent)

				return nil
			},
		}},
		func(route transport.Route) (httprouter.Handle, error) {
			return func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
				require.NoError(t, route.Handler(request.Context(), writer, request, params))
			}, nil
		},
		httpfrontend.ServerOptions{
			AllowedOrigins:   []string{"https://example.com"},
			MetricsNamespace: "server_test",
		},
	)
	require.NoError(t, err)

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)

	response = httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/healthz", nil)
	request.Header.Set("Origin", "https://example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	server.Handler().ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "https://example.com", response.Header().Get("Access-Control-Allow-Origin"))
}

func TestServerRequiresApplicationBuilder(t *testing.T) {
	t.Parallel()

	contract, err := factoryapi.NewContract(t.Context())
	require.NoError(t, err)

	server, err := httpfrontend.NewServer(contract, nil, nil, httpfrontend.ServerOptions{})
	require.EqualError(t, err, "application handler builder is required")
	require.Nil(t, server)
}

func TestServerRejectsNilBuiltHandler(t *testing.T) {
	t.Parallel()

	contract, err := factoryapi.NewContract(t.Context())
	require.NoError(t, err)

	server, err := httpfrontend.NewServer(contract, []transport.Route{{
		Method: http.MethodGet, Path: "/healthz", OperationID: "getHealth",
		Access: transport.AccessPublic, Protocol: transport.ProtocolOperational,
		Handler: func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error { return nil },
	}}, func(transport.Route) (httprouter.Handle, error) {
		return nil, nil //nolint:nilnil // Exercise the invalid builder result rejected by NewServer.
	}, httpfrontend.ServerOptions{MetricsNamespace: "server_nil_handler_test"})
	require.ErrorContains(t, err, "handler is required")
	require.Nil(t, server)
}

func TestServerRetainsOnlyFinalHandler(t *testing.T) {
	t.Parallel()

	typeOfServer := reflect.TypeFor[httpfrontend.Server]()
	require.Equal(t, 1, typeOfServer.NumField())
	require.Equal(t, "handler", typeOfServer.Field(0).Name)
}
