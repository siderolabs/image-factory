// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/routers"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
)

func TestBrowserCallbackContract(t *testing.T) {
	t.Parallel()

	for _, callbackPath := range []string{"/callback", "/oauth/callback", "/oauth/callback/"} {
		t.Run(callbackPath, func(t *testing.T) {
			t.Parallel()

			contract, err := api.NewContract(t.Context(), api.WithBrowserCallbackPath(callbackPath))
			require.NoError(t, err)
			require.NoError(t, contract.Document.Validate(t.Context()))
			require.NoError(t, contract.ValidateRuntimeOperation(http.MethodGet, callbackPath, "completeBrowserLogin"))
			require.Error(t, contract.ValidateRuntimeOperation(http.MethodPost, callbackPath, "completeBrowserLogin"))
			require.Error(t, contract.ValidateRuntimeOperation(http.MethodGet, callbackPath, "listVersions"))
			require.Error(t, contract.ValidateRuntimeOperation(http.MethodGet, "/versions", "completeBrowserLogin"))
			require.NoError(t, contract.ValidateRuntimeOperation(http.MethodGet, "/versions", "listVersions"))

			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, callbackPath+"?code=original&state=state", nil)
			route, _, err := contract.ValidateRequest(t.Context(), request)
			require.NoError(t, err)
			require.Equal(t, "completeBrowserLogin", route.Operation.OperationID)
			require.Equal(t, callbackPath, request.URL.Path)
			require.Equal(t, "code=original&state=state", request.URL.RawQuery)

			request.Method = http.MethodPost
			_, _, err = contract.ValidateRequest(t.Context(), request)
			require.ErrorIs(t, err, routers.ErrMethodNotAllowed)

			if callbackPath != "/callback" {
				require.Error(t, contract.ValidateRuntimeOperation(http.MethodGet, "/callback", "completeBrowserLogin"))
				_, _, err = contract.ValidateRequest(t.Context(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback", nil))
				require.ErrorIs(t, err, routers.ErrPathNotFound)
			}
		})
	}

	canonical, err := api.NewContract(t.Context())
	require.NoError(t, err)
	require.NoError(t, canonical.ValidateRuntimeOperation(http.MethodGet, "/callback", "completeBrowserLogin"))
	require.Error(t, canonical.ValidateRuntimeOperation(http.MethodGet, "/oauth/callback", "completeBrowserLogin"))
}

func TestBrowserCallbackRejectsInvalidAndConflictingPaths(t *testing.T) {
	t.Parallel()

	for _, callbackPath := range []string{
		"", "callback", "https://example.com/callback", "//example.com/callback",
		"/oauth/../callback", "/oauth//callback", "/oauth/./callback",
		"/oauth/:callback", "/oauth/*callback", "/oauth/{callback}",
		"/oauth/callback?code=x", "/oauth/callback#fragment", "/oauth/%63allback",
		"/oauth/call back", "/oauth\\callback", "/oauth/callback\n",
		"/", "/login", "/logout", "/versions", "/schematics", "/ui/wizard",
		"/css/callback", "/js/oauth/callback", "/image/schematic/version/callback",
		"/v2/repository/manifests/callback",
	} {
		t.Run(callbackPath, func(t *testing.T) {
			t.Parallel()

			contract, err := api.NewContract(t.Context(), api.WithBrowserCallbackPath(callbackPath))
			require.Error(t, err)
			require.Nil(t, contract)
		})
	}
}
