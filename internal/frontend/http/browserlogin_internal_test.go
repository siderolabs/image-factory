// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// In-package: registerBrowserLogin is unexported, and NewFrontend cannot be stood up in a
// unit test without a registry to pull from.

package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// stubBrowserLogin is an AuthProvider that also serves browser login on callbackPath.
type stubBrowserLogin struct {
	callbackPath string
}

func (stubBrowserLogin) Run(context.Context) error { return nil }

func (stubBrowserLogin) Middleware(h enterprise.Handler) enterprise.Handler { return h }

func (stubBrowserLogin) UsernameFromContext(context.Context) (string, bool) { return "", false }

func (stubBrowserLogin) ContextWithUsername(ctx context.Context, _ string) context.Context {
	return ctx
}

func (stubBrowserLogin) BrowserLoginEnabled() bool { return true }

func (s stubBrowserLogin) CallbackPath() string { return s.callbackPath }

func (stubBrowserLogin) LoginHandler() enterprise.Handler { return nopHandler }

func (stubBrowserLogin) LogoutHandler() enterprise.Handler { return nopHandler }

func (stubBrowserLogin) CallbackHandler() enterprise.Handler { return nopHandler }

func nopHandler(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
	return nil
}

// TestRegisterBrowserLoginRoutes pins the routes a browser-login provider gets. They are all
// fixed paths, so a collision with an earlier route is a bug that panics at startup.
func TestRegisterBrowserLoginRoutes(t *testing.T) {
	t.Parallel()

	f := &Frontend{router: httprouter.New()}
	f.options.AuthProvider = stubBrowserLogin{callbackPath: "/callback"}

	var registered []string

	f.registerBrowserLogin(func(_ func(string, httprouter.Handle), path string, _ Handler) {
		registered = append(registered, path)
	})

	require.Equal(t, []string{"/login", "/logout", "/logout", "/callback"}, registered)
}

// TestRegisterBrowserLoginSkipsPlainProvider pins that a provider without browser login
// leaves the routes unregistered rather than panicking on its nil internals.
func TestRegisterBrowserLoginSkipsPlainProvider(t *testing.T) {
	t.Parallel()

	f := &Frontend{router: httprouter.New()}

	f.registerBrowserLogin(func(func(string, httprouter.Handle), string, Handler) {
		t.Fatal("no route may be registered without a browser-login provider")
	})
}
