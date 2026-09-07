// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"net/http"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func (f *Frontend) browserLoginRoutes() []transport.Route {
	provider, ok := f.options.AuthProvider.(enterprise.BrowserLoginProvider)
	if !ok || !provider.BrowserLoginEnabled() {
		return nil
	}

	return []transport.Route{
		{Method: http.MethodGet, Path: "/login", OperationID: "startBrowserLogin", Access: transport.AccessPublic, Protocol: transport.ProtocolBrowserAuth, Handler: provider.LoginHandler()},
		{Method: http.MethodGet, Path: "/logout", OperationID: "getBrowserLogout", Access: transport.AccessPublic, Protocol: transport.ProtocolBrowserAuth, Handler: provider.LogoutHandler()},
		{Method: http.MethodPost, Path: "/logout", OperationID: "postBrowserLogout", Access: transport.AccessPublic, Protocol: transport.ProtocolBrowserAuth, Handler: provider.LogoutHandler()},
		{
			Method:      http.MethodGet,
			Path:        provider.CallbackPath(),
			OperationID: "completeBrowserLogin",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolBrowserAuth,
			Handler:     provider.CallbackHandler(),
		},
	}
}

// logoutEnabled reports whether pages should show a logout link, which requires the auth
// provider to serve the interactive browser login flow — htpasswd's Basic-auth challenge
// has no route to hit that would clear the browser's cached credentials.
func (f *Frontend) logoutEnabled() bool {
	provider, ok := f.options.AuthProvider.(enterprise.BrowserLoginProvider)

	return ok && provider.BrowserLoginEnabled()
}
