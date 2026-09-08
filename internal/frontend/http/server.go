// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	httproutermiddleware "github.com/slok/go-http-metrics/middleware/httprouter"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

// ServerOptions configures HTTP server middleware.
type ServerOptions struct {
	MetricsNamespace string
	AllowedOrigins   []string
}

// Server owns HTTP router assembly and the final handler chain.
type Server struct {
	handler http.Handler
}

// NewServer constructs a contract-backed HTTP server.
func NewServer(
	contract *api.Contract,
	routes []transport.Route,
	applicationBuilder transport.HandlerBuilder,
	options ServerOptions,
) (*Server, error) {
	return newServer(contract, routes, applicationBuilder, options)
}

func newServer(
	contract *api.Contract,
	routes []transport.Route,
	applicationBuilder transport.HandlerBuilder,
	options ServerOptions,
) (*Server, error) {
	if applicationBuilder == nil {
		return nil, fmt.Errorf("application handler builder is required")
	}

	monitoring := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{Prefix: options.MetricsNamespace}),
	})

	instrumentedApplication := func(route transport.Route) (httprouter.Handle, error) {
		handle, err := applicationBuilder(route)
		if err != nil {
			return nil, err
		}

		if handle == nil {
			return nil, fmt.Errorf("application handler is required")
		}

		return httproutermiddleware.Handler(route.Path, handle, monitoring), nil
	}

	staticPipeline := func(route transport.Route) (httprouter.Handle, error) {
		return func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
			if err := route.Handler(request.Context(), writer, request, params); err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
			}
		}, nil
	}

	pipeline, err := transport.NewPipeline(instrumentedApplication, instrumentedApplication, staticPipeline)
	if err != nil {
		return nil, err
	}

	registrar, err := transport.NewRegistrar(contract, pipeline)
	if err != nil {
		return nil, err
	}

	if err = registrar.Register(routes); err != nil {
		return nil, err
	}

	server := &Server{}
	server.handler = cors.New(cors.Options{
		AllowedOrigins: options.AllowedOrigins,
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodOptions,
		},
		AllowedHeaders: []string{"Cache-Control"},
		ExposedHeaders: []string{"Content-Disposition", "Content-Length", "Content-Type"},
	}).Handler(registrar.Handler())

	return server, nil
}

// Handler returns the assembled HTTP handler.
func (server *Server) Handler() http.Handler {
	return server.handler
}
