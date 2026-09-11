// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package http_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/enterprise/scanner"
	"github.com/siderolabs/image-factory/enterprise/spdx"
	"github.com/siderolabs/image-factory/enterprise/tokens"
	"github.com/siderolabs/image-factory/enterprise/vex"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestEnterpriseRouteCatalogMatchesContract(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	plugins := []enterprise.FrontendPlugin{
		scanner.NewFrontend(nil, nil, nil),
		spdx.NewFrontend(nil, nil, nil),
		vex.NewFrontend(nil),
		tokens.NewJWKSFrontend(nil),
		tokens.NewListCreateFrontend(nil, nil, 0),
		tokens.NewRevokeFrontend(nil, nil),
	}

	for _, plugin := range plugins {
		require.NotEmpty(t, plugin.Routes())
	}

	frontend := newCatalogFrontend(t, browserLoginProvider{}, plugins...)
	routes, err := frontend.EnterpriseRoutes(plugins)
	require.NoError(t, err)

	allRoutes := append([]transport.Route{}, routes...)
	allRoutes = append(allRoutes, frontend.Routes()...)
	allRoutes = append(allRoutes, browserauth.New(browserLoginProvider{}).Routes()...)
	requireOpenAPIOperationOwnership(t, contract, allRoutes)

	requireRouteInventory(t, contract, routes, []string{
		"GET /scans/:schematic/:version/:arch/:report",
		"HEAD /scans/:schematic/:version/:arch/:report",
		"GET /spdx/:schematic/:version/:arch",
		"HEAD /spdx/:schematic/:version/:arch",
		"GET /vex/:version/vex.json",
		"HEAD /vex/:version/vex.json",
		"GET /.well-known/jwks.json",
		"GET /tokens",
		"POST /tokens",
		"POST /tokens/:id/revoke",
	})
}
