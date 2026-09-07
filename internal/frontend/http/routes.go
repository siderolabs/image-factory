// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	httproutermiddleware "github.com/slok/go-http-metrics/middleware/httprouter"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func (f *Frontend) registerRoutes(router *httprouter.Router, enterprisePlugins []enterprise.FrontendPlugin) error {
	monitoring := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{
			Prefix: f.options.MetricsNamespace,
		}),
	})

	applicationPipeline := func(route transport.Route) (httprouter.Handle, error) {
		var handle httprouter.Handle

		switch route.Access {
		case transport.AccessPublic:
			handle = f.wrapHandlerProtocol(route.Handler, false, route.Protocol)
		case transport.AccessAuthenticated, transport.AccessImageDownload:
			handle = f.wrapHandlerProtocol(route.Handler, true, route.Protocol)
		default:
			return nil, fmt.Errorf("route %s %s has unsupported access policy %d", route.Method, route.Path, route.Access)
		}

		return httproutermiddleware.Handler(route.Path, handle, monitoring), nil
	}

	ociPipeline := func(route transport.Route) (httprouter.Handle, error) {
		return applicationPipeline(route)
	}

	staticPipeline := func(route transport.Route) (httprouter.Handle, error) {
		return func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
			if err := route.Handler(request.Context(), writer, request, params); err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
			}
		}, nil
	}

	pipeline, err := transport.NewPipeline(applicationPipeline, ociPipeline, staticPipeline)
	if err != nil {
		return err
	}

	registrar, err := transport.NewRegistrar(f.contract, router, pipeline)
	if err != nil {
		return err
	}

	routes := append(f.enterpriseRoutes(enterprisePlugins), f.routes()...)
	routes = append(routes, f.browserLoginRoutes()...)

	return registrar.Register(routes)
}

func (f *Frontend) enterpriseRoutes(plugins []enterprise.FrontendPlugin) []transport.Route {
	var routes []transport.Route

	for _, plugin := range plugins {
		for _, route := range plugin.Routes() {
			routes = append(routes, transport.Route{
				Method:      route.Method,
				Path:        route.Path,
				OperationID: route.OperationID,
				Access:      enterpriseAccessPolicy(route.Access),
				Protocol:    transport.ProtocolAPI,
				Handler:     route.Handler,
			})
		}
	}

	return routes
}

func enterpriseAccessPolicy(policy enterprise.RouteAccessPolicy) transport.AccessPolicy {
	switch policy {
	case enterprise.RouteAccessPublic:
		return transport.AccessPublic
	case enterprise.RouteAccessAuthenticated:
		return transport.AccessAuthenticated
	default:
		return 0
	}
}

func (f *Frontend) routes() []transport.Route {
	return []transport.Route{
		{Method: http.MethodGet, Path: "/healthz", OperationID: "getHealth", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.handleHealth},
		{Method: http.MethodHead, Path: "/healthz", OperationID: "headHealth", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.handleHealth},
		{Method: http.MethodGet, Path: "/readyz", OperationID: "getReadiness", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.handleReady},
		{Method: http.MethodHead, Path: "/readyz", OperationID: "headReadiness", Access: transport.AccessPublic, Protocol: transport.ProtocolOperational, Handler: f.handleReady},

		{Method: http.MethodGet, Path: "/image/:schematic/:version/:path", OperationID: "getImage", Access: transport.AccessImageDownload, Protocol: transport.ProtocolAPI, Handler: f.handleImage},
		{Method: http.MethodHead, Path: "/image/:schematic/:version/:path", OperationID: "headImage", Access: transport.AccessImageDownload, Protocol: transport.ProtocolAPI, Handler: f.handleImage},
		{Method: http.MethodGet, Path: "/pxe/:schematic/:version/:path", OperationID: "getPXEScript", Access: transport.AccessImageDownload, Protocol: transport.ProtocolAPI, Handler: f.handlePXE},

		{Method: http.MethodGet, Path: "/v2", OperationID: "checkRegistry", Access: transport.AccessImageDownload, Protocol: transport.ProtocolOCI, Handler: f.handleHealth},
		{Method: http.MethodHead, Path: "/v2", OperationID: "headRegistry", Access: transport.AccessImageDownload, Protocol: transport.ProtocolOCI, Handler: f.handleHealth},
		{
			Method:   http.MethodGet,
			Path:     "/v2/*path",
			Access:   transport.AccessImageDownload,
			Protocol: transport.ProtocolOCI,
			Handler:  f.handleV2,
			DispatchedOperationIDs: []string{
				"checkRegistrySlash",
				"getRegistryManifest",
				"getRegistryBlob",
				"listRegistryTags",
				"getRegistryReferrers",
			},
		},
		{
			Method:   http.MethodHead,
			Path:     "/v2/*path",
			Access:   transport.AccessImageDownload,
			Protocol: transport.ProtocolOCI,
			Handler:  f.handleV2,
			DispatchedOperationIDs: []string{
				"headRegistrySlash",
				"headRegistryManifest",
				"headRegistryBlob",
				"headRegistryTags",
				"headRegistryReferrers",
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/oci/cosign/signing-key.pub",
			OperationID: "getCosignSigningKey",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.handleCosignSigningKeyPub,
		},

		{Method: http.MethodPost, Path: "/schematics", OperationID: "createSchematic", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolAPI, Handler: f.handleSchematicCreate},
		{Method: http.MethodGet, Path: "/schematics/:schematic", OperationID: "getSchematic", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolAPI, Handler: f.handleSchematicGet},

		{Method: http.MethodGet, Path: "/versions", OperationID: "listVersions", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleVersions},
		{
			Method:      http.MethodGet,
			Path:        "/version/:version/extensions/official",
			OperationID: "listOfficialExtensions",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.handleOfficialExtensions,
		},
		{
			Method:      http.MethodGet,
			Path:        "/version/:version/overlays/official",
			OperationID: "listOfficialOverlays",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.handleOfficialOverlays,
		},
		{
			Method:      http.MethodGet,
			Path:        "/secureboot/signing-cert.pem",
			OperationID: "getSecureBootSigningCertificate",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolAPI,
			Handler:     f.handleSecureBootSigningCert,
		},
		{Method: http.MethodGet, Path: "/talosctl/:version", OperationID: "listTalosctlDownloads", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleTalosctlList},
		{Method: http.MethodHead, Path: "/talosctl/:version/:path", OperationID: "headTalosctl", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleTalosctl},
		{Method: http.MethodGet, Path: "/talosctl/:version/:path", OperationID: "getTalosctl", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleTalosctl},
		{Method: http.MethodGet, Path: "/llms.txt", OperationID: "getLLMsText", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleLLMsTxt},
		{Method: http.MethodGet, Path: "/openapi.yaml", OperationID: "getOpenAPI", Access: transport.AccessPublic, Protocol: transport.ProtocolAPI, Handler: f.handleOpenAPI},

		{Method: http.MethodGet, Path: "/", OperationID: "getUI", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: f.handleUI},
		{Method: http.MethodHead, Path: "/", OperationID: "headUI", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: f.handleUI},
		{Method: http.MethodPost, Path: "/ui/wizard", OperationID: "postUIWizard", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: f.handleUIWizard},
		{
			Method:      http.MethodGet,
			Path:        "/ui/version-doc",
			OperationID: "getUIVersionDocumentation",
			Access:      transport.AccessAuthenticated,
			Protocol:    transport.ProtocolHTML,
			Handler:     f.handleUIVersionDoc,
		},
		{
			Method:      http.MethodPost,
			Path:        "/ui/extensions-list",
			OperationID: "postUIExtensionsList",
			Access:      transport.AccessAuthenticated,
			Protocol:    transport.ProtocolHTML,
			Handler:     f.handleUIExtensionsList,
		},
		{Method: http.MethodGet, Path: "/ui/tokens", OperationID: "getUITokens", Access: transport.AccessAuthenticated, Protocol: transport.ProtocolHTML, Handler: f.handleTokensUI},

		{
			Method:      http.MethodGet,
			Path:        "/css/*filepath",
			OperationID: "getCSSAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     serveFiles(http.FS(ensure.Value(fs.Sub(cssFS, "css")))),
		},
		{
			Method:      http.MethodGet,
			Path:        "/favicons/*filepath",
			OperationID: "getFaviconAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     serveFiles(http.FS(ensure.Value(fs.Sub(faviconsFS, "favicons")))),
		},
		{
			Method:      http.MethodGet,
			Path:        "/js/*filepath",
			OperationID: "getJavaScriptAsset",
			Access:      transport.AccessPublic,
			Protocol:    transport.ProtocolStatic,
			Handler:     serveFiles(http.FS(ensure.Value(fs.Sub(jsFS, "js")))),
		},
	}
}

func serveFiles(filesystem http.FileSystem) transport.Handler {
	server := http.FileServer(filesystem)

	return func(_ context.Context, writer http.ResponseWriter, request *http.Request, params httprouter.Params) error {
		request.URL.Path = params.ByName("filepath")
		server.ServeHTTP(writer, request)

		return nil
	}
}
