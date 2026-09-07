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

	"github.com/siderolabs/image-factory/api"
	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestFrontendRegistersCatalogThroughTransport(t *testing.T) {
	t.Parallel()

	router := httprouter.New()
	require.NoError(t, httpfrontend.RegisterTestRoutes(t.Context(), zap.NewNop(), router))

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotEmpty(t, response.Header().Get(httpfrontend.RequestIDHeader))

	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/not-registered", status: http.StatusNotFound},
		{method: http.MethodPost, path: "/healthz", status: http.StatusMethodNotAllowed},
	} {
		response = httptest.NewRecorder()
		request = httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
		router.ServeHTTP(response, request)

		require.Equal(t, test.status, response.Code)
	}
}

func TestCommunityRouteCatalogMatchesContract(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	frontend := httpfrontend.NewTestFrontend(zap.NewNop())
	routes := frontend.Routes()

	want := []string{
		"GET /healthz", "HEAD /healthz", "GET /readyz", "HEAD /readyz",
		"GET /image/:schematic/:version/:path", "HEAD /image/:schematic/:version/:path",
		"GET /pxe/:schematic/:version/:path",
		"GET /v2", "HEAD /v2", "GET /v2/*path", "HEAD /v2/*path",
		"GET /oci/cosign/signing-key.pub",
		"POST /schematics", "GET /schematics/:schematic",
		"GET /versions", "GET /version/:version/extensions/official", "GET /version/:version/overlays/official",
		"GET /secureboot/signing-cert.pem",
		"GET /talosctl/:version", "HEAD /talosctl/:version/:path", "GET /talosctl/:version/:path",
		"GET /llms.txt", "GET /openapi.yaml",
		"GET /", "HEAD /", "POST /ui/wizard", "GET /ui/version-doc", "POST /ui/extensions-list", "GET /ui/tokens",
		"GET /css/*filepath", "GET /favicons/*filepath", "GET /js/*filepath",
	}

	requireRouteInventory(t, contract, routes, want)
	requireGETRoute(t, routes, "/v2/*path", transport.AccessImageDownload, transport.ProtocolOCI)
	requireGETRoute(t, routes, "/css/*filepath", transport.AccessPublic, transport.ProtocolStatic)
	requireGETRoute(t, routes, "/", transport.AccessAuthenticated, transport.ProtocolHTML)
	requireGETRoute(t, routes, "/healthz", transport.AccessPublic, transport.ProtocolOperational)
}

func TestBrowserLoginRouteCatalogMatchesContract(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	frontend := httpfrontend.NewTestFrontendWithAuth(zap.NewNop(), browserLoginProvider{})
	routes := frontend.BrowserLoginRoutes()

	requireRouteInventory(t, contract, routes, []string{
		"GET /login",
		"GET /logout",
		"POST /logout",
		"GET /callback",
	})

	for _, route := range routes {
		require.Equal(t, transport.AccessPublic, route.Access)
		require.Equal(t, transport.ProtocolBrowserAuth, route.Protocol)
	}
}

func requireRouteInventory(t *testing.T, contract *api.Contract, routes []transport.Route, want []string) {
	t.Helper()

	got := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))

	for _, route := range routes {
		require.NoError(t, route.ValidateContract(contract), "%s %s", route.Method, route.Path)

		key := route.Method + " " + route.Path
		_, duplicate := seen[key]
		require.False(t, duplicate, "duplicate route %s", key)
		seen[key] = struct{}{}
		got = append(got, key)
	}

	require.ElementsMatch(t, want, got)
}

func requireGETRoute(t *testing.T, routes []transport.Route, path string, access transport.AccessPolicy, protocol transport.Protocol) {
	t.Helper()

	for _, route := range routes {
		if route.Method == http.MethodGet && route.Path == path {
			require.Equal(t, access, route.Access)
			require.Equal(t, protocol, route.Protocol)

			return
		}
	}

	t.Fatalf("route GET %s not found", path)
}

type browserLoginProvider struct{}

func (browserLoginProvider) Run(ctx context.Context) error {
	<-ctx.Done()

	return ctx.Err()
}

func (browserLoginProvider) Middleware(handler enterprise.Handler) enterprise.Handler {
	return handler
}

func (browserLoginProvider) UsernameFromContext(context.Context) (string, bool) {
	return "", false
}

func (browserLoginProvider) ContextWithUsername(ctx context.Context, _ string) context.Context {
	return ctx
}

func (browserLoginProvider) BrowserLoginEnabled() bool {
	return true
}

func (browserLoginProvider) LoginHandler() enterprise.Handler {
	return noOpHandler
}

func (browserLoginProvider) CallbackHandler() enterprise.Handler {
	return noOpHandler
}

func (browserLoginProvider) CallbackPath() string {
	return "/callback"
}

func (browserLoginProvider) LogoutHandler() enterprise.Handler {
	return noOpHandler
}

func noOpHandler(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
	return nil
}
