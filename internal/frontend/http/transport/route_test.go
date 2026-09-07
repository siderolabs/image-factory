// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

func TestRouteValidate(t *testing.T) {
	t.Parallel()

	handler := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		return nil
	}

	for _, test := range []struct {
		route   *transport.Route
		name    string
		wantErr string
	}{
		{
			name: "public API",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/versions",
				OperationID: "listVersions",
				Access:      transport.AccessPublic,
				Protocol:    transport.ProtocolAPI,
				Handler:     handler,
			},
		},
		{
			name: "authenticated API",
			route: &transport.Route{
				Method:      http.MethodPost,
				Path:        "/schematics",
				OperationID: "createSchematic",
				Access:      transport.AccessAuthenticated,
				Protocol:    transport.ProtocolAPI,
				Handler:     handler,
			},
		},
		{
			name: "static greedy route",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/css/*filepath",
				OperationID: "getCSSAsset",
				Access:      transport.AccessPublic,
				Protocol:    transport.ProtocolStatic,
				Handler:     handler,
			},
		},
		{
			name: "OCI dispatcher",
			route: &transport.Route{
				Method: http.MethodGet,
				Path:   "/v2/*path",
				DispatchedOperationIDs: []string{
					"checkRegistrySlash",
					"getRegistryManifest",
					"getRegistryBlob",
					"listRegistryTags",
					"getRegistryReferrers",
				},
				Access:   transport.AccessImageDownload,
				Protocol: transport.ProtocolOCI,
				Handler:  handler,
			},
		},
		{
			name: "missing method",
			route: &transport.Route{
				Path:        "/versions",
				OperationID: "listVersions",
				Handler:     handler,
			},
			wantErr: "method is required",
		},
		{
			name: "missing path",
			route: &transport.Route{
				Method:      http.MethodGet,
				OperationID: "listVersions",
				Handler:     handler,
			},
			wantErr: "path is required",
		},
		{
			name: "missing operation ID",
			route: &transport.Route{
				Method:  http.MethodGet,
				Path:    "/versions",
				Handler: handler,
			},
			wantErr: "operation ID is required",
		},
		{
			name: "missing handler",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/versions",
				OperationID: "listVersions",
			},
			wantErr: "handler is required",
		},
		{
			name: "invalid access policy",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/versions",
				OperationID: "listVersions",
				Access:      transport.AccessPolicy(255),
				Handler:     handler,
			},
			wantErr: "unsupported access policy",
		},
		{
			name: "invalid protocol",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/versions",
				OperationID: "listVersions",
				Protocol:    transport.Protocol(255),
				Handler:     handler,
			},
			wantErr: "unsupported protocol",
		},
		{
			name: "authenticated static route",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/css/*filepath",
				OperationID: "getCSSAsset",
				Access:      transport.AccessAuthenticated,
				Protocol:    transport.ProtocolStatic,
				Handler:     handler,
			},
			wantErr: "static routes must be public",
		},
		{
			name: "dispatcher operations on ordinary API route",
			route: &transport.Route{
				Method:                 http.MethodGet,
				Path:                   "/versions",
				DispatchedOperationIDs: []string{"listVersions"},
				Handler:                handler,
			},
			wantErr: "dispatcher operations require an OCI dispatcher",
		},
		{
			name: "OCI dispatcher with one operation",
			route: &transport.Route{
				Method:      http.MethodGet,
				Path:        "/v2/*path",
				OperationID: "getRegistryManifest",
				Access:      transport.AccessImageDownload,
				Protocol:    transport.ProtocolOCI,
				Handler:     handler,
			},
			wantErr: "OCI dispatcher must declare dispatched operation IDs",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.route.Validate()
			if test.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestRouteValidateContract(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	handler := func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
		return nil
	}

	require.NoError(t, (transport.Route{
		Method:      http.MethodGet,
		Path:        "/versions",
		OperationID: "listVersions",
		Access:      transport.AccessPublic,
		Protocol:    transport.ProtocolAPI,
		Handler:     handler,
	}).ValidateContract(contract))

	require.ErrorContains(t, (transport.Route{
		Method:      http.MethodGet,
		Path:        "/versions",
		OperationID: "headHealth",
		Access:      transport.AccessPublic,
		Protocol:    transport.ProtocolAPI,
		Handler:     handler,
	}).ValidateContract(contract), `declares operation "listVersions", not "headHealth"`)

	require.NoError(t, (transport.Route{
		Method: http.MethodGet,
		Path:   "/v2/*path",
		DispatchedOperationIDs: []string{
			"checkRegistrySlash",
			"getRegistryManifest",
			"getRegistryBlob",
			"listRegistryTags",
			"getRegistryReferrers",
		},
		Access:   transport.AccessImageDownload,
		Protocol: transport.ProtocolOCI,
		Handler:  handler,
	}).ValidateContract(contract))

	require.ErrorContains(t, (transport.Route{
		Method:                 http.MethodGet,
		Path:                   "/v2/*path",
		DispatchedOperationIDs: []string{"checkRegistrySlash"},
		Access:                 transport.AccessImageDownload,
		Protocol:               transport.ProtocolOCI,
		Handler:                handler,
	}).ValidateContract(contract), `missing OpenAPI operation "getRegistryManifest"`)
}
