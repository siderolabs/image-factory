// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestCommunityConfigurationOwnership(t *testing.T) {
	t.Parallel()

	for _, browser := range []bool{false, true} {
		t.Run(fmt.Sprintf("browser=%t", browser), func(t *testing.T) {
			t.Parallel()
			contract, err := api.NewContract(t.Context())
			require.NoError(t, err)

			var provider enterprise.AuthProvider
			if browser {
				provider = browserLoginProvider{}
			}

			routes := newCatalogFrontend(t, provider).Routes()
			routes = append(routes, browserauth.New(provider).Routes()...)
			require.Empty(t, configurationOwnershipProblems(contract, routes, false, browser))
		})
	}
}

func TestConfigurationOwnershipRejectsMissingDispatchedOwner(t *testing.T) {
	t.Parallel()
	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)
	routes := newCatalogFrontend(t, nil).Routes()
	require.Empty(t, configurationOwnershipProblems(contract, routes, false, false))

	for i := range routes {
		if len(routes[i].DispatchedOperationIDs) == 0 {
			continue
		}

		missing := routes[i].DispatchedOperationIDs[0]
		routes[i].DispatchedOperationIDs = slices.Clone(routes[i].DispatchedOperationIDs[1:])
		require.Contains(t, configurationOwnershipProblems(contract, routes, false, false), fmt.Sprintf("%s: expected 1 owner, got 0", missing))

		return
	}

	t.Fatal("fixture has no dispatched operation")
}

func TestConfigurationOwnershipRejectsDuplicateAndUnexpectedOwners(t *testing.T) {
	t.Parallel()
	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	routes := newCatalogFrontend(t, nil).Routes()
	for _, operation := range []string{"checkRegistry", "unknownOperation", "startBrowserLogin", "getAPITokenJWKS"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			candidate := append(slices.Clone(routes), transport.Route{OperationID: operation})
			require.NotEmpty(t, configurationOwnershipProblems(contract, candidate, false, false))
		})
	}
}

// The expected operation set is derived from the contract and explicit deployment
// capabilities, never from the routes under test. Enterprise annotations describe
// product availability, not registration: the token UI shell is always registered,
// while browser endpoints depend on the provider capability in either build.
func configurationOwnershipProblems(contract *api.Contract, routes []transport.Route, enterpriseEnabled, browserEnabled bool) []string {
	expected := map[string]bool{}

	var problems []string

	for path, item := range contract.Document.Paths.Map() {
		for method, operation := range item.Operations() {
			id := operation.OperationID
			if id == "" {
				problems = append(problems, fmt.Sprintf("%s %s: missing operation ID", method, path))

				continue
			}

			if _, exists := expected[id]; exists {
				problems = append(problems, "duplicate contract operation: "+id)
			}

			enabled := enterpriseEnabled || operation.Extensions["x-image-factory-enterprise"] != true

			switch id {
			case "getUITokens":
				enabled = true
			case "startBrowserLogin", "getBrowserLogout", "postBrowserLogout", "completeBrowserLogin":
				enabled = browserEnabled
			}

			expected[id] = enabled
		}
	}

	owned := operationOwners(routes)

	for id, enabled := range expected {
		want := 0
		if enabled {
			want = 1
		}

		if len(owned[id]) != want {
			problems = append(problems, fmt.Sprintf("%s: expected %d owner, got %d", id, want, len(owned[id])))
		}
	}

	for id := range owned {
		if _, exists := expected[id]; !exists {
			problems = append(problems, "unknown operation: "+id)
		}
	}

	slices.Sort(problems)

	return problems
}
