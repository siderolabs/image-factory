// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package browserauth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestHandlerPublishesBrowserAuthenticationRoutes(t *testing.T) {
	t.Parallel()

	provider := browserLoginProvider{enabled: true, callbackPath: "/oauth/callback"}
	handler := browserauth.New(provider)

	routes := handler.Routes()
	require.Len(t, routes, 4)
	require.Equal(t, []routeIdentity{
		{method: http.MethodGet, path: "/login", operationID: "startBrowserLogin"},
		{method: http.MethodGet, path: "/logout", operationID: "getBrowserLogout"},
		{method: http.MethodPost, path: "/logout", operationID: "postBrowserLogout"},
		{method: http.MethodGet, path: "/oauth/callback", operationID: "completeBrowserLogin"},
	}, routeIdentities(routes))

	for _, route := range routes {
		require.Equal(t, transport.AccessPublic, route.Access)
		require.Equal(t, transport.ProtocolBrowserAuth, route.Protocol)
		require.NotNil(t, route.Handler)
	}

	require.True(t, handler.LogoutEnabled())
}

func TestHandlerOmitsDisabledBrowserAuthentication(t *testing.T) {
	t.Parallel()

	handler := browserauth.New(browserLoginProvider{})

	require.Empty(t, handler.Routes())
	require.False(t, handler.LogoutEnabled())
}

func TestHandlerOmitsProviderWithoutBrowserAuthentication(t *testing.T) {
	t.Parallel()

	handler := browserauth.New(authProvider{})

	require.Empty(t, handler.Routes())
	require.False(t, handler.LogoutEnabled())
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

type authProvider struct{}

func (authProvider) Run(context.Context) error { return nil }

func (authProvider) Middleware(next enterprise.Handler) enterprise.Handler { return next }

func (authProvider) UsernameFromContext(context.Context) (string, bool) { return "", false }

func (authProvider) ContextWithUsername(ctx context.Context, _ string) context.Context { return ctx }

type browserLoginProvider struct {
	authProvider

	callbackPath string
	enabled      bool
}

func (provider browserLoginProvider) BrowserLoginEnabled() bool { return provider.enabled }

func (browserLoginProvider) LoginHandler() enterprise.Handler { return noopHandler }

func (browserLoginProvider) CallbackHandler() enterprise.Handler { return noopHandler }

func (provider browserLoginProvider) CallbackPath() string { return provider.callbackPath }

func (browserLoginProvider) LogoutHandler() enterprise.Handler { return noopHandler }

func noopHandler(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
	return nil
}
