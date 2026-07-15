// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
)

// RouteNotFoundTag is an error tag for registry paths that match no known route.
type RouteNotFoundTag struct{}

type v2Target int

const (
	// v2TargetPing is the OCI base check (GET /v2/).
	v2TargetPing v2Target = iota
	// v2TargetManifest is a schematic manifest: /v2/<image>/<schematic>/manifests/<tag>.
	v2TargetManifest
	// v2TargetBlob is a schematic blob: /v2/<image>/<schematic>/blobs/<digest>.
	v2TargetBlob
	// v2TargetProxy is the image proxy: /v2/siderolabs/<image>/{manifests|blobs}/...,
	// the tag listing /v2/siderolabs/<image>/tags/list, or the referrers API
	// /v2/siderolabs/<image>/referrers/<digest>.
	v2TargetProxy
)

type v2Route struct {
	image     string
	schematic string
	resource  string
	reference string
	target    v2Target
}

func routeV2(path string) (v2Route, error) {
	notFound := func() (v2Route, error) {
		return v2Route{}, xerrors.NewTaggedf[RouteNotFoundTag]("unknown registry path: %q", path)
	}

	trimmed := strings.Trim(path, "/")

	// GET /v2 health check
	if trimmed == "" {
		return v2Route{target: v2TargetPing}, nil
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

		return v2Route{
			target:    v2TargetProxy,
			image:     name,
			resource:  resource,
			reference: reference,
		}, nil
	}

	// Schematic image: only manifest/blob pulls, repository name is exactly
	// "<image>/<schematic>".
	if resource != "manifests" && resource != "blobs" {
		return notFound()
	}

	if len(repo) != 2 {
		return notFound()
	}

	target := v2TargetManifest
	if resource == "blobs" {
		target = v2TargetBlob
	}

	return v2Route{
		target:    target,
		image:     repo[0],
		schematic: repo[1],
		resource:  resource,
		reference: reference,
	}, nil
}

// handleV2 is the catch-all entry point for /v2/ registry requests.
// Either serves an image via image factory or proxies the request to the backing image repository.
func (f *Frontend) handleV2(ctx context.Context, w http.ResponseWriter, req *http.Request, p httprouter.Params) error {
	route, err := routeV2(p.ByName("path"))
	if err != nil {
		return err
	}

	switch route.target {
	case v2TargetPing:
		// always healthy :)
		return nil
	case v2TargetManifest:
		return f.handleManifest(ctx, w, req, route)
	case v2TargetBlob:
		return f.handleBlob(ctx, w, req, route)
	case v2TargetProxy:
		return f.handleImageProxy(ctx, w, req, route)
	default:
		return xerrors.NewTaggedf[RouteNotFoundTag]("unknown registry route")
	}
}
