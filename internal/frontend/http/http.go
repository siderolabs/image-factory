// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package http implements the HTTP frontend.
package http

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"time"

	"github.com/getkin/kin-openapi/routers"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	applicationapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/authentication"
	"github.com/siderolabs/image-factory/internal/frontend/http/browserauth"
	"github.com/siderolabs/image-factory/internal/frontend/http/metadata"
	"github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/frontend/http/operational"
	staticfiles "github.com/siderolabs/image-factory/internal/frontend/http/static"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/frontend/http/ui"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/secureboot"
	"github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	server            *Server
	contract          *api.Contract
	schematicAPI      *applicationapi.SchematicHandler
	imageAPI          *applicationapi.ImageHandler
	pxeAPI            *applicationapi.PXEHandler
	talosctlAPI       *applicationapi.TalosctlHandler
	browserAuth       *browserauth.Handler
	schematicFactory  *schematic.Factory
	assetBuilder      *asset.Builder
	artifactsManager  *artifacts.Manager
	secureBootService *secureboot.Service
	checksummer       enterprise.Checksummer
	signatureWriter   enterprise.SignatureWriter
	logger            *zap.Logger
	puller            remotewrap.Puller
	pusher            remotewrap.Pusher
	imageSigner       signer.Signer
	evidencePublisher enterprise.InstallerEvidencePublisher
	metadata          *metadata.Handler
	oci               *oci.Handler
	operational       *operational.Handler
	staticCSS         *staticfiles.Handler
	staticFavicons    *staticfiles.Handler
	staticJavaScript  *staticfiles.Handler
	ui                *ui.Handler

	options Options
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

// Handler is a custom handler type that includes the context and httprouter params, and returns an error.
type Handler = transport.Handler

// InvalidRequestTag marks requests rejected by the OpenAPI contract.
type InvalidRequestTag = transport.InvalidRequestTag

// RouteNotFoundTag marks frontend paths that match no known route.
type RouteNotFoundTag = transport.RouteNotFoundTag

// MethodNotAllowedTag marks requests whose path exists but method is not declared.
type MethodNotAllowedTag = transport.MethodNotAllowedTag

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
	frontend := &Frontend{
		schematicFactory:  schematicFactory,
		assetBuilder:      assetBuilder,
		artifactsManager:  artifactsManager,
		secureBootService: secureBootService,
		checksummer:       checksummer,
		signatureWriter:   signatureWriter,
		logger:            logger.With(zap.String("frontend", "http")),
		options:           opts,
	}
	schematicService := schematic.NewService(schematicFactory, opts.AuthProvider != nil, enterprise.Enabled())
	frontend.schematicAPI = applicationapi.NewSchematicHandler(schematicService)
	imageService := asset.NewImageService(
		schematicService,
		assetBuilder,
		asset.NewImageProfileEnhancer(artifactsManager, secureBootService),
		asset.ImageServiceOptions{
			ChecksumGenerator:  checksummer,
			SignatureGenerator: signatureWriter,
			Logger:             frontend.logger,
		},
	)
	frontend.imageAPI = applicationapi.NewImageHandler(
		imageService,
		applicationapi.ImageHandlerOptions{
			ExternalPXEURL: opts.ExternalPXEURL,
			Logger:         frontend.logger,
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

	frontend.contract, err = api.NewContract(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAPI contract: %w", err)
	}

	frontend.puller, err = remotewrap.NewPuller(opts.RegistryRefreshInterval, opts.InstallerInternalNameOptions, opts.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create puller: %w", err)
	}

	frontend.pusher, err = remotewrap.NewPusher(opts.RegistryRefreshInterval, opts.InstallerInternalNameOptions, opts.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create pusher: %w", err)
	}

	frontend.imageSigner = opts.CacheImageSigner
	frontend.metadata = metadata.New(artifactsManager, secureBootService, frontend.imageSigner, getLLMsTxt)

	frontend.evidencePublisher, err = enterprise.NewInstallerEvidencePublisher(
		frontend.logger,
		frontend.imageSigner,
		opts.InstallerSBOMSource,
		frontend.pusher,
		frontend.puller,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Installer evidence publisher: %w", err)
	}

	frontend.initializeRegistry()

	if err = frontend.registerRoutes(enterprisePlugins); err != nil {
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

func (f *Frontend) wrapper(h Handler) httprouter.Handle {
	return f.wrapHandlerProtocol(h, true, transport.ProtocolAPI)
}

func (f *Frontend) wrapHandler(h Handler, requireAuth bool) httprouter.Handle {
	return f.wrapHandlerProtocol(h, requireAuth, transport.ProtocolAPI)
}

func (f *Frontend) wrapHandlerProtocol(h Handler, requireAuth bool, protocol transport.Protocol) httprouter.Handle {
	access := transport.AccessPublic
	if requireAuth {
		access = transport.AccessAuthenticated
	}

	return f.wrapHandlerProtocolAccess(h, access, protocol)
}

func (f *Frontend) wrapHandlerProtocolAccess(h Handler, access transport.AccessPolicy, protocol transport.Protocol) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		requestID := requestIDFrom(r)
		ctx := ctxlog.WithRequestID(r.Context(), requestID)
		logger := ctxlog.Logger(ctx, f.logger)

		var state responseState

		sw := wrapResponseWriter(w, &state)

		sw.Header().Set("Server", version.ServerString())
		sw.Header().Set(RequestIDHeader, requestID)

		handler := f.withContractValidation(f.withAuth(h, access, &state))

		start := time.Now()
		err := handler(ctx, sw, r, p)
		duration := time.Since(start)

		classification := transport.ClassifyError(err)
		if state.Status() == 0 {
			transport.RenderError(sw, r, protocol, classification)
		}

		status := classification.Status
		if state.Status() != 0 {
			status = state.Status()
		} else {
			// Nothing was committed, so net/http sends an implicit 200 that reaches no hook.
			state.ApplyCacheControlPin(sw)
		}

		logger.Log(
			classification.Level, "request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.Error(err),
		)

		if access != transport.AccessPublic {
			username := ""
			if principal, ok := authn.PrincipalFromContext(r.Context()); ok { //nolint:contextcheck // Authentication middleware publishes its derived context to the request.
				username = principal.Username()
			}

			f.audit(ctx, logger, audit.Record{
				Time:      start,
				RequestID: requestID,
				Username:  username,
				ClientIP:  r.RemoteAddr,
				Method:    r.Method,
				Path:      r.URL.Path,
				Status:    status,
				Duration:  duration,
				Error:     errString(err),
			})
		}
	}
}

func (f *Frontend) withContractValidation(h Handler) Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		if f.contract != nil {
			_, _, err := f.contract.ValidateRequest(ctx, r)
			if errors.Is(err, routers.ErrPathNotFound) {
				return xerrors.NewTagged[RouteNotFoundTag](fmt.Errorf("route not registered in OpenAPI: %w", err))
			}

			if errors.Is(err, routers.ErrMethodNotAllowed) {
				return xerrors.NewTagged[MethodNotAllowedTag](fmt.Errorf("method not registered in OpenAPI: %w", err))
			}

			if err != nil {
				return xerrors.NewTagged[InvalidRequestTag](fmt.Errorf("invalid request: %w", err))
			}
		}

		return h(ctx, w, r, p)
	}
}

// requestIDFrom returns the incoming request ID, or a freshly generated one.
func requestIDFrom(r *http.Request) string {
	if id := r.Header.Get(RequestIDHeader); id != "" {
		return id
	}

	return uuid.NewString()
}

// withAuth selects authentication from the route's declarative access policy.
func (f *Frontend) withAuth(h Handler, access transport.AccessPolicy, state *responseState) Handler {
	if access == transport.AccessPublic || f.options.AuthProvider == nil {
		return h
	}

	selector := authentication.New(f.logger, f.options.AuthProvider, f.options.TokenVerifier)
	authenticated := selector.Middleware(access, h)

	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		w.Header().Set("Cache-Control", "no-store")
		state.PinCacheControl(w)

		return authenticated(ctx, w, r, p)
	}
}

// audit records one entry for an authenticated request; a sink failure is logged
// but never fails the request.
func (f *Frontend) audit(ctx context.Context, logger *zap.Logger, record audit.Record) {
	if f.options.AuditSink == nil {
		return
	}

	if err := f.options.AuditSink.Log(ctx, record); err != nil {
		logger.Error("failed to write audit record", zap.Error(err))
	}
}

// errString returns err's message, or "" when err is nil.
func errString(err error) string {
	if err != nil {
		return err.Error()
	}

	return ""
}

// MatchError is a compatibility adapter around transport.ClassifyError.
func MatchError(err error, callback func(message string, code int)) (zapcore.Level, int) {
	classification := transport.ClassifyError(err)
	if classification.Render {
		callback(classification.Message, classification.Status)
	}

	return classification.Level, classification.Status
}
