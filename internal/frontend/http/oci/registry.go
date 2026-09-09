// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oci

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/registry"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Registry resolves OCI operations without depending on HTTP.
type Registry interface {
	Manifest(context.Context, registry.Request) (registry.Location, error)
	Blob(context.Context, registry.Request) (registry.Location, error)
	DiscoverReferrers(context.Context, registry.Request, string) (registry.Referrers, error)
	Proxy(context.Context, registry.Request) (registry.Location, error)
}

// NewRegistryHandler binds registry application outputs to the OCI transport.
func NewRegistryHandler(service Registry, logger *zap.Logger) *Handler {
	return New(func(ctx context.Context, w http.ResponseWriter, req *http.Request, route V2Route) error {
		request := registry.Request{Image: route.Image, Schematic: route.Schematic, Reference: route.Reference, Resource: route.Resource}

		var (
			location registry.Location
			err      error
		)

		switch route.Target {
		case V2TargetPing:
			return nil
		case V2TargetManifest:
			location, err = service.Manifest(ctx, request)
		case V2TargetBlob:
			location, err = service.Blob(ctx, request)
		case V2TargetReferrers:
			artifactType := req.URL.Query().Get("artifactType")

			result, resolveErr := service.DiscoverReferrers(ctx, request, artifactType)
			if resolveErr != nil {
				return resolveErr
			}

			w.Header().Set("Content-Type", string(types.OCIImageIndex))
			w.Header().Set("Docker-Content-Digest", result.Digest.String())
			w.Header().Set("Content-Length", strconv.Itoa(len(result.Manifest)))
			ApplyReferrersFilterHeader(w.Header(), artifactType)
			_, err = w.Write(result.Manifest)

			return err
		case V2TargetProxy:
			location, err = service.Proxy(ctx, request)
		default:
			return xerrors.NewTaggedf[RouteNotFoundTag]("unknown registry route")
		}

		if err != nil {
			return err
		}

		base := url.URL{Scheme: location.Scheme, Host: location.Host, Path: "/"}
		target := base.JoinPath("v2", location.Repository, location.Resource, location.Reference)

		requestLogger := ctxlog.Logger(ctx, logger)
		if location.Proxy {
			proxyRegistryRequest(requestLogger, target, w, req)

			return nil
		}

		requestLogger.Info("redirecting manifest/blob", zap.Stringer("location", target))
		w.Header().Add("Location", target.String())
		w.WriteHeader(http.StatusTemporaryRedirect)

		return nil
	})
}

// ApplyReferrersFilterHeader reports the applied OCI discovery filter.
func ApplyReferrersFilterHeader(header http.Header, artifactType string) {
	if artifactType != "" {
		header.Set("Oci-Filters-Applied", "artifactType")
	}
}

func proxyRegistryRequest(logger *zap.Logger, location *url.URL, w http.ResponseWriter, req *http.Request) {
	logger.Info("proxying registry request", zap.Stringer("location", location))

	proxy := &httputil.ReverseProxy{
		Director: func(out *http.Request) { //nolint:staticcheck // refactor me later to use Rewrite
			out.URL.Scheme = location.Scheme
			out.URL.Host = location.Host
			out.URL.Path = location.Path

			out.URL.RawPath = ""
			if location.RawQuery != "" {
				out.URL.RawQuery = location.RawQuery
			}
			// we don't forward the host header to avoid TLS issues with some registries
			out.Host = ""
			out.Header.Del("Authorization")
		},
		Transport: remotewrap.GetTransport(),
	}
	proxy.ServeHTTP(w, req)
}
