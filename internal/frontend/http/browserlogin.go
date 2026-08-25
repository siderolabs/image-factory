// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// registerBrowserLogin adds /login, /logout and the callback route when the auth provider
// serves them.
func (f *Frontend) registerBrowserLogin(registerPublicRoute func(func(string, httprouter.Handle), string, Handler)) {
	blp, ok := f.options.AuthProvider.(enterprise.BrowserLoginProvider)
	if !ok || !blp.BrowserLoginEnabled() {
		return
	}

	registerPublicRoute(f.router.GET, "/login", blp.LoginHandler())
	registerPublicRoute(f.router.GET, "/logout", blp.LogoutHandler())
	registerPublicRoute(f.router.POST, "/logout", blp.LogoutHandler())
	registerPublicRoute(f.router.GET, blp.CallbackPath(), blp.CallbackHandler())
}

// browserLoginEnabled reports whether the auth provider serves the interactive login flow,
// gating whether pages show the nav bar's logout/node-tokens links.
func (f *Frontend) browserLoginEnabled() bool {
	blp, ok := f.options.AuthProvider.(enterprise.BrowserLoginProvider)

	return ok && blp.BrowserLoginEnabled()
}
