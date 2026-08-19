// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/apitoken"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "3.1.0", document.OpenAPI)
	assert.Equal(t, "https://spec.openapis.org/oas/3.1/dialect/2024-11-10", document.JSONSchemaDialect)
	require.NotNil(t, document.Info)
	assert.Equal(t, "Image Factory API", document.Info.Title)
	require.NotNil(t, document.Paths)

	versions := document.Paths.Find("/versions")
	require.NotNil(t, versions)
	require.NotNil(t, versions.Get)
	assert.Equal(t, "listVersions", versions.Get.OperationID)
}

func TestSourceSpecificationIsModular(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("openapi.yaml")
	require.NoError(t, err)

	assert.Contains(t, string(source), `$ref: "./openapi/paths/`)
	assert.Contains(t, string(source), `$ref: "./openapi/components/`)
	assert.NotContains(t, string(source), "operationId:")
}

func TestSpecificationIsSelfContained(t *testing.T) {
	t.Parallel()

	specification, err := api.Specification()
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(specification), `$ref: "./openapi/`))

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	document, err := loader.LoadFromData(specification)
	require.NoError(t, err)
	require.NoError(t, document.Validate(t.Context(), openapi3.EnableMultiError()))

	secondSpecification, err := api.Specification()
	require.NoError(t, err)
	assert.Equal(t, specification, secondSpecification)

	specification[0] ^= 0xff

	thirdSpecification, err := api.Specification()
	require.NoError(t, err)
	assert.Equal(t, secondSpecification, thirdSpecification)
}

func TestDocumentationSchemas(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	schematic := document.Components.Schemas["Schematic"].Value
	require.NotNil(t, schematic)
	assert.NotEmpty(t, schematic.Description)
	assert.NotEmpty(t, schematic.Examples)

	versions := document.Components.Schemas["VersionList"].Value
	require.NotNil(t, versions)
	require.NotNil(t, versions.Items)
	require.NotNil(t, versions.Items.Value)
	assert.NotEmpty(t, versions.Items.Value.Pattern)
	assert.NotEmpty(t, versions.Examples)

	parameters := document.Components.Parameters
	assert.NotEmpty(t, parameters["TalosVersion"].Value.Schema.Value.Pattern)
	assert.NotNil(t, parameters["TalosVersion"].Value.Example)
	assert.NotNil(t, parameters["ArtifactPath"].Value.Example)
	assert.Equal(t, `.+\.(json|table|sarif|cdx)$`, parameters["ScanReport"].Value.Schema.Value.Pattern)
	assert.NotNil(t, parameters["RegistryNameTail"].Value.Example)
	assert.NotEmpty(t, parameters["OCIReference"].Value.Schema.Value.OneOf)
	assert.NotEmpty(t, parameters["OCIDigest"].Value.Schema.Value.Pattern)

	assert.Nil(t, document.Paths.Find("/download-token"))
	assert.NotContains(t, document.Components.Schemas, "DownloadToken")
	assert.NotContains(t, document.Components.Parameters, "DownloadToken")
	assert.NotContains(t, document.Components.SecuritySchemes, "downloadToken")
	assert.NotContains(t, document.Components.Parameters, "APIToken")
	assert.Contains(t, document.Components.SecuritySchemes, "apiTokenQuery")

	callback := document.Paths.Find("/callback").Get
	require.NotNil(t, callback)
	assert.NotNil(t, callback.Responses.Value("403"))
	assert.Nil(t, callback.Responses.Value("400"))
}

func TestAuthenticationContract(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	require.Contains(t, document.Components.SecuritySchemes, "sessionCookie")
	assertSecurityAlternatives(t, document.Security, "basicAuth", "bearerAuth", "sessionCookie")
	assert.Len(t, document.Security, 3)

	basic := document.Components.SecuritySchemes["basicAuth"].Value
	require.NotNil(t, basic)
	assert.Contains(t, basic.Description, "API token")

	bearer := document.Components.SecuritySchemes["bearerAuth"].Value
	require.NotNil(t, bearer)
	assert.Contains(t, bearer.Description, "API token")

	unauthorized := document.Components.Responses["Unauthorized"].Value
	require.NotNil(t, unauthorized)

	challenge := unauthorized.Headers["WWW-Authenticate"].Value
	require.NotNil(t, challenge)
	assert.Contains(t, challenge.Description, "Basic")
	assert.Contains(t, challenge.Description, "never a Bearer challenge")
	require.NotNil(t, challenge.Schema)
	require.NotNil(t, challenge.Schema.Value)
	assert.Equal(t, `Basic realm="Image Factory Enterprise", charset="UTF-8"`, challenge.Schema.Value.Example)

	htmxRedirect := unauthorized.Headers["Hx-Redirect"].Value
	require.NotNil(t, htmxRedirect)
	assert.Contains(t, htmxRedirect.Description, "mutually exclusive")

	expectedTokenScopes := map[string][]string{
		"createAPIToken":        {"token:issue"},
		"listAPITokens":         {"token:read"},
		"revokeAPIToken":        {"token:revoke"},
		"createSchematic":       {"schematic:create"},
		"getSchematic":          {"schematic:read"},
		"getImage":              {"image:read"},
		"headImage":             {"image:read"},
		"getPXEScript":          {"image:read"},
		"getSPDX":               {"report:read"},
		"headSPDX":              {"report:read"},
		"getVEX":                {"report:read"},
		"headVEX":               {"report:read"},
		"getVulnerabilityScan":  {"report:read"},
		"headVulnerabilityScan": {"report:read"},
		"checkRegistry":         {"image:read", "source:pull"},
		"headRegistry":          {"image:read", "source:pull"},
		"checkRegistrySlash":    {"image:read", "source:pull"},
		"headRegistrySlash":     {"image:read", "source:pull"},
		"getRegistryManifest":   {"image:read", "source:pull"},
		"headRegistryManifest":  {"image:read", "source:pull"},
		"getRegistryBlob":       {"image:read", "source:pull"},
		"headRegistryBlob":      {"image:read", "source:pull"},
		"listRegistryTags":      {"image:read", "source:pull"},
		"headRegistryTags":      {"image:read", "source:pull"},
		"getRegistryReferrers":  {"image:read", "source:pull"},
		"headRegistryReferrers": {"image:read", "source:pull"},
	}

	seenTokenOperations := map[string]struct{}{}

	for path, pathItem := range document.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			assert.NotContains(t, operation.Extensions, "x-enterprise", "%s %s", method, path)

			security := document.Security
			if operation.Security != nil {
				security = *operation.Security
			}

			if len(security) == 0 {
				assert.NotContains(t, operation.Extensions, "x-image-factory-api-token-scopes", "%s %s", method, path)

				continue
			}

			assertSecurityAlternatives(t, security, "basicAuth", "bearerAuth", "sessionCookie")
			require.NotNil(t, operation.Responses.Value("401"), "missing 401 response for %s %s", method, path)
			require.NotNil(t, operation.Responses.Value("303"), "missing browser-login redirect for %s %s", method, path)

			expectedScopes, acceptsAPIToken := expectedTokenScopes[operation.OperationID]
			if !acceptsAPIToken {
				assert.NotContains(t, operation.Extensions, "x-image-factory-api-token-scopes", "%s %s", method, path)
				assert.False(t, hasSecurityAlternative(security, "apiTokenQuery"), "%s %s", method, path)

				continue
			}

			seenTokenOperations[operation.OperationID] = struct{}{}

			actualScopes, ok := operation.Extensions["x-image-factory-api-token-scopes"].([]any)
			require.True(t, ok, "missing API-token scopes for %s %s", method, path)

			actualScopeNames := make([]string, 0, len(actualScopes))
			for _, scope := range actualScopes {
				scopeName, ok := scope.(string)
				require.True(t, ok, "non-string API-token scope on %s %s", method, path)
				assert.True(t, apitoken.Valid(scopeName), "unknown API-token scope on %s %s", method, path)
				actualScopeNames = append(actualScopeNames, scopeName)
			}

			assert.ElementsMatch(t, expectedScopes, actualScopeNames, "%s %s", method, path)

			for _, scope := range apitoken.Scopes() {
				acceptedByRuntime := false
				for _, requestPath := range materializeOpenAPIPath(path) {
					acceptedByRuntime = acceptedByRuntime || apitoken.Allows([]apitoken.Scope{scope}, strings.ToUpper(method), requestPath)
				}

				assert.Equal(t, slices.Contains(actualScopeNames, scope), acceptedByRuntime,
					"runtime/OpenAPI scope mismatch for %s %s and %s", method, path, scope)
			}

			querySafe := (method == http.MethodGet || method == http.MethodHead) && apitoken.URLSafe(actualScopeNames)
			assert.Equal(t, querySafe, hasSecurityAlternative(security, "apiTokenQuery"), "%s %s", method, path)
		}
	}

	assert.Len(t, seenTokenOperations, len(expectedTokenScopes))
}

func assertSecurityAlternatives(t *testing.T, security openapi3.SecurityRequirements, names ...string) {
	t.Helper()

	for _, requirement := range security {
		assert.Len(t, requirement, 1, "authentication mechanisms must be alternatives")
	}

	for _, name := range names {
		assert.True(t, hasSecurityAlternative(security, name), "missing %s security alternative", name)
	}
}

func hasSecurityAlternative(security openapi3.SecurityRequirements, name string) bool {
	for _, requirement := range security {
		if _, ok := requirement[name]; ok {
			return true
		}
	}

	return false
}

func materializeOpenAPIPath(path string) []string {
	paths := []string{path}
	if strings.Contains(path, "{name+}") {
		paths = []string{
			strings.ReplaceAll(path, "{name+}", "metal-installer/schematic"),
			strings.ReplaceAll(path, "{name+}", "siderolabs/installer"),
		}
	}

	replacer := strings.NewReplacer(
		"{schematic}", "schematic",
		"{version}", "v1.0.0",
		"{path}", "metal-amd64.raw.xz",
		"{arch}", "amd64",
		"{report}", "report.json",
		"{id}", "token-id",
		"{reference}", "v1.0.0",
		"{digest}", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)

	for index := range paths {
		paths[index] = replacer.Replace(paths[index])
	}

	return paths
}

func TestContractOperations(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	expected := map[string]map[string]string{
		"/": {
			http.MethodGet:  "getUI",
			http.MethodHead: "headUI",
		},
		"/.well-known/jwks.json": {
			http.MethodGet: "getAPITokenJWKS",
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
		"/callback": {
			http.MethodGet: "completeBrowserLogin",
		},
		"/css/{filepath+}": {
			http.MethodGet: "getCSSAsset",
		},
		"/favicons/{filepath+}": {
			http.MethodGet: "getFaviconAsset",
		},
		"/healthz": {
			http.MethodGet:  "getHealth",
			http.MethodHead: "headHealth",
		},
		"/js/{filepath+}": {
			http.MethodGet: "getJavaScriptAsset",
		},
		"/login": {
			http.MethodGet: "startBrowserLogin",
		},
		"/logout": {
			http.MethodGet:  "getBrowserLogout",
			http.MethodPost: "postBrowserLogout",
		},
		"/readyz": {
			http.MethodGet:  "getReadiness",
			http.MethodHead: "headReadiness",
		},
		"/ui/extensions-list": {
			http.MethodPost: "postUIExtensionsList",
		},
		"/ui/version-doc": {
			http.MethodGet: "getUIVersionDocumentation",
		},
		"/ui/wizard": {
			http.MethodPost: "postUIWizard",
		},
		"/ui/tokens": {
			http.MethodGet: "getUITokens",
		},
		"/tokens": {
			http.MethodGet:  "listAPITokens",
			http.MethodPost: "createAPIToken",
		},
		"/tokens/{id}/revoke": {
			http.MethodPost: "revokeAPIToken",
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

	assert.Equal(t, expectedCount, actualCount, "the contract operation inventory must be explicit")
}

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

func TestContractValidatesRuntimeRoutes(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/image/:schematic/:version/:path"},
		{method: http.MethodGet, path: "/v2/*path"},
		{method: http.MethodHead, path: "/v2/*path"},
		{method: http.MethodGet, path: "/css/*filepath"},
		{method: http.MethodPost, path: "/ui/wizard"},
		{method: http.MethodGet, path: "/callback"},
	} {
		require.NoError(t, contract.ValidateRuntimeRoute(test.method, test.path), "%s %s", test.method, test.path)
	}

	require.ErrorContains(t, contract.ValidateRuntimeRoute(http.MethodGet, "/future-route"), "is not declared in OpenAPI")
	require.ErrorContains(t, contract.ValidateRuntimeRoute(http.MethodGet, "/spdx/*path"), "is not declared in OpenAPI")
	require.ErrorContains(t, contract.ValidateRuntimeRoute(http.MethodGet, "/image/:id/:version/:path"), "is not declared in OpenAPI")
	require.ErrorContains(t, contract.ValidateRuntimeRoute(http.MethodDelete, "/image/:schematic/:version/:path"), "is not declared in OpenAPI")
	require.ErrorContains(t, contract.ValidateRuntimeRoute(http.MethodDelete, "/v2/*path"), "is not declared in OpenAPI")

	contract.Document.Paths.Find("/v2/{name+}/tags/list").Head = nil
	require.ErrorContains(
		t,
		contract.ValidateRuntimeRoute(http.MethodHead, "/v2/*path"),
		"requires OpenAPI operation HEAD /v2/{name+}/tags/list",
	)
}

func TestArtifactResponseAcceptsRuntimeMediaTypes(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	artifact := document.Components.Responses["Artifact"].Value
	require.NotNil(t, artifact)

	for _, mediaType := range []string{
		"*/*",
		"application/efi",
		"application/gzip",
		"application/octet-stream",
		"application/x-iso9660-image",
		"application/x-qemu-disk",
		"application/x-xz",
		"text/plain",
	} {
		require.Contains(t, artifact.Content, mediaType)
	}
}

func TestContractValidateRequest(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	tests := []struct {
		name    string
		method  string
		target  string
		body    string
		headers map[string]string
		wantErr string
	}{
		{
			name:   "valid schematic",
			method: http.MethodPost,
			target: "/schematics",
			body:   `{"customization":{"extraKernelArgs":["console=ttyS0"]}}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "handler-owned schematic body validation",
			method: http.MethodPost,
			target: "/schematics",
			body:   `{"unknown":true}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "handler-owned schematic path validation",
			method: http.MethodGet,
			target: "/schematics/not-a-digest",
		},
		{
			name:   "greedy OCI repository name",
			method: http.MethodGet,
			target: "/v2/my-company/platform/backend/manifests/v1.12.0",
		},
		{
			name:   "API token list",
			method: http.MethodGet,
			target: "/tokens",
		},
		{
			name:   "API token creation",
			method: http.MethodPost,
			target: "/tokens",
			body:   `{"name":"production-cluster","actor":"talos"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "handler-owned API token body validation without a name",
			method: http.MethodPost,
			target: "/tokens",
			body:   `{}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "handler-owned API token body validation with arbitrary content type",
			method: http.MethodPost,
			target: "/tokens",
			body:   `{"name":"production-cluster","actor":"talos"}`,
			headers: map[string]string{
				"Content-Type": "application/x-custom-json",
			},
		},
		{
			name:   "handler-owned API token body size validation",
			method: http.MethodPost,
			target: "/tokens",
			body:   strings.Repeat("x", 1<<13),
		},
		{
			// the page's JS sends no body and no content type for a revocation
			name:   "API token revocation",
			method: http.MethodPost,
			target: "/tokens/01K3S2QW1V6P1Z0J9YQK4M8H7B/revoke",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(t.Context(), test.method, test.target, strings.NewReader(test.body))
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}

			_, _, validationErr := contract.ValidateRequest(request.Context(), request)
			if test.wantErr == "" {
				require.NoError(t, validationErr)

				return
			}

			require.ErrorContains(t, validationErr, test.wantErr)
		})
	}
}

func TestContractLeavesArtifactPathValidationToImageHandler(t *testing.T) {
	t.Parallel()

	contract, err := api.NewContract(t.Context())
	require.NoError(t, err)

	const prefix = "/image/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/v1.12.0/"

	tests := []struct {
		path string
	}{
		{path: "kernel-amd64"},
		{path: "kernel-arm64"},
		{path: "kernel-x86_64"},
		{path: "kernel-i386"},
		{path: "cmdline-metal-amd64"},
		{path: "cmdline-digital-ocean-arm64-secureboot"},
		{path: "initramfs-amd64.xz"},
		{path: "metal-amd64.iso"},
		{path: "metal-arm64-secureboot.iso"},
		{path: "metal-amd64-secureboot-uki.efi"},
		{path: "installer-amd64.tar"},
		{path: "installer-arm64-secureboot.tar"},
		{path: "aws-installer-amd64.tar"},
		{path: "digital-ocean-installer-arm64-secureboot.tar"},
		{path: "metal-amd64.raw"},
		{path: "metal-amd64.raw.xz"},
		{path: "metal-amd64.raw.zst"},
		{path: "gcp-amd64.raw.tar.gz"},
		{path: "digital-ocean-arm64.raw.gz"},
		{path: "foo.bar-amd64.raw"},
		{path: "aws-amd64.qcow2"},
		{path: "aws-amd64.qcow2.xz"},
		{path: "azure-amd64.vhd"},
		{path: "vmware-amd64.ova"},
		{path: "metal-amd64.raw.xz.sha256"},
		{path: "metal-amd64.iso.sha512"},
		{path: "kernel-amd64.sigstore.json"},
		{path: "not-an-artifact"},
		{path: "kernel-riscv64"},
		{path: "metal-amd64.img"},
		{path: "metal-amd64.raw.bz2"},
		{path: "metal-amd64.raw.xz.sha256sum"},
		{path: "metal-amd64.raw.xz.sha256.sigstore.json"},
		{path: "kernel-amd64.iso"},
		{path: "cmdline-amd64.raw"},
		{path: "installer-installer-amd64.tar"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			for _, method := range []string{http.MethodGet, http.MethodHead} {
				request := httptest.NewRequestWithContext(t.Context(), method, prefix+test.path, nil)
				_, _, validationErr := contract.ValidateRequest(request.Context(), request)

				require.NoError(t, validationErr, method)
			}
		})
	}

	for name, test := range map[string]struct {
		target  string
		methods []string
	}{
		"PXE profile": {
			target:  "/pxe/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/v1.12.0/not-an-image-profile",
			methods: []string{http.MethodGet},
		},
		"talosctl binary": {
			target:  "/talosctl/v1.12.0/custom-binary-name",
			methods: []string{http.MethodGet, http.MethodHead},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, method := range test.methods {
				request := httptest.NewRequestWithContext(t.Context(), method, test.target, nil)
				_, _, validationErr := contract.ValidateRequest(request.Context(), request)
				require.NoError(t, validationErr, method)
			}
		})
	}
}
