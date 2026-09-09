// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestOperationOwnersPreserveDuplicates(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		routes []transport.Route
	}{
		{name: "ordinary", routes: []transport.Route{{OperationID: "operation"}, {OperationID: "operation"}}},
		{name: "dispatched", routes: []transport.Route{{DispatchedOperationIDs: []string{"operation"}}, {DispatchedOperationIDs: []string{"operation"}}}},
		{name: "mixed", routes: []transport.Route{{OperationID: "operation"}, {DispatchedOperationIDs: []string{"operation"}}}},
		{name: "within dispatcher", routes: []transport.Route{{DispatchedOperationIDs: []string{"operation", "operation"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Len(t, operationOwners(test.routes)["operation"], 2)
		})
	}
}

func TestOperationOwnersIncludeContractAliases(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	routes := newCatalogFrontend(t, nil).Routes()
	owners := operationOwners(routes)
	require.Equal(t, []string{"GET /v2"}, owners["checkRegistry"])
	require.Equal(t, []string{"GET /v2/*path"}, owners["checkRegistrySlash"])
	require.Equal(t, []string{"HEAD /v2"}, owners["headRegistry"])
	require.Equal(t, []string{"HEAD /v2/*path"}, owners["headRegistrySlash"])

	for _, route := range routes {
		require.NoError(t, route.ValidateContract(contract))
	}
	// A trailing slash is a distinct canonical operation, not permission for
	// a second owner of the unsuffixed operation ID.
	require.Error(t, contract.ValidateRuntimeOperation("GET", "/v2/", "checkRegistry"))
	require.NoError(t, contract.ValidateRuntimeOperation("GET", "/v2/", "checkRegistrySlash"))
}

func operationOwners(routes []transport.Route) map[string][]string {
	owners := make(map[string][]string, len(routes))
	for _, route := range routes {
		if route.OperationID != "" {
			owners[route.OperationID] = append(owners[route.OperationID], route.Method+" "+route.Path)
		}

		for _, operationID := range route.DispatchedOperationIDs {
			owners[operationID] = append(owners[operationID], route.Method+" "+route.Path)
		}
	}

	return owners
}
