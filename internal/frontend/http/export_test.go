// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/metadata"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

var testMetricsNamespace atomic.Uint64

// NewTestFrontend builds a minimal Frontend wired only with a logger, for tests
// in the external test package that need to exercise the request wrapper.
func NewTestFrontend(logger *zap.Logger) *Frontend {
	return &Frontend{logger: logger, metadata: metadata.New(nil, nil, nil, getLLMsTxt())}
}

// NewTestFrontendWithAuth builds a minimal Frontend with an authentication provider.
func NewTestFrontendWithAuth(logger *zap.Logger, provider enterprise.AuthProvider) *Frontend {
	return &Frontend{
		logger:   logger,
		metadata: metadata.New(nil, nil, nil, getLLMsTxt()),
		options:  Options{AuthProvider: provider},
	}
}

// Routes exposes the Community route catalog for external contract tests.
func (f *Frontend) Routes() []transport.Route {
	return f.routes()
}

// BrowserLoginRoutes exposes the optional browser-auth route catalog for external contract tests.
func (f *Frontend) BrowserLoginRoutes() []transport.Route {
	return f.browserLoginRoutes()
}

// EnterpriseRoutes exposes Enterprise plugin descriptors for external contract tests.
func (f *Frontend) EnterpriseRoutes(plugins []enterprise.FrontendPlugin) []transport.Route {
	return f.enterpriseRoutes(plugins)
}

// RegisterTestRoutes registers the Community catalog through the production registrar.
func RegisterTestRoutes(ctx context.Context, logger *zap.Logger, router *httprouter.Router) error {
	return RegisterTestRoutesWithAuth(ctx, logger, router, nil)
}

// RegisterTestRoutesWithAuth registers the Community catalog through the production pipeline with authentication.
func RegisterTestRoutesWithAuth(
	ctx context.Context,
	logger *zap.Logger,
	router *httprouter.Router,
	provider enterprise.AuthProvider,
) error {
	contract, err := api.NewContract(ctx)
	if err != nil {
		return err
	}

	frontend := &Frontend{
		logger:   logger,
		contract: contract,
		options: Options{
			AuthProvider:     provider,
			MetricsNamespace: fmt.Sprintf("image_factory_test_%d", testMetricsNamespace.Add(1)),
		},
	}
	frontend.initializeEndpointOwners(nil)
	frontend.metadata = metadata.New(nil, nil, nil, getLLMsTxt())

	return frontend.registerRoutes(router, nil)
}

// WrapHandler exposes the unexported request wrapper for external tests.
func (f *Frontend) WrapHandler(h Handler) httprouter.Handle {
	return f.wrapper(h)
}

// WrapHandlerForProtocol exposes protocol-aware error handling for external compatibility tests.
func (f *Frontend) WrapHandlerForProtocol(h Handler, protocol transport.Protocol) httprouter.Handle {
	return f.wrapHandlerProtocol(h, true, protocol)
}

// HandleLLMsTxt exposes the llms.txt handler for external tests.
func (f *Frontend) HandleLLMsTxt() Handler {
	return f.metadata.LLMsText
}

// HandleTokensUI exposes the token management page handler for external tests.
func (f *Frontend) HandleTokensUI() Handler {
	return f.handleTokensUI
}

func ApplyReferrersFilterHeader(header http.Header, artifactType string) {
	applyReferrersFilterHeader(header, artifactType)
}
