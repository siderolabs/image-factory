// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	t.Parallel()

	router, err := NewRouter(t.Context())
	require.NoError(t, err)

	tests := []struct {
		name        string
		method      string
		target      string
		operationID string
		pathParams  map[string]string
	}{
		{
			name:        "static",
			method:      http.MethodGet,
			target:      "/versions",
			operationID: "listVersions",
			pathParams:  map[string]string{},
		},
		{
			name:        "artifact",
			method:      http.MethodHead,
			target:      "/image/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/v1.12.0/metal-amd64.raw.xz",
			operationID: "headImage",
			pathParams: map[string]string{
				"schematic": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"version":   "v1.12.0",
				"path":      "metal-amd64.raw.xz",
			},
		},
		{
			name:        "nested OCI repository name",
			method:      http.MethodGet,
			target:      "/v2/my-company/platform/backend/manifests/v1.12.0",
			operationID: "getRegistryManifest",
			pathParams: map[string]string{
				"name":      "my-company/platform/backend",
				"reference": "v1.12.0",
			},
		},
		{
			name:        "registry trailing slash",
			method:      http.MethodHead,
			target:      "/v2/",
			operationID: "headRegistrySlash",
			pathParams:  map[string]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.target, nil)
			route, pathParams, routeErr := router.FindRoute(request)
			require.NoError(t, routeErr)
			require.NotNil(t, route.Operation)
			assert.Equal(t, test.operationID, route.Operation.OperationID)
			assert.Equal(t, test.pathParams, pathParams)
		})
	}
}
