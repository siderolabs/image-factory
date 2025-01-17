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
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	httproutermiddleware "github.com/slok/go-http-metrics/middleware/httprouter"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/skyssolutions/siderolabs-image-factory/internal/artifacts"
	"github.com/skyssolutions/siderolabs-image-factory/internal/asset"
	"github.com/skyssolutions/siderolabs-image-factory/internal/image/signer"
	"github.com/skyssolutions/siderolabs-image-factory/internal/profile"
	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic"
	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic/storage"
	"github.com/skyssolutions/siderolabs-image-factory/internal/secureboot"
	schematicpkg "github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	router            *httprouter.Router
	schematicFactory  *schematic.Factory
	assetBuilder      *asset.Builder
	artifactsManager  *artifacts.Manager
	secureBootService *secureboot.Service
	logger            *zap.Logger
	puller            *remote.Puller
	pusher            *remote.Pusher
	imageSigner       *signer.Signer
	sf                singleflight.Group
	options           Options
}

// Options configures the HTTP frontend.
type Options struct {
	ExternalURL    *url.URL
	ExternalPXEURL *url.URL

	InstallerInternalRepository name.Repository
	InstallerExternalRepository name.Repository

	CacheSigningKey crypto.PrivateKey

	RemoteOptions []remote.Option
}

// NewFrontend creates a new HTTP frontend.
func NewFrontend(
	logger *zap.Logger,
	schematicFactory *schematic.Factory,
	assetBuilder *asset.Builder,
	artifactsManager *artifacts.Manager,
	secureBootService *secureboot.Service,
	opts Options,
) (*Frontend, error) {
	frontend := &Frontend{
		router:            httprouter.New(),
		schematicFactory:  schematicFactory,
		assetBuilder:      assetBuilder,
		artifactsManager:  artifactsManager,
		secureBootService: secureBootService,
		logger:            logger.With(zap.String("frontend", "http")),
		options:           opts,
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

	// monitoring middleware
	mdlw := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})

	registerRoute := func(registrator func(string, httprouter.Handle), path string, handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error) {
		registrator(path, httproutermiddleware.Handler(path, frontend.wrapper(handler), mdlw))
	}

	// images
	registerRoute(frontend.router.GET, "/image/:schematic/:version/:path", frontend.handleImage)
	registerRoute(frontend.router.HEAD, "/image/:schematic/:version/:path", frontend.handleImage)

	// PXE
	registerRoute(frontend.router.GET, "/pxe/:schematic/:version/:path", frontend.handlePXE)

	// registry
	registerRoute(frontend.router.GET, "/v2", frontend.handleHealth)
	registerRoute(frontend.router.HEAD, "/v2", frontend.handleHealth)
	registerRoute(frontend.router.GET, "/healthz", frontend.handleHealth)
	registerRoute(frontend.router.HEAD, "/healthz", frontend.handleHealth)
	registerRoute(frontend.router.GET, "/v2/:image/:schematic/blobs/:digest", frontend.handleBlob)
	registerRoute(frontend.router.HEAD, "/v2/:image/:schematic/blobs/:digest", frontend.handleBlob)
	registerRoute(frontend.router.GET, "/v2/:image/:schematic/manifests/:tag", frontend.handleManifest)
	registerRoute(frontend.router.HEAD, "/v2/:image/:schematic/manifests/:tag", frontend.handleManifest)
	registerRoute(frontend.router.GET, "/oci/cosign/signing-key.pub", frontend.handleCosignSigningKeyPub)

	// schematic
	registerRoute(frontend.router.POST, "/schematics", frontend.handleSchematicCreate)

	// meta
	registerRoute(frontend.router.GET, "/versions", frontend.handleVersions)
	registerRoute(frontend.router.GET, "/version/:version/extensions/official", frontend.handleOfficialExtensions)
	registerRoute(frontend.router.GET, "/version/:version/overlays/official", frontend.handleOfficialOverlays)

	// secureboot
	registerRoute(frontend.router.GET, "/secureboot/signing-cert.pem", frontend.handleSecureBootSigningCert)

	// UI
	registerRoute(frontend.router.GET, "/", frontend.handleUI)
	registerRoute(frontend.router.HEAD, "/", frontend.handleUI)
	registerRoute(frontend.router.POST, "/ui/wizard", frontend.handleUIWizard)
	registerRoute(frontend.router.GET, "/ui/version-doc", frontend.handleUIVersionDoc)
	registerRoute(frontend.router.POST, "/ui/extensions-list", frontend.handleUIExtensionsList)
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
