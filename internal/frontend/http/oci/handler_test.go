// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oci_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestHandlerPublishesRegistryRoutes(t *testing.T) {
	t.Parallel()

	routes := oci.New(nil).Routes()

	require.Equal(t, []routeIdentity{
		{method: http.MethodGet, path: "/v2", operationID: "checkRegistry"},
		{method: http.MethodHead, path: "/v2", operationID: "headRegistry"},
		{method: http.MethodGet, path: "/v2/*path"},
		{method: http.MethodHead, path: "/v2/*path"},
	}, routeIdentities(routes))

	for _, route := range routes {
		require.Equal(t, transport.AccessImageDownload, route.Access)
		require.Equal(t, transport.ProtocolOCI, route.Protocol)
	}

	require.Equal(t, []string{
		"checkRegistrySlash",
		"getRegistryManifest",
		"getRegistryBlob",
		"listRegistryTags",
		"getRegistryReferrers",
	}, routes[2].DispatchedOperationIDs)
	require.Equal(t, []string{
		"headRegistrySlash",
		"headRegistryManifest",
		"headRegistryBlob",
		"headRegistryTags",
		"headRegistryReferrers",
	}, routes[3].DispatchedOperationIDs)
}

func TestHandlerDispatchesParsedRegistryRoute(t *testing.T) {
	t.Parallel()

	var got oci.V2Route

	handler := oci.New(func(_ context.Context, _ http.ResponseWriter, _ *http.Request, route oci.V2Route) error {
		got = route

		return nil
	})

	route := requireRoute(t, handler.Routes(), http.MethodGet)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/installer/abc/manifests/v1.14.0", nil)
	recorder := httptest.NewRecorder()

	err := route.Handler(t.Context(), recorder, request, httprouter.Params{{Key: "path", Value: "/installer/abc/manifests/v1.14.0"}})
	require.NoError(t, err)
	require.Equal(t, oci.V2Route{
		Image:     "installer",
		Schematic: "abc",
		Resource:  "manifests",
		Reference: "v1.14.0",
		Target:    oci.V2TargetManifest,
	}, got)
}

func TestHandlerHandlesRegistrySlashPingWithoutDispatch(t *testing.T) {
	t.Parallel()

	dispatched := false
	handler := oci.New(func(context.Context, http.ResponseWriter, *http.Request, oci.V2Route) error {
		dispatched = true

		return nil
	})

	route := requireRoute(t, handler.Routes(), http.MethodHead)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/v2/", nil)
	recorder := httptest.NewRecorder()

	err := route.Handler(t.Context(), recorder, request, httprouter.Params{{Key: "path", Value: "/"}})
	require.NoError(t, err)
	require.False(t, dispatched)
	require.Empty(t, recorder.Body.String())
}

type routeIdentity struct {
	method      string
	path        string
	operationID string
}

func routeIdentities(routes []transport.Route) []routeIdentity {
	result := make([]routeIdentity, 0, len(routes))

	for _, route := range routes {
		result = append(result, routeIdentity{method: route.Method, path: route.Path, operationID: route.OperationID})
	}

	return result
}

func requireRoute(t *testing.T, routes []transport.Route, method string) transport.Route {
	t.Helper()

	const path = "/v2/*path"

	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return route
		}
	}

	require.FailNow(t, "route not found", "%s %s", method, path)

	return transport.Route{}
}
