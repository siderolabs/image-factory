// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"
)

// NewTestFrontend builds a minimal Frontend wired only with a logger, for tests
// in the external test package that need to exercise the request wrapper.
func NewTestFrontend(logger *zap.Logger) *Frontend {
	return &Frontend{logger: logger}
}

// WrapHandler exposes the unexported request wrapper for external tests.
func (f *Frontend) WrapHandler(h Handler) httprouter.Handle {
	return f.wrapper(h)
}
