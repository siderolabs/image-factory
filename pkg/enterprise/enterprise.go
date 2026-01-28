// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package enterprise provide glue to Enterprise code.
package enterprise

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// AuthProvider defines an authentication provider.
type AuthProvider interface {
	Middleware(
		func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error,
	) func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error
}
