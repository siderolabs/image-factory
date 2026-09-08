// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oci implements OCI Distribution route parsing and dispatch.
package oci

import (
	"context"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
)

// RouteNotFoundTag marks frontend and registry paths that match no known route.
type RouteNotFoundTag = transport.RouteNotFoundTag

// MethodNotAllowedTag marks requests whose path exists but method is not declared.
type MethodNotAllowedTag = transport.MethodNotAllowedTag

// V2Target identifies the operation selected for an OCI Distribution route.
type V2Target int

const (
	// V2TargetPing is the OCI base check (GET /v2/).
	V2TargetPing V2Target = iota
	// V2TargetManifest is a schematic manifest: /v2/<image>/<schematic>/manifests/<tag>.
	V2TargetManifest
	// V2TargetBlob is a schematic blob: /v2/<image>/<schematic>/blobs/<digest>.
	V2TargetBlob
	// V2TargetReferrers is the OCI referrers API for a generated schematic image.
	V2TargetReferrers
	// V2TargetProxy is the image proxy: /v2/siderolabs/<image>/{manifests|blobs}/...,
	// the tag listing /v2/siderolabs/<image>/tags/list, or the referrers API
	// /v2/siderolabs/<image>/referrers/<digest>.
	V2TargetProxy
)

// V2Route is a parsed OCI Distribution route.
type V2Route struct {
	Image     string
	Schematic string
	Resource  string
	Reference string
	Target    V2Target
}

// RouteV2 parses a path below /v2/ into a registry route.
func RouteV2(path string) (V2Route, error) {
	notFound := func() (V2Route, error) {
		return V2Route{}, xerrors.NewTaggedf[RouteNotFoundTag]("unknown registry path: %q", path)
	}

	trimmed := strings.Trim(path, "/")

	// GET /v2 health check
	if trimmed == "" {
		return V2Route{Target: V2TargetPing}, nil
	}

	segments := strings.Split(trimmed, "/")

	// Every OCI registry path ends in "<resource>/<reference>" where resource is
	// "manifests" or "blobs"; everything before that is the repository name, which
	// must have at least one component.
	if len(segments) < 3 {
		return notFound()
	}

	reference := segments[len(segments)-1]
	resource := segments[len(segments)-2]
	repo := segments[:len(segments)-2]

	if reference == "" {
		return notFound()
	}

	// siderolabs proxy path: forward manifest/blob pulls, the tags/list endpoint,
	// and the referrers API straight to the backing registry.
	if repo[0] == "siderolabs" {
		switch {
		case resource == "manifests" || resource == "blobs":
			// pull by tag or digest

		case resource == "tags" && reference == "list":
			// list tags

		case resource == "referrers":
			// OCI referrers API, used to discover signature/attestation bundles

		default:
			return notFound()
		}

		name := strings.Join(repo[1:], "/")
		if name == "" {
			return notFound()
		}

		return V2Route{
			Target:    V2TargetProxy,
			Image:     name,
			Resource:  resource,
			Reference: reference,
		}, nil
	}

	// Schematic image: manifest/blob pulls and OCI referrer discovery, repository name is exactly
	// "<image>/<schematic>".
	if resource != "manifests" && resource != "blobs" && resource != "referrers" {
		return notFound()
	}

	if len(repo) != 2 {
		return notFound()
	}

	target := V2TargetManifest

	switch resource {
	case "blobs":
		target = V2TargetBlob
	case "referrers":
		target = V2TargetReferrers
	}

	return V2Route{
		Target:    target,
		Image:     repo[0],
		Schematic: repo[1],
		Resource:  resource,
		Reference: reference,
	}, nil
}

// ServeFunc dispatches a parsed non-ping OCI request to registry application behavior.
type ServeFunc func(context.Context, http.ResponseWriter, *http.Request, V2Route) error

// Handler owns OCI Distribution routes and path dispatch.
type Handler struct {
	serve ServeFunc
}

// New creates an OCI Distribution adapter.
func New(serve ServeFunc) *Handler {
	return &Handler{serve: serve}
}

// Routes returns the OCI Distribution routes owned by the adapter.
func (handler *Handler) Routes() []transport.Route {
	return []transport.Route{
		{
			Method:      http.MethodGet,
			Path:        "/v2",
			OperationID: "checkRegistry",
			Access:      transport.AccessImageDownload,
			Protocol:    transport.ProtocolOCI,
			Handler:     handler.ping,
		},
		{
			Method:      http.MethodHead,
			Path:        "/v2",
			OperationID: "headRegistry",
			Access:      transport.AccessImageDownload,
			Protocol:    transport.ProtocolOCI,
			Handler:     handler.ping,
		},
		{
			Method:   http.MethodGet,
			Path:     "/v2/*path",
			Access:   transport.AccessImageDownload,
			Protocol: transport.ProtocolOCI,
			Handler:  handler.servePath,
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
			Handler:  handler.servePath,
			DispatchedOperationIDs: []string{
				"headRegistrySlash",
				"headRegistryManifest",
				"headRegistryBlob",
				"headRegistryTags",
				"headRegistryReferrers",
			},
		},
	}
}

func (*Handler) ping(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error {
	return nil
}

// servePath is the catch-all entry point for /v2/ registry requests.
func (handler *Handler) servePath(ctx context.Context, w http.ResponseWriter, req *http.Request, p httprouter.Params) error {
	route, err := RouteV2(p.ByName("path"))
	if err != nil {
		return err
	}

	if route.Target == V2TargetPing {
		return nil
	}

	if handler.serve == nil {
		return xerrors.NewTaggedf[RouteNotFoundTag]("unknown registry route")
	}

	return handler.serve(ctx, w, req, route)
}
