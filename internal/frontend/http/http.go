// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/gen/ensure"
	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/audit"
	applicationapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/metadata"
	"github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/frontend/http/operational"
	staticfiles "github.com/siderolabs/image-factory/internal/frontend/http/static"
	"github.com/siderolabs/image-factory/internal/frontend/http/ui"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/registry"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/secureboot"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	server           *Server
	contract         *api.Contract
	schematicAPI     *applicationapi.SchematicHandler
	imageAPI         *applicationapi.ImageHandler
	pxeAPI           *applicationapi.PXEHandler
	talosctlAPI      *applicationapi.TalosctlHandler
	browserAuth      *browserauth.Handler
	metadata         *metadata.Handler
	oci              *oci.Handler
	operational      *operational.Handler
	staticCSS        *staticfiles.Handler
	staticFavicons   *staticfiles.Handler
	staticJavaScript *staticfiles.Handler
	ui               *ui.Handler
}

// Options configures the HTTP frontend.
type Options struct {
	ImageProxy          ImageProxyOptions
	CacheImageSigner    signer.Signer
	InstallerSBOMSource enterprise.SPDXSource
	AuthProvider        enterprise.AuthProvider
	TokenVerifier       enterprise.TokenVerifier

	ExternalURL                      *url.URL
	ExternalPXEURL                   *url.URL
	AuditSink                        audit.Sink
	InstallerInternalRepository      name.Repository
	InstallerInternalNameOptions     []name.Option
	InstallerExternalRepository      name.Repository
	MetricsNamespace                 string
	AllowedOrigins                   []string
	RemoteOptions                    []remote.Option
	RegistryRefreshInterval          time.Duration
	ProxyInstallerInternalRepository bool
}

type ImageProxyOptions struct {
	Images          map[string]string
	BackingRegistry name.Registry
	Namespace       string
}

// NewFrontend creates a new HTTP frontend.
func NewFrontend(
	ctx context.Context,
	logger *zap.Logger,
	schematicFactory *schematic.Factory,
	assetBuilder *asset.Builder,
	artifactsManager *artifacts.Manager,
	secureBootService *secureboot.Service,
	checksummer enterprise.Checksummer,
	signatureWriter enterprise.SignatureWriter,
	enterprisePlugins []enterprise.FrontendPlugin,
	opts Options,
) (*Frontend, error) {
	logger = logger.With(zap.String("frontend", "http"))
	frontend := &Frontend{}
	schematicService := schematic.NewService(schematicFactory, opts.AuthProvider != nil, enterprise.Enabled())
	frontend.schematicAPI = applicationapi.NewSchematicHandler(schematicService)
	imageService := asset.NewImageService(
		schematicService,
		assetBuilder,
		asset.NewImageProfileEnhancer(artifactsManager, secureBootService),
		asset.ImageServiceOptions{
			ChecksumGenerator:  checksummer,
			SignatureGenerator: signatureWriter,
			Logger:             logger,
		},
	)
	frontend.imageAPI = applicationapi.NewImageHandler(
		imageService,
		applicationapi.ImageHandlerOptions{
			ExternalPXEURL: opts.ExternalPXEURL,
			Logger:         logger,
		},
	)
	frontend.pxeAPI = applicationapi.NewPXEHandler(
		imageService,
		applicationapi.PXEHandlerOptions{
			ExternalPXEURL: opts.ExternalPXEURL,
			AuthEnabled:    opts.AuthProvider != nil,
		},
	)
	frontend.talosctlAPI = applicationapi.NewTalosctlHandler(
		artifacts.NewTalosctlService(artifactsManager),
		opts.ExternalURL,
	)
	frontend.browserAuth = browserauth.New(opts.AuthProvider)
	frontend.ui = ui.New(schematicService, artifactsManager, ui.Options{
		ExternalURL:    opts.ExternalURL,
		ExternalPXEURL: opts.ExternalPXEURL,
		AuthProvider:   opts.AuthProvider,
		TokensEnabled:  opts.TokenVerifier != nil,
		LogoutEnabled:  frontend.browserAuth.LogoutEnabled(),
	})

	var readinessCheckers []operational.ReadinessChecker

	for _, plugin := range enterprisePlugins {
		if checker, ok := plugin.(enterprise.ReadinessChecker); ok {
			readinessCheckers = append(readinessCheckers, checker)
		}
	}

	frontend.initializeEndpointOwners(readinessCheckers)

	var err error

	var contractOptions []api.ContractOption
	if provider, ok := opts.AuthProvider.(enterprise.BrowserLoginProvider); ok && provider.BrowserLoginEnabled() {
		contractOptions = append(contractOptions, api.WithBrowserCallbackPath(provider.CallbackPath()))
	}

	frontend.contract, err = api.NewContract(ctx, contractOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAPI contract: %w", err)
	}

	puller, err := remotewrap.NewPuller(opts.RegistryRefreshInterval, opts.InstallerInternalNameOptions, opts.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create puller: %w", err)
	}

	pusher, err := remotewrap.NewPusher(opts.RegistryRefreshInterval, opts.InstallerInternalNameOptions, opts.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create pusher: %w", err)
	}

	imageSigner := opts.CacheImageSigner
	frontend.metadata = metadata.New(artifactsManager, secureBootService, imageSigner, getLLMsTxt)

	evidencePublisher, err := enterprise.NewInstallerEvidencePublisher(
		logger,
		imageSigner,
		opts.InstallerSBOMSource,
		pusher,
		puller,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Installer evidence publisher: %w", err)
	}

	registryService := registry.New(
		registrySchematics{factory: schematicFactory, provider: opts.AuthProvider},
		artifactsManager,
		assetBuilder,
		func(ctx context.Context, prof talosprofile.Profile, configuration *schematicpkg.Schematic, versionTag string) (profile.EnhancementResult, error) {
			return profile.EnhanceFromSchematicWithDependencies(ctx, prof, configuration, artifactsManager, secureBootService, versionTag)
		},
		registry.Options{
			InternalRepository: opts.InstallerInternalRepository,
			ExternalRepository: opts.InstallerExternalRepository,
			ProxyInternal:      opts.ProxyInstallerInternalRepository,
			AuthEnabled:        opts.AuthProvider != nil,
			ProxyImages:        opts.ImageProxy.Images,
			ProxyRegistry:      opts.ImageProxy.BackingRegistry,
			ProxyNamespace:     opts.ImageProxy.Namespace,
			Puller:             puller,
			Pusher:             pusher,
			Signer:             imageSigner,
			EvidencePublisher:  evidencePublisher,
			Logger:             logger,
		},
	)
	frontend.oci = oci.NewRegistryHandler(registryService, logger)

	middleware := NewRequestMiddleware(logger, frontend.contract, opts.AuthProvider, opts.TokenVerifier, opts.AuditSink)
	if err = frontend.registerRoutes(enterprisePlugins, middleware, ServerOptions{
		AllowedOrigins:   opts.AllowedOrigins,
		MetricsNamespace: opts.MetricsNamespace,
	}); err != nil {
		return nil, fmt.Errorf("register HTTP routes: %w", err)
	}

	return frontend, nil
}

func (f *Frontend) initializeEndpointOwners(readinessCheckers []operational.ReadinessChecker) {
	f.operational = operational.New(readinessCheckers...)
	f.staticCSS = staticfiles.New(http.FS(ensure.Value(fs.Sub(cssFS, "css"))))
	f.staticFavicons = staticfiles.New(http.FS(ensure.Value(fs.Sub(faviconsFS, "favicons"))))
	f.staticJavaScript = staticfiles.New(http.FS(ensure.Value(fs.Sub(jsFS, "js"))))
}

// Handler returns the HTTP handler.
func (f *Frontend) Handler() http.Handler {
	return f.server.Handler()
}
