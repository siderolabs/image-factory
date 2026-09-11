// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package http_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/enterprise/scanner"
	"github.com/siderolabs/image-factory/enterprise/spdx"
	"github.com/siderolabs/image-factory/enterprise/tokens"
	"github.com/siderolabs/image-factory/enterprise/vex"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestEnterpriseConfigurationOwnership(t *testing.T) {
	t.Parallel()

	for _, browser := range []bool{false, true} {
		t.Run(fmt.Sprintf("browser=%t", browser), func(t *testing.T) {
			t.Parallel()
			contract, err := api.NewContract(t.Context())
			require.NoError(t, err)

			plugins := []enterprise.FrontendPlugin{
				scanner.NewFrontend(nil, nil, nil), spdx.NewFrontend(nil, nil, nil), vex.NewFrontend(nil),
				tokens.NewJWKSFrontend(nil), tokens.NewListCreateFrontend(nil, nil, 0), tokens.NewRevokeFrontend(nil, nil),
			}

			var provider enterprise.AuthProvider
			if browser {
				provider = browserLoginProvider{}
			}

			frontend := newCatalogFrontend(t, provider, plugins...)
			routes := frontend.Routes()
			pluginRoutes, err := frontend.EnterpriseRoutes(plugins)
			require.NoError(t, err)

			routes = append(routes, pluginRoutes...)
			routes = append(routes, browserauth.New(provider).Routes()...)
			require.Empty(t, configurationOwnershipProblems(contract, routes, true, browser))
		})
	}
}
