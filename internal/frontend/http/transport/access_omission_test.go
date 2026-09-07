// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestRouteRejectsOmittedAccess(t *testing.T) {
	t.Parallel()

	route := transport.Route{
		Method:      http.MethodGet,
		Path:        "/versions",
		OperationID: "listVersions",
		Handler:     func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error { return nil },
	}

	require.ErrorContains(t, route.Validate(), "unsupported access policy")
}

func TestRegistrarRejectsOmittedAccessAtomically(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	builds := 0
	builder := func(route transport.Route) (httprouter.Handle, error) {
		builds++

		return directBuilder(route)
	}
	pipeline, err := transport.NewPipeline(builder, builder, builder)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	handler := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error { return nil }
	require.NoError(t, registrar.Register([]transport.Route{{
		Method:      http.MethodGet,
		Path:        "/healthz",
		OperationID: "getHealth",
		Access:      transport.AccessPublic,
		Protocol:    transport.ProtocolOperational,
		Handler:     handler,
	}}))

	builds = 0

	require.ErrorContains(t, registrar.Register([]transport.Route{
		{Method: http.MethodGet, Path: "/versions", OperationID: "listVersions", Access: transport.AccessPublic, Handler: handler},
		{Method: http.MethodGet, Path: "/openapi.yaml", OperationID: "getOpenAPI", Handler: handler},
	}), "unsupported access policy")
	require.Zero(t, builds)

	for _, test := range []struct {
		path   string
		status int
	}{{"/healthz", http.StatusOK}, {"/versions", http.StatusNotFound}, {"/openapi.yaml", http.StatusNotFound}} {
		response := httptest.NewRecorder()
		registrar.Handler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))
		require.Equal(t, test.status, response.Code, test.path)
	}
}
