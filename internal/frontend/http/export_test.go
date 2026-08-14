// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
)

// NewTestFrontend builds a minimal Frontend wired only with a logger, for tests
// in the external test package that need to exercise the request wrapper.
func NewTestFrontend(logger *zap.Logger) *Frontend {
	return &Frontend{logger: logger}
}

// NewTestFrontendWithContract builds a minimal Frontend with OpenAPI request validation enabled.
func NewTestFrontendWithContract(logger *zap.Logger) *Frontend {
	frontend := NewTestFrontend(logger)
	frontend.contract = ensure.Value(api.NewContract(context.Background()))

	return frontend
}

// WrapHandler exposes the unexported request wrapper for external tests.
func (f *Frontend) WrapHandler(h Handler) httprouter.Handle {
	return f.wrapper(h)
}

// HandleLLMsTxt exposes the llms.txt handler for external tests.
func (f *Frontend) HandleLLMsTxt() Handler {
	return f.handleLLMsTxt
}

// HandleOpenAPI exposes the OpenAPI handler for external tests.
func (f *Frontend) HandleOpenAPI() Handler {
	return f.handleOpenAPI
}

func ApplyReferrersFilterHeader(header http.Header, artifactType string) {
	applyReferrersFilterHeader(header, artifactType)
}
