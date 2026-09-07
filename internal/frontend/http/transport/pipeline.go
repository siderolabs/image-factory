// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package transport

import (
	"fmt"

	"github.com/julienschmidt/httprouter"
)

// HandlerBuilder assembles the request pipeline for a validated route.
type HandlerBuilder func(Route) (httprouter.Handle, error)

// Pipeline selects the explicit application, OCI, or static request path for a route.
type Pipeline struct {
	application HandlerBuilder
	oci         HandlerBuilder
	static      HandlerBuilder
}

// NewPipeline constructs a complete transport pipeline selector.
func NewPipeline(application, oci, static HandlerBuilder) (*Pipeline, error) {
	if application == nil {
		return nil, fmt.Errorf("application pipeline is required")
	}

	if oci == nil {
		return nil, fmt.Errorf("OCI pipeline is required")
	}

	if static == nil {
		return nil, fmt.Errorf("static pipeline is required")
	}

	return &Pipeline{
		application: application,
		oci:         oci,
		static:      static,
	}, nil
}

// Handler builds the protocol-specific request pipeline for route.
func (pipeline *Pipeline) Handler(route Route) (httprouter.Handle, error) {
	if pipeline == nil {
		return nil, fmt.Errorf("transport pipeline is required")
	}

	var builder HandlerBuilder

	switch route.Protocol {
	case ProtocolAPI, ProtocolHTML, ProtocolBrowserAuth, ProtocolOperational:
		builder = pipeline.application
	case ProtocolOCI:
		builder = pipeline.oci
	case ProtocolStatic:
		builder = pipeline.static
	default:
		return nil, fmt.Errorf("route %s %s has unsupported protocol %d", route.Method, route.Path, route.Protocol)
	}

	handle, err := builder(route)
	if err != nil {
		return nil, fmt.Errorf("build %s %s pipeline: %w", route.Method, route.Path, err)
	}

	if handle == nil {
		return nil, fmt.Errorf("build %s %s pipeline: handler is required", route.Method, route.Path)
	}

	return handle, nil
}
