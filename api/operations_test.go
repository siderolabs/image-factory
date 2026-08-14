// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
)

func TestUserFacingOperations(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	expected := map[string]map[string]string{
		"/.well-known/jwks.json": {
			http.MethodGet: "getDownloadTokenJWKS",
		},
		"/download-token": {
			http.MethodPost: "createDownloadToken",
		},
		"/image/{schematic}/{version}/{path}": {
			http.MethodGet:  "getImage",
			http.MethodHead: "headImage",
		},
		"/llms.txt": {
			http.MethodGet: "getLLMsText",
		},
		"/oci/cosign/signing-key.pub": {
			http.MethodGet: "getCosignSigningKey",
		},
		"/openapi.yaml": {
			http.MethodGet: "getOpenAPI",
		},
		"/pxe/{schematic}/{version}/{path}": {
			http.MethodGet: "getPXEScript",
		},
		"/scans/{schematic}/{version}/{arch}/{report}": {
			http.MethodGet:  "getVulnerabilityScan",
			http.MethodHead: "headVulnerabilityScan",
		},
		"/schematics": {
			http.MethodPost: "createSchematic",
		},
		"/schematics/{schematic}": {
			http.MethodGet: "getSchematic",
		},
		"/secureboot/signing-cert.pem": {
			http.MethodGet: "getSecureBootSigningCertificate",
		},
		"/spdx/{schematic}/{version}/{arch}": {
			http.MethodGet:  "getSPDX",
			http.MethodHead: "headSPDX",
		},
		"/talosctl/{version}": {
			http.MethodGet: "listTalosctlDownloads",
		},
		"/talosctl/{version}/{path}": {
			http.MethodGet:  "getTalosctl",
			http.MethodHead: "headTalosctl",
		},
		"/v2": {
			http.MethodGet:  "checkRegistry",
			http.MethodHead: "headRegistry",
		},
		"/v2/": {
			http.MethodGet:  "checkRegistrySlash",
			http.MethodHead: "headRegistrySlash",
		},
		"/v2/{name+}/blobs/{digest}": {
			http.MethodGet:  "getRegistryBlob",
			http.MethodHead: "headRegistryBlob",
		},
		"/v2/{name+}/manifests/{reference}": {
			http.MethodGet:  "getRegistryManifest",
			http.MethodHead: "headRegistryManifest",
		},
		"/v2/{name+}/referrers/{digest}": {
			http.MethodGet:  "getRegistryReferrers",
			http.MethodHead: "headRegistryReferrers",
		},
		"/v2/{name+}/tags/list": {
			http.MethodGet:  "listRegistryTags",
			http.MethodHead: "headRegistryTags",
		},
		"/version/{version}/extensions/official": {
			http.MethodGet: "listOfficialExtensions",
		},
		"/version/{version}/overlays/official": {
			http.MethodGet: "listOfficialOverlays",
		},
		"/versions": {
			http.MethodGet: "listVersions",
		},
		"/vex/{version}/vex.json": {
			http.MethodGet:  "getVEX",
			http.MethodHead: "headVEX",
		},
	}

	actualCount := 0

	for path, methods := range expected {
		pathItem := document.Paths.Find(path)
		require.NotNil(t, pathItem, "missing OpenAPI path %s", path)

		for method, operationID := range methods {
			operation := pathItem.GetOperation(method)
			require.NotNil(t, operation, "missing OpenAPI operation %s %s", method, path)
			assert.Equal(t, operationID, operation.OperationID, "%s %s", method, path)
			require.NotNil(t, operation.Responses.Value("500"), "missing 500 response for %s %s", method, path)
		}
	}

	for path := range document.Paths.Map() {
		actualCount += len(document.Paths.Value(path).Operations())
	}

	expectedCount := 0
	for _, methods := range expected {
		expectedCount += len(methods)
	}

	assert.Equal(t, expectedCount, actualCount, "the contract must not contain undocumented user-facing operations")
}
