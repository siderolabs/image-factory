// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package browserauth adapts optional browser authentication providers to HTTP routes.
package browserauth

import (
	"net/http"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// Handler owns the browser-authentication HTTP route catalog.
type Handler struct {
	provider enterprise.BrowserLoginProvider
}

// New constructs a browser-authentication adapter from an optional authentication provider.
func New(provider enterprise.AuthProvider) *Handler {
	browserProvider, ok := provider.(enterprise.BrowserLoginProvider)
	if !ok {
		return &Handler{}
	}

	return &Handler{provider: browserProvider}
}

// Routes returns the public login, callback, and logout routes when browser login is enabled.
func (handler *Handler) Routes() []transport.Route {
	if !handler.LogoutEnabled() {
		return nil
	}

	return []transport.Route{
		{Method: http.MethodGet, Path: "/login", OperationID: "startBrowserLogin", Access: transport.AccessPublic, Protocol: transport.ProtocolBrowserAuth, Handler: handler.provider.LoginHandler()},
		{Method: http.MethodGet, Path: "/logout", OperationID: "getBrowserLogout", Access: transport.AccessPublic, Protocol: transport.ProtocolBrowserAuth, Handler: handler.provider.LogoutHandler()},
		{
			Method:      http.MethodPost,
			Path:        "/logout",
			OperationID: "postBrowserLogout",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolBrowserAuth,
			Handler:     handler.provider.LogoutHandler(),
		},
		{
			Method:      http.MethodGet,
			Path:        handler.provider.CallbackPath(),
			OperationID: "completeBrowserLogin",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolBrowserAuth,
			Handler:     handler.provider.CallbackHandler(),
		},
	}
}

// LogoutEnabled reports whether pages should show a logout link.
func (handler *Handler) LogoutEnabled() bool {
	return handler != nil && handler.provider != nil && handler.provider.BrowserLoginEnabled()
}
