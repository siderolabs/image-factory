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
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/metadata"
	ociadapter "github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	uiadapter "github.com/siderolabs/image-factory/internal/frontend/http/ui"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

var testMetricsNamespace atomic.Uint64

// NewTestFrontend builds a minimal Frontend wired only with a logger, for tests
// in the external test package that need to exercise the request wrapper.
func NewTestFrontend(logger *zap.Logger) *Frontend {
	return &Frontend{
		logger:      logger,
		browserAuth: browserauth.New(nil),
		metadata:    metadata.New(nil, nil, nil, getLLMsTxt),
		oci:         ociadapter.New(nil),
		ui:          uiadapter.New(nil, nil, uiadapter.Options{}),
	}
}

// NewTestFrontendWithAuth builds a minimal Frontend with an authentication provider.
func NewTestFrontendWithAuth(logger *zap.Logger, provider enterprise.AuthProvider) *Frontend {
	browserAuth := browserauth.New(provider)

	return &Frontend{
		logger:      logger,
		browserAuth: browserAuth,
		metadata:    metadata.New(nil, nil, nil, getLLMsTxt),
		oci:         ociadapter.New(nil),
		ui:          uiadapter.New(nil, nil, uiadapter.Options{AuthProvider: provider, LogoutEnabled: browserAuth.LogoutEnabled()}),
		options:     Options{AuthProvider: provider},
	}
}

// Routes exposes the Community route catalog for external contract tests.
func (f *Frontend) Routes() []transport.Route {
	routes := append(f.routes(), f.oci.Routes()...)

	return append(routes, f.ui.Routes()...)
}

// BrowserLoginRoutes exposes the optional browser-auth route catalog for external contract tests.
func (f *Frontend) BrowserLoginRoutes() []transport.Route {
	return f.browserAuth.Routes()
}

// EnterpriseRoutes exposes Enterprise plugin descriptors for external contract tests.
func (f *Frontend) EnterpriseRoutes(plugins []enterprise.FrontendPlugin) ([]transport.Route, error) {
	return f.enterpriseRoutes(plugins)
}

// RegisterTestRoutes registers the Community catalog through the production registrar.
func RegisterTestRoutes(ctx context.Context, logger *zap.Logger) (http.Handler, error) {
	return RegisterTestRoutesWithAuth(ctx, logger, nil)
}

// RegisterTestRoutesWithAuth registers the Community catalog through the production pipeline with authentication.
func RegisterTestRoutesWithAuth(
	ctx context.Context,
	logger *zap.Logger,
	provider enterprise.AuthProvider,
) (http.Handler, error) {
	contract, err := api.NewContract(ctx)
	if err != nil {
		return nil, err
	}

	frontend := &Frontend{
		logger:      logger,
		contract:    contract,
		browserAuth: browserauth.New(provider),
		options: Options{
			AuthProvider:     provider,
			MetricsNamespace: fmt.Sprintf("image_factory_test_%d", testMetricsNamespace.Add(1)),
		},
	}
	frontend.ui = uiadapter.New(nil, nil, uiadapter.Options{
		AuthProvider:  provider,
		LogoutEnabled: frontend.browserAuth.LogoutEnabled(),
	})
	frontend.initializeEndpointOwners(nil)
	frontend.metadata = metadata.New(nil, nil, nil, getLLMsTxt)

	if err = frontend.registerRoutes(nil); err != nil {
		return nil, err
	}

	return frontend.Handler(), nil
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

func ApplyReferrersFilterHeader(header http.Header, artifactType string) {
	applyReferrersFilterHeader(header, artifactType)
}
