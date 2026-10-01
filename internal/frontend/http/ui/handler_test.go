// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/frontend/http/ui"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestHandlerPublishesUIRoutes(t *testing.T) {
	t.Parallel()

	handler := ui.New(nil, nil, ui.Options{})
	routes := handler.Routes()

	require.Equal(t, []routeIdentity{
		{method: http.MethodGet, path: "/", operationID: "getUI"},
		{method: http.MethodHead, path: "/", operationID: "headUI"},
		{method: http.MethodPost, path: "/ui/wizard", operationID: "postUIWizard"},
		{method: http.MethodGet, path: "/ui/version-doc", operationID: "getUIVersionDocumentation"},
		{method: http.MethodPost, path: "/ui/extensions-list", operationID: "postUIExtensionsList"},
		{method: http.MethodGet, path: "/ui/tokens", operationID: "getUITokens"},
	}, routeIdentities(routes))

	for _, route := range routes {
		require.Equal(t, transport.AccessAuthenticated, route.Access)
		require.Equal(t, transport.ProtocolHTML, route.Protocol)
		require.NotNil(t, route.Handler)
	}
}

func TestHandlerHeadReturnsNoBodyWithoutCallingDependencies(t *testing.T) {
	t.Parallel()

	handler := ui.New(nil, nil, ui.Options{})
	route := requireRootRoute(t, handler.Routes(), http.MethodHead)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/", nil)

	require.NoError(t, route.Handler(t.Context(), recorder, request, nil))
	require.Empty(t, recorder.Body.String())
}

func TestHandlerLanguageSelectionSetsCookieAndHTMXRedirect(t *testing.T) {
	t.Parallel()

	handler := ui.New(nil, nil, ui.Options{})
	route := requireRootRoute(t, handler.Routes(), http.MethodGet)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?keep=value&lang=pl", nil)

	require.NoError(t, route.Handler(t.Context(), recorder, request, nil))
	require.Equal(t, "/?keep=value", recorder.Header().Get("Hx-Redirect"))

	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "lang", cookies[0].Name)
	require.Equal(t, "pl", cookies[0].Value)
	require.Equal(t, "/", cookies[0].Path)
	require.Equal(t, 60*60*24*365, cookies[0].MaxAge)
	require.True(t, cookies[0].HttpOnly)
}

func TestHandlerRendersLocalizedPageThroughArtifactPort(t *testing.T) {
	t.Parallel()

	handler := ui.New(nil, fakeArtifactSource{}, ui.Options{})
	route := requireRootRoute(t, handler.Routes(), http.MethodGet)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("Accept-Language", "pl")

	require.NoError(t, route.Handler(t.Context(), recorder, request, nil))
	require.Contains(t, recorder.Body.String(), "Typ Sprzętu")
	require.Contains(t, recorder.Body.String(), "Serwer Fizyczny")
	require.Contains(t, recorder.Body.String(), `<html lang="pl">`)
}

func TestHandlerRendersSearchMetadata(t *testing.T) {
	t.Parallel()

	handler := ui.New(nil, fakeArtifactSource{}, ui.Options{})
	route := requireRootRoute(t, handler.Routes(), http.MethodGet)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	require.NoError(t, route.Handler(t.Context(), recorder, request, nil))

	body := recorder.Body.String()
	title := "Talos Linux Image Factory"
	description := "Build custom Talos Linux boot images and installers. Pick your platform, architecture, and system extensions, then download an ISO, disk image, or PXE link."

	if enterprise.Enabled() {
		title = "Talos Linux Image Factory Enterprise"
		description = "Build signed Talos Enterprise Linux boot images and installers, with an SBOM and VEX for every image. Sign in with your Talos Enterprise account."
	}

	require.Contains(t, body, `<html lang="en">`)
	require.Contains(t, body, "<title>"+title+"</title>")
	require.Contains(t, body, `<meta name="description" content="`+description+`">`)
	require.Contains(t, body, `<meta property="og:title" content="`+title+`">`)
	require.Contains(t, body, `<meta property="og:description" content="`+description+`">`)
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

// requireRootRoute finds the UI root route for method; every test here drives "/".
func requireRootRoute(t *testing.T, routes []transport.Route, method string) transport.Route {
	t.Helper()

	for _, route := range routes {
		if route.Method == method && route.Path == "/" {
			return route
		}
	}

	t.Fatalf("route %s / not found", method)

	return transport.Route{}
}

type fakeArtifactSource struct{}

func (fakeArtifactSource) GetTalosVersions(context.Context) ([]semver.Version, error) {
	return []semver.Version{{Major: 1, Minor: 14}}, nil
}

func (fakeArtifactSource) GetOfficialExtensions(context.Context, string) ([]artifacts.ExtensionRef, error) {
	return nil, nil
}

func (fakeArtifactSource) GetTalosctlTuples(context.Context, string) ([]artifacts.TalosctlTuple, error) {
	return nil, nil
}
