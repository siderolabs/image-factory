// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operational

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// ReadinessChecker reports whether one application dependency is ready to serve traffic.
type ReadinessChecker interface {
	Ready() error
}

// Handler owns operational HTTP endpoints.
type Handler struct {
	readinessCheckers []ReadinessChecker
}

// New creates an operational endpoint handler.
func New(readinessCheckers ...ReadinessChecker) *Handler {
	return &Handler{readinessCheckers: readinessCheckers}
}

// Health reports process health.
func (*Handler) Health(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
	return nil
}

// Ready reports readiness of every configured dependency.
func (handler *Handler) Ready(_ context.Context, writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	for _, checker := range handler.readinessCheckers {
		if err := checker.Ready(); err != nil {
			http.Error(writer, "not ready", http.StatusServiceUnavailable)

			// The readiness failure is fully represented by the HTTP response.
			//nolint:nilerr
			return nil
		}
	}

	return nil
}
