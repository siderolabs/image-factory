// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

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

// Routes exposes the Community route catalog for external contract tests.
// Keep the private production catalog authoritative instead of duplicating it
// in external fixtures or adding a production API solely for descriptor tests.
func (f *Frontend) Routes() []transport.Route {
	routes := append(f.routes(), f.oci.Routes()...)

	return append(routes, f.ui.Routes()...)
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
// It isolates middleware/registrar behavior from application services, including
// the OCI adapter's deliberate no-service fallback. Real constructor wiring is
// covered separately by catalog and registry composition tests.
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
		contract:    contract,
		browserAuth: browserauth.New(provider),
		oci:         ociadapter.New(nil),
	}
	frontend.ui = uiadapter.New(nil, nil, uiadapter.Options{
		AuthProvider:  provider,
		LogoutEnabled: frontend.browserAuth.LogoutEnabled(),
	})
	frontend.initializeEndpointOwners(nil)
	frontend.metadata = metadata.New(nil, nil, nil, getLLMsTxt)

	middleware := NewRequestMiddleware(logger, contract, provider, nil, nil)
	if err = frontend.registerRoutes(nil, middleware, ServerOptions{MetricsNamespace: fmt.Sprintf("image_factory_test_%d", testMetricsNamespace.Add(1))}); err != nil {
		return nil, err
	}

	return frontend.Handler(), nil
}
