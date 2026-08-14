// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
)

func TestNewRouter(t *testing.T) {
	t.Parallel()

	router, err := api.NewRouter(t.Context())
	require.NoError(t, err)

	testRoute := func(name, method, target, operationID string, expectedParams map[string]string) {
		t.Helper()

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(t.Context(), method, target, nil)
			route, pathParams, routeErr := router.FindRoute(request)
			require.NoError(t, routeErr)
			require.NotNil(t, route.Operation)
			assert.Equal(t, operationID, route.Operation.OperationID)
			assert.Equal(t, expectedParams, pathParams)
		})
	}

	testRoute("static", http.MethodGet, "/versions", "listVersions", map[string]string{})
	testRoute(
		"artifact",
		http.MethodHead,
		"/image/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/v1.12.0/metal-amd64.raw.xz",
		"headImage",
		map[string]string{
			"schematic": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"version":   "v1.12.0",
			"path":      "metal-amd64.raw.xz",
		},
	)
	testRoute(
		"nested OCI manifest repository name",
		http.MethodGet,
		"/v2/my-company/platform/backend/manifests/v1.12.0",
		"getRegistryManifest",
		map[string]string{
			"name":      "my-company/platform/backend",
			"reference": "v1.12.0",
		},
	)
	testRoute(
		"nested OCI blob repository name",
		http.MethodGet,
		"/v2/my-company/platform/backend/blobs/sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"getRegistryBlob",
		map[string]string{
			"name":   "my-company/platform/backend",
			"digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	)
	testRoute(
		"nested OCI tags repository name",
		http.MethodGet,
		"/v2/my-company/platform/backend/tags/list",
		"listRegistryTags",
		map[string]string{
			"name": "my-company/platform/backend",
		},
	)
	testRoute(
		"nested OCI referrers repository name",
		http.MethodGet,
		"/v2/my-company/platform/backend/referrers/sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"getRegistryReferrers",
		map[string]string{
			"name":   "my-company/platform/backend",
			"digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	)
	testRoute("registry trailing slash", http.MethodHead, "/v2/", "headRegistrySlash", map[string]string{})
}
