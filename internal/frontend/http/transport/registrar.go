// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport

import (
	"fmt"
	"sync"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/api"
)

// Registrar exclusively validates and installs declarative routes into an HTTP router.
type Registrar struct {
	contract *api.Contract
	router   *httprouter.Router
	pipeline *Pipeline

	registered map[routeKey]struct{}
	mu         sync.Mutex
}

// NewRegistrar creates a contract-aware route registrar.
func NewRegistrar(contract *api.Contract, router *httprouter.Router, pipeline *Pipeline) (*Registrar, error) {
	if contract == nil {
		return nil, fmt.Errorf("OpenAPI contract is required")
	}

	if router == nil {
		return nil, fmt.Errorf("HTTP router is required")
	}

	if pipeline == nil {
		return nil, fmt.Errorf("transport pipeline is required")
	}

	return &Registrar{
		contract:   contract,
		router:     router,
		pipeline:   pipeline,
		registered: map[routeKey]struct{}{},
	}, nil
}

// Register validates and assembles every route before mutating the router.
func (registrar *Registrar) Register(routes []Route) error {
	if registrar == nil {
		return fmt.Errorf("transport registrar is required")
	}

	registrar.mu.Lock()
	defer registrar.mu.Unlock()

	seen := make(map[routeKey]struct{}, len(routes))
	for _, route := range routes {
		if err := route.ValidateContract(registrar.contract); err != nil {
			return fmt.Errorf("validate runtime route %s %s: %w", route.Method, route.Path, err)
		}

		key := routeKey{method: route.Method, path: route.Path}
		if _, duplicate := registrar.registered[key]; duplicate {
			return fmt.Errorf("duplicate runtime route %s %s", route.Method, route.Path)
		}

		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate runtime route %s %s", route.Method, route.Path)
		}

		seen[key] = struct{}{}
	}

	handles := make([]httprouter.Handle, len(routes))
	for index, route := range routes {
		handle, err := registrar.pipeline.Handler(route)
		if err != nil {
			return err
		}

		handles[index] = handle
	}

	for index, route := range routes {
		registrar.router.Handle(route.Method, route.Path, handles[index])
		registrar.registered[routeKey{method: route.Method, path: route.Path}] = struct{}{}
	}

	return nil
}

type routeKey struct {
	method string
	path   string
}
