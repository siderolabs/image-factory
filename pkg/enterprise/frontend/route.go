// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package frontend defines dependency-neutral HTTP route contracts for Enterprise plugins.
package frontend

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Handler is the HTTP handler shape implemented by Enterprise route owners.
type Handler = func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error

// RouteAccessPolicy declares whether an Enterprise HTTP route requires authentication.
type RouteAccessPolicy uint8

const (
	// RouteAccessPublic exposes a route without authentication.
	RouteAccessPublic RouteAccessPolicy = iota + 1
	// RouteAccessAuthenticated requires the configured authentication provider.
	RouteAccessAuthenticated
)

// Route is translated into the internal transport route at the HTTP composition boundary.
type Route struct {
	Handler     Handler
	Method      string
	Path        string
	OperationID string
	Access      RouteAccessPolicy
}
