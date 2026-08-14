// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package api provides the canonical Image Factory OpenAPI contract.
package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

//go:embed openapi.yaml
var specification []byte

var greedyPathParameter = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_.-]*)\+\}`)

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

	document, err := loader.LoadFromData(specification)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI document: %w", err)
	}

	if err = document.Validate(ctx, openapi3.EnableMultiError()); err != nil {
		return nil, fmt.Errorf("validate OpenAPI document: %w", err)
	}

	return document, nil
}

// NewContract loads the canonical document and builds its request router.
func NewContract(ctx context.Context) (*Contract, error) {
	document, err := Load(ctx)
	if err != nil {
		return nil, err
	}

	routingDocument, err := newRoutingDocument(ctx, document)
	if err != nil {
		return nil, err
	}

	router, err := gorillamux.NewRouter(routingDocument)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router: %w", err)
	}

	return &Contract{Document: document, Router: router}, nil
}

func newRoutingDocument(ctx context.Context, document *openapi3.T) (*openapi3.T, error) {
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAPI routing document: %w", err)
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

	return routingDocument, nil
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

	input := &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			MultiError:         true,
		},
	}

	if err = openapi3filter.ValidateRequest(ctx, input); err != nil {
		return route, pathParams, fmt.Errorf("validate OpenAPI request: %w", err)
	}

	return route, pathParams, nil
}
