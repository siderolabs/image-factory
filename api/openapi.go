// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package api provides the canonical Image Factory OpenAPI contract.
package api

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

//go:embed openapi.yaml
var specification []byte

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

// NewRouter builds a router from the canonical OpenAPI contract.
func NewRouter(ctx context.Context) (routers.Router, error) {
	document, err := Load(ctx)
	if err != nil {
		return nil, err
	}

	router, err := gorillamux.NewRouter(document)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router: %w", err)
	}

	return router, nil
}
