// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestRegistrarValidatesAllRoutesBeforeMutation(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	buildCalls := 0
	pipeline, err := transport.NewPipeline(
		func(route transport.Route) (httprouter.Handle, error) {
			buildCalls++

			return testHandle(route), nil
		},
		func(route transport.Route) (httprouter.Handle, error) {
			buildCalls++

			return testHandle(route), nil
		},
		func(route transport.Route) (httprouter.Handle, error) {
			buildCalls++

			return testHandle(route), nil
		},
	)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	router := registrar.Handler()

	handler := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		return nil
	}

	err = registrar.Register([]transport.Route{
		{
			Method:      http.MethodGet,
			Path:        "/versions",
			OperationID: "listVersions",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     handler,
		},
		{
			Method:      http.MethodGet,
			Path:        "/openapi.yaml",
			OperationID: "notTheOpenAPIOperation",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     handler,
		},
	})
	require.ErrorContains(t, err, `declares operation "getOpenAPI", not "notTheOpenAPIOperation"`)
	require.Zero(t, buildCalls)

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/versions", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestRegistrarRejectsDuplicateRoutesBeforeMutation(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	pipeline, err := transport.NewPipeline(directBuilder, directBuilder, directBuilder)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	router := registrar.Handler()

	route := transport.Route{
		Method:      http.MethodGet,
		Path:        "/versions",
		OperationID: "listVersions",
		Access:      transport.AccessPublic,
		Protocol:    transport.ProtocolAPI,
		Handler: func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
			return nil
		},
	}

	err = registrar.Register([]transport.Route{route, route})
	require.ErrorContains(t, err, "duplicate runtime route GET /versions")

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/versions", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestRegistrarOwnsStableRouter(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	pipeline, err := transport.NewPipeline(directBuilder, directBuilder, directBuilder)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	router := registrar.Handler()
	require.NotNil(t, router)

	route := transport.Route{
		Method: http.MethodGet, Path: "/versions", OperationID: "listVersions",
		Access: transport.AccessPublic, Protocol: transport.ProtocolAPI,
		Handler: func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error { return nil },
	}

	require.NoError(t, registrar.Register([]transport.Route{route}))
	require.Same(t, router, registrar.Handler())

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/versions", nil))
	require.Equal(t, http.StatusOK, response.Code)
}

func TestRegistrarRegistersValidatedRoutes(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	pipeline, err := transport.NewPipeline(directBuilder, directBuilder, directBuilder)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	router := registrar.Handler()

	called := false
	err = registrar.Register([]transport.Route{
		{
			Method:      http.MethodGet,
			Path:        "/versions",
			OperationID: "listVersions",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler: func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
				called = true

				return nil
			},
		},
	})
	require.NoError(t, err)

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/versions", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.True(t, called)
}

func TestRegistrarRejectsRouteRegisteredByEarlierBatch(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	pipeline, err := transport.NewPipeline(directBuilder, directBuilder, directBuilder)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	route := transport.Route{
		Method:      http.MethodGet,
		Path:        "/versions",
		OperationID: "listVersions",
		Access:      transport.AccessPublic,
		Protocol:    transport.ProtocolAPI,
		Handler: func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
			return nil
		},
	}

	require.NoError(t, registrar.Register([]transport.Route{route}))
	require.ErrorContains(t, registrar.Register([]transport.Route{route}), "duplicate runtime route GET /versions")
}

func TestRegistrarBuildsEveryPipelineBeforeRouterMutation(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	build := func(route transport.Route) (httprouter.Handle, error) {
		if route.Path == "/openapi.yaml" {
			return nil, errors.New("pipeline unavailable")
		}

		return directBuilder(route)
	}
	pipeline, err := transport.NewPipeline(build, build, build)
	require.NoError(t, err)

	registrar, err := transport.NewRegistrar(contract, pipeline)
	require.NoError(t, err)

	router := registrar.Handler()

	handler := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		return nil
	}

	err = registrar.Register([]transport.Route{
		{Method: http.MethodGet, Path: "/versions", OperationID: "listVersions", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: handler},
		{Method: http.MethodGet, Path: "/openapi.yaml", OperationID: "getOpenAPI", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: handler},
	})
	require.ErrorContains(t, err, "pipeline unavailable")

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/versions", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
}

func directBuilder(route transport.Route) (httprouter.Handle, error) {
	return testHandle(route), nil
}

func testHandle(route transport.Route) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		if err := route.Handler(r.Context(), w, r, params); err != nil {
			panic(err)
		}
	}
}
