// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package static

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Handler serves one static filesystem with net/http semantics.
type Handler struct {
	server http.Handler
}

// New creates a static-file endpoint handler.
func New(filesystem http.FileSystem) *Handler {
	return &Handler{server: http.FileServer(filesystem)}
}

// Serve delegates to http.FileServer after restoring the wildcard path.
func (handler *Handler) Serve(_ context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
	request.URL.Path = params.ByName("filepath")
	handler.server.ServeHTTP(writer, request)

	return nil
}
