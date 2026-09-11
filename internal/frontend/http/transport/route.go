// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package transport defines declarative HTTP transport metadata.
package transport

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/api"
)

// Handler is an HTTP endpoint which receives router parameters and reports its error to the transport pipeline.
type Handler func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error

// AccessPolicy selects authentication and authorization middleware for a route.
type AccessPolicy uint8

const (
	// AccessPublic permits anonymous requests.
	AccessPublic AccessPolicy = iota + 1
	// AccessAuthenticated requires the configured authentication provider.
	AccessAuthenticated
	// AccessImageDownload accepts credentials allowed to retrieve image artifacts.
	AccessImageDownload
)

// Protocol selects protocol-specific transport behavior for a route.
type Protocol uint8

const (
	// ProtocolAPI identifies ordinary API responses.
	ProtocolAPI Protocol = iota
	// ProtocolHTML identifies HTML and HTMX responses.
	ProtocolHTML
	// ProtocolStatic identifies static file responses.
	ProtocolStatic
	// ProtocolBrowserAuth identifies browser authentication flows.
	ProtocolBrowserAuth
	// ProtocolOperational identifies liveness and readiness endpoints.
	ProtocolOperational
	// ProtocolOCI identifies OCI Distribution responses.
	ProtocolOCI
)

// Route declares one runtime HTTP route and its canonical OpenAPI ownership.
// DispatchedOperationIDs is reserved for the /v2/*path OCI dispatcher, whose
// runtime catch-all serves a finite set of canonical operations.
type Route struct {
	Handler Handler

	Method                 string
	Path                   string
	OperationID            string
	DispatchedOperationIDs []string
	Access                 AccessPolicy
	Protocol               Protocol
}

// Validate checks that route metadata is complete and internally consistent.
func (route Route) Validate() error {
	if route.Method == "" {
		return fmt.Errorf("route method is required")
	}

	if route.Path == "" {
		return fmt.Errorf("route path is required")
	}

	if !strings.HasPrefix(route.Path, "/") {
		return fmt.Errorf("route path %q must start with a slash", route.Path)
	}

	if route.Handler == nil {
		return fmt.Errorf("route handler is required")
	}

	if route.Access < AccessPublic || route.Access > AccessImageDownload {
		return fmt.Errorf("route %s %s has unsupported access policy %d", route.Method, route.Path, route.Access)
	}

	if route.Protocol > ProtocolOCI {
		return fmt.Errorf("route %s %s has unsupported protocol %d", route.Method, route.Path, route.Protocol)
	}

	if err := route.validatePolicy(); err != nil {
		return err
	}

	return route.validateOperationOwnership()
}

// ValidateContract checks route metadata against the canonical OpenAPI contract.
func (route Route) ValidateContract(contract *api.Contract) error {
	if err := route.Validate(); err != nil {
		return err
	}

	if contract == nil {
		return fmt.Errorf("OpenAPI contract is required")
	}

	if len(route.DispatchedOperationIDs) != 0 {
		return contract.ValidateRuntimeDispatcher(route.Method, route.Path, route.DispatchedOperationIDs)
	}

	return contract.ValidateRuntimeOperation(route.Method, route.Path, route.OperationID)
}

func (route Route) validatePolicy() error {
	switch route.Protocol {
	case ProtocolStatic:
		if route.Access != AccessPublic {
			return fmt.Errorf("static routes must be public")
		}
	case ProtocolBrowserAuth:
		if route.Access != AccessPublic {
			return fmt.Errorf("browser authentication routes must be public")
		}
	case ProtocolOperational:
		if route.Access != AccessPublic {
			return fmt.Errorf("operational routes must be public")
		}
	case ProtocolOCI:
		if route.Access != AccessAuthenticated && route.Access != AccessImageDownload {
			return fmt.Errorf("OCI routes must require authenticated or image-download access")
		}
	case ProtocolAPI, ProtocolHTML:
		if route.Access == AccessImageDownload && route.Protocol != ProtocolAPI {
			return fmt.Errorf("image-download access is unsupported for protocol %d", route.Protocol)
		}
	}

	return nil
}

func (route Route) validateOperationOwnership() error {
	if route.Path == "/v2/*path" && len(route.DispatchedOperationIDs) == 0 {
		return fmt.Errorf("OCI dispatcher must declare dispatched operation IDs")
	}

	if len(route.DispatchedOperationIDs) == 0 {
		if route.OperationID == "" {
			return fmt.Errorf("route operation ID is required")
		}

		return nil
	}

	if route.Path != "/v2/*path" || route.Protocol != ProtocolOCI {
		return fmt.Errorf("dispatcher operations require an OCI dispatcher")
	}

	if route.OperationID != "" {
		return fmt.Errorf("OCI dispatcher cannot declare a single operation ID")
	}

	seen := make(map[string]struct{}, len(route.DispatchedOperationIDs))
	for _, operationID := range route.DispatchedOperationIDs {
		if operationID == "" {
			return fmt.Errorf("OCI dispatcher operation ID is required")
		}

		if _, duplicate := seen[operationID]; duplicate {
			return fmt.Errorf("OCI dispatcher operation ID %q is duplicated", operationID)
		}

		seen[operationID] = struct{}{}
	}

	return nil
}
