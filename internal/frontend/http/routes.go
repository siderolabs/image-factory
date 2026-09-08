// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func (f *Frontend) registerRoutes(enterprisePlugins []enterprise.FrontendPlugin) error {
	routes, err := f.enterpriseRoutes(enterprisePlugins)
	if err != nil {
		return err
	}

	routes = append(routes, f.routes()...)
	routes = append(routes, f.oci.Routes()...)
	routes = append(routes, f.browserAuth.Routes()...)
	routes = append(routes, f.ui.Routes()...)

	server, err := newServer(f.contract, routes, f.buildApplicationHandler, ServerOptions{
		AllowedOrigins:   f.options.AllowedOrigins,
		MetricsNamespace: f.options.MetricsNamespace,
	})
	if err != nil {
		return err
	}

	f.server = server

	return nil
}

func (f *Frontend) buildApplicationHandler(route transport.Route) (httprouter.Handle, error) {
	switch route.Access {
	case transport.AccessPublic, transport.AccessAuthenticated, transport.AccessImageDownload:
		return f.wrapHandlerProtocolAccess(route.Handler, route.Access, route.Protocol), nil
	default:
		return nil, fmt.Errorf("route %s %s has unsupported access policy %d", route.Method, route.Path, route.Access)
	}
}

func (f *Frontend) enterpriseRoutes(plugins []enterprise.FrontendPlugin) ([]transport.Route, error) {
	var routes []transport.Route

	for _, plugin := range plugins {
		for _, route := range plugin.Routes() {
			access, err := enterpriseAccessPolicy(route.Access)
			if err != nil {
				return nil, fmt.Errorf("enterprise route %s %s: %w", route.Method, route.Path, err)
			}

			routes = append(routes, transport.Route{
				Method:      route.Method,
				Path:        route.Path,
				OperationID: route.OperationID,
				Access:      access,
				Protocol:    transport.ProtocolAPI,
				Handler:     route.Handler,
			})
		}
	}

	return routes, nil
}

func enterpriseAccessPolicy(policy enterprise.RouteAccessPolicy) (transport.AccessPolicy, error) {
	switch policy {
	case enterprise.RouteAccessPublic:
		return transport.AccessPublic, nil
	case enterprise.RouteAccessAuthenticated:
		return transport.AccessAuthenticated, nil
	default:
		return 0, fmt.Errorf("unsupported access policy %d", policy)
	}
}

func (f *Frontend) routes() []transport.Route {
	return []transport.Route{
		{Method: http.MethodGet, Path: "/healthz", OperationID: "getHealth", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.operational.Health},
		{Method: http.MethodHead, Path: "/healthz", OperationID: "headHealth", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.operational.Health},
		{Method: http.MethodGet, Path: "/readyz", OperationID: "getReadiness", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.operational.Ready},
		{Method: http.MethodHead, Path: "/readyz", OperationID: "headReadiness", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.operational.Ready},

		{
			Method:      http.MethodGet,
			Path:        "/image/:schematic/:version/:path",
			OperationID: "getImage",
			Access:      transport.AccessImageDownload,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.imageAPI.Serve,
		},
		{
			Method:      http.MethodHead,
			Path:        "/image/:schematic/:version/:path",
			OperationID: "headImage",
			Access:      transport.AccessImageDownload,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.imageAPI.Serve,
		},
		{
			Method:      http.MethodGet,
			Path:        "/pxe/:schematic/:version/:path",
			OperationID: "getPXEScript",
			Access:      transport.AccessImageDownload,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.pxeAPI.Serve,
		},

		{
			Method:      http.MethodGet,
			Path:        "/oci/cosign/signing-key.pub",
			OperationID: "getCosignSigningKey",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.metadata.CosignSigningKey,
		},

		{Method: http.MethodPost, Path: "/schematics", OperationID: "createSchematic", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolAPI, Handler: f.schematicAPI.Create},
		{Method: http.MethodGet, Path: "/schematics/:schematic", OperationID: "getSchematic", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolAPI, Handler: f.schematicAPI.Get},

		{Method: http.MethodGet, Path: "/versions", OperationID: "listVersions", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.metadata.Versions},
		{
			Method:      http.MethodGet,
			Path:        "/version/:version/extensions/official",
			OperationID: "listOfficialExtensions",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.metadata.OfficialExtensions,
		},
		{
			Method:      http.MethodGet,
			Path:        "/version/:version/overlays/official",
			OperationID: "listOfficialOverlays",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.metadata.OfficialOverlays,
		},
		{
			Method:      http.MethodGet,
			Path:        "/secureboot/signing-cert.pem",
			OperationID: "getSecureBootSigningCertificate",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.metadata.SecureBootSigningCertificate,
		},
		{Method: http.MethodGet, Path: "/talosctl/:version", OperationID: "listTalosctlDownloads", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.talosctlAPI.List},
		{Method: http.MethodHead, Path: "/talosctl/:version/:path", OperationID: "headTalosctl", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.talosctlAPI.Download},
		{Method: http.MethodGet, Path: "/talosctl/:version/:path", OperationID: "getTalosctl", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.talosctlAPI.Download},
		{Method: http.MethodGet, Path: "/llms.txt", OperationID: "getLLMsText", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.metadata.LLMsText},
		{Method: http.MethodGet, Path: "/openapi.yaml", OperationID: "getOpenAPI", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleOpenAPI},

		{
			Method:      http.MethodGet,
			Path:        "/css/*filepath",
			OperationID: "getCSSAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     f.staticCSS.Serve,
		},
		{
			Method:      http.MethodGet,
			Path:        "/favicons/*filepath",
			OperationID: "getFaviconAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     f.staticFavicons.Serve,
		},
		{
			Method:      http.MethodGet,
			Path:        "/js/*filepath",
			OperationID: "getJavaScriptAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     f.staticJavaScript.Serve,
		},
	}
}
