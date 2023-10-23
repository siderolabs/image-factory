// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	router           *httprouter.Router
	schematicFactory *schematic.Factory
	assetBuilder     *asset.Builder
	artifactsManager *artifacts.Manager
	logger           *zap.Logger
	puller           *remote.Puller
	pusher           *remote.Pusher
	imageSigner      *signer.Signer
	sf               singleflight.Group
	options          Options
}

// Options configures the HTTP frontend.
type Options struct {
	ExternalURL *url.URL

	InstallerInternalRepository name.Repository
	InstallerExternalRepository name.Repository

	CacheSigningKey crypto.PrivateKey

	RemoteOptions []remote.Option
}

// NewFrontend creates a new HTTP frontend.
func NewFrontend(logger *zap.Logger, schematicFactory *schematic.Factory, assetBuilder *asset.Builder, artifactsManager *artifacts.Manager, opts Options) (*Frontend, error) {
	frontend := &Frontend{
		router:           httprouter.New(),
		schematicFactory: schematicFactory,
		assetBuilder:     assetBuilder,
		artifactsManager: artifactsManager,
		logger:           logger.With(zap.String("frontend", "http")),
		options:          opts,
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

	frontend.imageSigner, err = signer.NewSigner(opts.CacheSigningKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create image signer: %w", err)
	}

	// images
	frontend.router.GET("/image/:schematic/:version/:path", frontend.wrapper(frontend.handleImage))
	frontend.router.HEAD("/image/:schematic/:version/:path", frontend.wrapper(frontend.handleImage))

	// PXE
	frontend.router.GET("/pxe/:schematic/:version/:path", frontend.wrapper(frontend.handlePXE))

	// registry
	frontend.router.GET("/v2", frontend.wrapper(frontend.handleHealth))
	frontend.router.HEAD("/v2", frontend.wrapper(frontend.handleHealth))
	frontend.router.GET("/healthz", frontend.wrapper(frontend.handleHealth))
	frontend.router.HEAD("/healthz", frontend.wrapper(frontend.handleHealth))
	frontend.router.GET("/v2/:image/:schematic/blobs/:digest", frontend.wrapper(frontend.handleBlob))
	frontend.router.HEAD("/v2/:image/:schematic/blobs/:digest", frontend.wrapper(frontend.handleBlob))
	frontend.router.GET("/v2/:image/:schematic/manifests/:tag", frontend.wrapper(frontend.handleManifest))
	frontend.router.HEAD("/v2/:image/:schematic/manifests/:tag", frontend.wrapper(frontend.handleManifest))
	frontend.router.GET("/oci/cosign/signing-key.pub", frontend.wrapper(frontend.handleCosignSigningKeyPub))

	// schematic
	frontend.router.POST("/schematics", frontend.wrapper(frontend.handleSchematicCreate))

	// meta
	frontend.router.GET("/versions", frontend.wrapper(frontend.handleVersions))
	frontend.router.GET("/version/:version/extensions/official", frontend.wrapper(frontend.handleOfficialExtensions))

	// UI
	frontend.router.GET("/", frontend.wrapper(frontend.handleUI))
	frontend.router.HEAD("/", frontend.wrapper(frontend.handleUI))
	frontend.router.GET("/ui/schematic-config", frontend.wrapper(frontend.handleUISchematicConfig))
	frontend.router.GET("/ui/versions", frontend.wrapper(frontend.handleUIVersions))
	frontend.router.POST("/ui/schematics", frontend.wrapper(frontend.handleUISchematics))
	frontend.router.ServeFiles("/css/*filepath", http.FS(ensure.Value(fs.Sub(cssFS, "css"))))
	frontend.router.ServeFiles("/favicons/*filepath", http.FS(ensure.Value(fs.Sub(faviconsFS, "favicons"))))
	frontend.router.ServeFiles("/js/*filepath", http.FS(ensure.Value(fs.Sub(jsFS, "js"))))

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
			xerrors.TagIs[schematicpkg.InvalidErrorTag](err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, context.Canceled):
			// client closed connection
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}
