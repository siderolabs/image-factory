// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// registerBrowserLogin adds /login, /logout and the callback route when the auth provider
// serves them.
func (f *Frontend) registerBrowserLogin(registerPublicRoute func(string, func(string, httprouter.Handle), string, Handler)) {
	blp, ok := f.options.AuthProvider.(enterprise.BrowserLoginProvider)
	if !ok || !blp.BrowserLoginEnabled() {
		return
	}

	registerPublicRoute(http.MethodGet, f.router.GET, "/login", blp.LoginHandler())
	registerPublicRoute(http.MethodGet, f.router.GET, "/logout", blp.LogoutHandler())
	registerPublicRoute(http.MethodPost, f.router.POST, "/logout", blp.LogoutHandler())
	registerPublicRoute(http.MethodGet, f.router.GET, blp.CallbackPath(), blp.CallbackHandler())
}
