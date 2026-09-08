// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/api"
)

// Registrar exclusively validates, assembles, and owns the runtime HTTP router.
type Registrar struct {
	contract *api.Contract
	handler  *publishedRouter
	pipeline *Pipeline

	registered map[routeKey]struct{}
	installed  []registeredRoute
	mu         sync.Mutex
}

// NewRegistrar creates a contract-aware route registrar and its exclusively owned router.
func NewRegistrar(contract *api.Contract, pipeline *Pipeline) (*Registrar, error) {
	if contract == nil {
		return nil, fmt.Errorf("OpenAPI contract is required")
	}

	if pipeline == nil {
		return nil, fmt.Errorf("transport pipeline is required")
	}

	return &Registrar{
		contract:   contract,
		handler:    newPublishedRouter(),
		pipeline:   pipeline,
		registered: map[routeKey]struct{}{},
	}, nil
}

// Handler returns the registrar-owned, atomically published HTTP handler.
func (registrar *Registrar) Handler() http.Handler {
	if registrar == nil {
		return nil
	}

	return registrar.handler
}

// Register validates and assembles every route before atomically replacing the owned router tree.
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

	candidateRoutes := make([]registeredRoute, len(registrar.installed), len(registrar.installed)+len(routes))
	copy(candidateRoutes, registrar.installed)

	for _, route := range routes {
		handle, err := registrar.pipeline.Handler(route)
		if err != nil {
			return err
		}

		candidateRoutes = append(candidateRoutes, registeredRoute{route: route, handle: handle})
	}

	candidate := httprouter.New()
	if err := installRoutes(candidate, candidateRoutes); err != nil {
		return err
	}

	registrar.handler.store(candidate)
	registrar.installed = candidateRoutes

	for key := range seen {
		registrar.registered[key] = struct{}{}
	}

	return nil
}

func installRoutes(router *httprouter.Router, routes []registeredRoute) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("conflicting runtime routes: %v", recovered)
		}
	}()

	for _, route := range routes {
		router.Handle(route.route.Method, route.route.Path, route.handle)
	}

	return nil
}

type registeredRoute struct {
	handle httprouter.Handle
	route  Route
}

type routeKey struct {
	method string
	path   string
}

type publishedRouter struct {
	current atomic.Pointer[httprouter.Router]
}

func newPublishedRouter() *publishedRouter {
	handler := &publishedRouter{}
	handler.store(httprouter.New())

	return handler
}

func (handler *publishedRouter) store(router *httprouter.Router) {
	handler.current.Store(router)
}

func (handler *publishedRouter) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.current.Load().ServeHTTP(writer, request)
}
