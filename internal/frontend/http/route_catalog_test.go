// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
	httpfrontend "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestFrontendRegistersCatalogThroughTransport(t *testing.T) {
	t.Parallel()

	router, err := httpfrontend.RegisterTestRoutes(t.Context(), zap.NewNop())
	require.NoError(t, err)

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

func TestAssembledRouterDispatchesOCIWithoutCallback(t *testing.T) {
	t.Parallel()

	router, err := httpfrontend.RegisterTestRoutes(t.Context(), zap.NewNop())
	require.NoError(t, err)

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v2/installer/schematic/manifests/v1.13.0", nil)
	require.NotPanics(t, func() { router.ServeHTTP(response, request) })
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Contains(t, response.Body.String(), "unknown registry route")
}

func TestCommunityRouteCatalogMatchesContract(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	frontend := newCatalogFrontend(t, nil)
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

	routes := browserauth.New(browserLoginProvider{}).Routes()

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

func TestEnterpriseRouteCatalogRejectsInvalidAccessPolicy(t *testing.T) {
	t.Parallel()

	frontend := newCatalogFrontend(t, nil)

	for _, access := range []enterprise.RouteAccessPolicy{0, 255} {
		_, err := frontend.EnterpriseRoutes([]enterprise.FrontendPlugin{invalidAccessPlugin{access: access}})
		require.ErrorContains(t, err, "unsupported access policy")
	}
}

// newCatalogFrontend exercises production composition without starting upstream
// services. Catalog tests inspect descriptors only; publication must not run.
func newCatalogFrontend(t *testing.T, provider enterprise.AuthProvider, plugins ...enterprise.FrontendPlugin) *httpfrontend.Frontend {
	t.Helper()

	repository, err := name.NewRepository("registry.example.com/catalog")
	require.NoError(t, err)

	externalURL := &url.URL{Scheme: "https", Host: "factory.example.com"}
	frontend, err := httpfrontend.NewFrontend(t.Context(), zap.NewNop(), nil, nil, nil, nil, nil, nil, plugins, httpfrontend.Options{
		ExternalURL: externalURL, ExternalPXEURL: externalURL,
		InstallerInternalRepository: repository, InstallerExternalRepository: repository,
		RegistryRefreshInterval: time.Minute, AuthProvider: provider,
		CacheImageSigner: compositionSigner{}, InstallerSBOMSource: compositionSBOM{},
		MetricsNamespace: "catalog_" + t.Name(),
	})
	require.NoError(t, err)

	return frontend
}

func requireOpenAPIOperationOwnership(t *testing.T, contract *api.Contract, routes []transport.Route) {
	t.Helper()

	owned := operationOwners(routes)

	for path, pathItem := range contract.Document.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			require.NotEmpty(t, operation.OperationID, "%s %s has no operation ID", method, path)
			require.Len(t, owned[operation.OperationID], 1, "%s %s operation %q must have exactly one runtime owner: %v", method, path, operation.OperationID, owned[operation.OperationID])
		}
	}
}

func requireRouteInventory(t *testing.T, contract *api.Contract, routes []transport.Route, want []string) {
	t.Helper()

	got := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))

	owned := operationOwners(routes)
	for operationID, owners := range owned {
		require.Len(t, owners, 1, "operation %q has duplicate runtime owners: %v", operationID, owners)
	}

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

type invalidAccessPlugin struct {
	access enterprise.RouteAccessPolicy
}

func (plugin invalidAccessPlugin) Routes() []enterprise.Route {
	return []enterprise.Route{{Access: plugin.access}}
}

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
