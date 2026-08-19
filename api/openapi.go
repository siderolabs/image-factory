// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package api provides the canonical Image Factory OpenAPI contract.
package api

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"go.yaml.in/yaml/v4"
)

//go:embed openapi.yaml openapi/*/*.yaml
var specificationFS embed.FS

var bundledSpecification = sync.OnceValues(bundleSpecification)

var greedyPathParameter = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_.-]*)\+\}`)

// Specification returns an isolated, self-contained copy of the canonical OpenAPI YAML document.
func Specification() ([]byte, error) {
	specification, err := bundledSpecification()
	if err != nil {
		return nil, err
	}

	return bytes.Clone(specification), nil
}

// Contract binds the canonical document to its request router and validator.
type Contract struct {
	Document *openapi3.T
	Router   routers.Router
}

// Load parses and validates the embedded OpenAPI document.
func Load(ctx context.Context) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.Context = ctx
	loader.IncludeOrigin = true
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = readSpecificationResource

	document, err := loader.LoadFromFile("openapi.yaml")
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI document: %w", err)
	}

	if err = document.Validate(ctx, openapi3.EnableMultiError()); err != nil {
		return nil, fmt.Errorf("validate OpenAPI document: %w", err)
	}

	return document, nil
}

func readSpecificationResource(_ *openapi3.Loader, location *url.URL) ([]byte, error) {
	if location.Scheme != "" || location.Host != "" {
		return nil, fmt.Errorf("read embedded OpenAPI resource: %w", openapi3.ErrURINotSupported)
	}

	name := path.Clean(strings.TrimPrefix(location.Path, "/"))
	if name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return nil, fmt.Errorf("invalid embedded OpenAPI resource path %q", location.Path)
	}

	data, err := specificationFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read embedded OpenAPI resource %q: %w", name, err)
	}

	return data, nil
}

func bundleSpecification() ([]byte, error) {
	document, err := Load(context.Background())
	if err != nil {
		return nil, err
	}

	document.InternalizeRefs(context.Background(), nil)

	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal bundled OpenAPI document: %w", err)
	}

	var value any

	if err = yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode bundled OpenAPI document: %w", err)
	}

	data, err = yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode bundled OpenAPI document: %w", err)
	}

	return data, nil
}

// NewContract loads the canonical document and builds its request router.
func NewContract(ctx context.Context) (*Contract, error) {
	document, err := Load(ctx)
	if err != nil {
		return nil, err
	}

	routingDocument, err := newRoutingDocument(ctx)
	if err != nil {
		return nil, err
	}

	router, err := gorillamux.NewRouter(routingDocument)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router: %w", err)
	}

	return &Contract{Document: document, Router: router}, nil
}

func newRoutingDocument(ctx context.Context) (*openapi3.T, error) {
	data, err := Specification()
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	loader.Context = ctx

	routingDocument, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI routing document: %w", err)
	}

	for path, pathItem := range routingDocument.Paths.Map() {
		matches := greedyPathParameter.FindAllStringSubmatch(path, -1)
		if len(matches) == 0 {
			continue
		}

		routingPath := greedyPathParameter.ReplaceAllString(path, `{$1:.+}`)

		for _, match := range matches {
			normalizeGreedyParameter(pathItem, match[1])
		}

		routingDocument.Paths.Delete(path)
		routingDocument.Paths.Set(routingPath, pathItem)
	}

	relaxRoutingPathValidation(routingDocument)

	return routingDocument, nil
}

func relaxRoutingPathValidation(document *openapi3.T) {
	relax := func(parameters openapi3.Parameters) {
		for _, parameter := range parameters {
			if parameter.Value != nil &&
				parameter.Value.In == openapi3.ParameterInPath {
				parameter.Value.Schema = &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}
			}
		}
	}

	for _, pathItem := range document.Paths.Map() {
		relax(pathItem.Parameters)

		for _, operation := range pathItem.Operations() {
			relax(operation.Parameters)
		}
	}
}

func normalizeGreedyParameter(pathItem *openapi3.PathItem, name string) {
	normalize := func(parameters openapi3.Parameters) {
		for _, parameter := range parameters {
			if parameter.Value != nil && parameter.Value.Name == name+"+" {
				parameter.Value.Name = name
			}
		}
	}

	normalize(pathItem.Parameters)

	for _, operation := range pathItem.Operations() {
		normalize(operation.Parameters)
	}
}

// NewRouter builds a router from the canonical OpenAPI contract.
func NewRouter(ctx context.Context) (routers.Router, error) {
	contract, err := NewContract(ctx)
	if err != nil {
		return nil, err
	}

	return contract.Router, nil
}

// ValidateRuntimeRoute verifies that a runtime router pattern maps exactly to an
// operation declared by the canonical OpenAPI contract. The OCI dispatcher is
// the sole intentional catch-all and is checked against every operation it serves.
func (contract *Contract) ValidateRuntimeRoute(method, runtimePath string) error {
	if runtimePath == "/v2/*path" {
		return contract.validateRegistryDispatcher(method)
	}

	contractPath := runtimeContractPath(runtimePath)

	pathItem := contract.Document.Paths.Map()[contractPath]
	if pathItem != nil && pathItem.GetOperation(method) != nil {
		return nil
	}

	return fmt.Errorf("runtime route %s %s is not declared in OpenAPI", method, runtimePath)
}

func (contract *Contract) validateRegistryDispatcher(method string) error {
	var contractPaths []string

	switch method {
	case http.MethodGet, http.MethodHead:
		contractPaths = []string{
			"/v2/",
			"/v2/{name+}/manifests/{reference}",
			"/v2/{name+}/blobs/{digest}",
			"/v2/{name+}/tags/list",
			"/v2/{name+}/referrers/{digest}",
		}
	default:
		return fmt.Errorf("runtime route %s /v2/*path is not declared in OpenAPI", method)
	}

	for _, contractPath := range contractPaths {
		pathItem := contract.Document.Paths.Map()[contractPath]
		if pathItem == nil || pathItem.GetOperation(method) == nil {
			return fmt.Errorf("runtime route %s /v2/*path requires OpenAPI operation %s %s", method, method, contractPath)
		}
	}

	return nil
}

func runtimeContractPath(runtimePath string) string {
	segments := strings.Split(runtimePath, "/")

	for index, segment := range segments {
		switch {
		case strings.HasPrefix(segment, ":"):
			segments[index] = "{" + strings.TrimPrefix(segment, ":") + "}"
		case strings.HasPrefix(segment, "*"):
			segments[index] = "{" + strings.TrimPrefix(segment, "*") + "+}"
		}
	}

	return strings.Join(segments, "/")
}

// ValidateRequest matches and validates a request against the contract.
// Authentication remains the responsibility of the HTTP frontend.
func (contract *Contract) ValidateRequest(
	ctx context.Context,
	request *http.Request,
) (*routers.Route, map[string]string, error) {
	route, pathParams, err := contract.Router.FindRoute(request)
	if err != nil {
		return nil, nil, fmt.Errorf("match OpenAPI route: %w", err)
	}

	handlerValidatesBody := route.Operation.Extensions["x-image-factory-handler-validates-body"] == true

	ignoreContentType := route.Operation.Extensions["x-image-factory-ignore-content-type"] == true
	if !handlerValidatesBody && (ignoreContentType || request.Header.Get("Content-Type") == "") {
		if route.Operation.RequestBody != nil && route.Operation.RequestBody.Value != nil {
			if _, ok := route.Operation.RequestBody.Value.Content["application/yaml"]; ok {
				originalHeader := request.Header.Clone()
				request.Header.Set("Content-Type", "application/yaml")

				defer func() {
					request.Header = originalHeader
				}()
			}
		}
	}

	input := &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc:  openapi3filter.NoopAuthenticationFunc,
			ExcludeRequestBody:  handlerValidatesBody,
			MultiError:          true,
			SkipSettingDefaults: true,
		},
	}

	if err = openapi3filter.ValidateRequest(ctx, input); err != nil {
		return route, pathParams, fmt.Errorf("validate OpenAPI request: %w", err)
	}

	return route, pathParams, nil
}
