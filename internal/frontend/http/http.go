// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-service/internal/asset"
	flvr "github.com/siderolabs/image-service/internal/flavor"
	"github.com/siderolabs/image-service/internal/flavor/storage"
	"github.com/siderolabs/image-service/internal/profile"
	"github.com/siderolabs/image-service/pkg/flavor"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	router        *httprouter.Router
	flavorService *flvr.Service
	assetBuilder  *asset.Builder
	logger        *zap.Logger
	puller        *remote.Puller
	pusher        *remote.Pusher
	sf            singleflight.Group
	options       Options
}

// Options configures the HTTP frontend.
type Options struct {
	ExternalURL *url.URL

	InstallerInternalRepository name.Repository
	InstallerExternalRepository name.Repository

	RemoteOptions []remote.Option
}

// NewFrontend creates a new HTTP frontend.
func NewFrontend(logger *zap.Logger, flavorService *flvr.Service, assetBuilder *asset.Builder, opts Options) (*Frontend, error) {
	frontend := &Frontend{
		router:        httprouter.New(),
		flavorService: flavorService,
		assetBuilder:  assetBuilder,
		logger:        logger.With(zap.String("frontend", "http")),
		options:       opts,
	}

	var err error

	frontend.puller, err = remote.NewPuller(opts.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create puller: %w", err)
	}

	frontend.pusher, err = remote.NewPusher(opts.RemoteOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create pusher: %w", err)
	}

	// images
	frontend.router.GET("/image/:flavor/:version/:path", frontend.wrapper(frontend.handleImage))
	frontend.router.HEAD("/image/:flavor/:version/:path", frontend.wrapper(frontend.handleImage))

	// PXE
	frontend.router.GET("/pxe/:flavor/:version/:path", frontend.wrapper(frontend.handlePXE))

	// registry
	frontend.router.GET("/v2", frontend.wrapper(frontend.handleHealth))
	frontend.router.HEAD("/v2", frontend.wrapper(frontend.handleHealth))
	frontend.router.GET("/healthz", frontend.wrapper(frontend.handleHealth))
	frontend.router.HEAD("/healthz", frontend.wrapper(frontend.handleHealth))
	frontend.router.GET("/v2/:image/:flavor/blobs/:digest", frontend.wrapper(frontend.handleBlob))
	frontend.router.HEAD("/v2/:image/:flavor/blobs/:digest", frontend.wrapper(frontend.handleBlob))
	frontend.router.GET("/v2/:image/:flavor/manifests/:tag", frontend.wrapper(frontend.handleManifest))
	frontend.router.HEAD("/v2/:image/:flavor/manifests/:tag", frontend.wrapper(frontend.handleManifest))

	// flavor
	frontend.router.POST("/flavor", frontend.wrapper(frontend.handleFlavorCreate))

	return frontend, nil
}

// Handler returns the HTTP handler.
func (f *Frontend) Handler() http.Handler {
	return f.router
}

func (f *Frontend) wrapper(h func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := r.Context()

		err := h(ctx, w, r, p)

		f.logger.Info("request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Error(err))

		switch {
		case err == nil:
			// happy case
		case xerrors.TagIs[storage.ErrNotFoundTag](err):
			http.Error(w, err.Error(), http.StatusNotFound)
		case xerrors.TagIs[profile.InvalidErrorTag](err),
			xerrors.TagIs[flavor.InvalidErrorTag](err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, context.Canceled):
			// client closed connection
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
