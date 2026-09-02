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
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/cors"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xerrors"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	httproutermiddleware "github.com/slok/go-http-metrics/middleware/httprouter"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
	"github.com/siderolabs/image-factory/internal/secureboot"
	"github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	router            *httprouter.Router
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
	readinessCheckers []enterprise.ReadinessChecker
	sf                singleflight.Group
	options           Options
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
type Handler = func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

// NewFrontend creates a new HTTP frontend.
func NewFrontend(
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
		router:            httprouter.New(),
		schematicFactory:  schematicFactory,
		assetBuilder:      assetBuilder,
		artifactsManager:  artifactsManager,
		secureBootService: secureBootService,
		checksummer:       checksummer,
		signatureWriter:   signatureWriter,
		logger:            logger.With(zap.String("frontend", "http")),
		options:           opts,
	}

	for _, p := range enterprisePlugins {
		if rc, ok := p.(enterprise.ReadinessChecker); ok {
			frontend.readinessCheckers = append(frontend.readinessCheckers, rc)
		}
	}

	var err error

	frontend.puller, err = remotewrap.NewPuller(opts.RegistryRefreshInterval, opts.InstallerInternalNameOptions, opts.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create puller: %w", err)
	}

	frontend.pusher, err = remotewrap.NewPusher(opts.RegistryRefreshInterval, opts.InstallerInternalNameOptions, opts.RemoteOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create pusher: %w", err)
	}

	frontend.imageSigner = opts.CacheImageSigner

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

	// monitoring middleware
	mdlw := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{
			Prefix: opts.MetricsNamespace,
		}),
	})

	registerRoute := func(registrator func(string, httprouter.Handle), path string, handler Handler) {
		registrator(path, httproutermiddleware.Handler(path, frontend.wrapper(handler), mdlw))
	}

	registerPublicRoute := func(registrator func(string, httprouter.Handle), path string, handler Handler) {
		registrator(path, httproutermiddleware.Handler(path, frontend.wrapperPublic(handler), mdlw))
	}

	// enterprise
	for _, enterpriseRoute := range enterprisePlugins {
		_, isPublic := enterpriseRoute.(enterprise.PublicRoute)

		for _, method := range enterpriseRoute.Methods() {
			var registrator func(string, httprouter.Handle)

			switch method {
			case http.MethodGet:
				registrator = frontend.router.GET
			case http.MethodHead:
				registrator = frontend.router.HEAD
			case http.MethodPost:
				registrator = frontend.router.POST
			default:
				panic(fmt.Sprintf("unsupported method %s for enterprise route %s", method, enterpriseRoute.Path()))
			}

			if isPublic {
				registerPublicRoute(registrator, enterpriseRoute.Path(), enterpriseRoute.Handle)
			} else {
				registerRoute(registrator, enterpriseRoute.Path(), enterpriseRoute.Handle)
			}
		}
	}

	// /healthz and /readyz are always public (Kubernetes probes, monitoring)
	registerPublicRoute(frontend.router.GET, "/healthz", frontend.handleHealth)
	registerPublicRoute(frontend.router.HEAD, "/healthz", frontend.handleHealth)
	registerPublicRoute(frontend.router.GET, "/readyz", frontend.handleReady)
	registerPublicRoute(frontend.router.HEAD, "/readyz", frontend.handleReady)

	// images - require auth (API tokens bypass auth via JWT verification)
	registerRoute(frontend.router.GET, "/image/:schematic/:version/:path", frontend.handleImage)
	registerRoute(frontend.router.HEAD, "/image/:schematic/:version/:path", frontend.handleImage)

	// PXE - require auth
	registerRoute(frontend.router.GET, "/pxe/:schematic/:version/:path", frontend.handlePXE)

	// registry - /v2 requires auth (OCI spec: 401 challenge when auth enabled)
	registerRoute(frontend.router.GET, "/v2", frontend.handleHealth)
	registerRoute(frontend.router.HEAD, "/v2", frontend.handleHealth)
	registerRoute(frontend.router.GET, "/v2/*path", frontend.handleV2)
	registerRoute(frontend.router.HEAD, "/v2/*path", frontend.handleV2)
	registerPublicRoute(frontend.router.GET, "/oci/cosign/signing-key.pub", frontend.handleCosignSigningKeyPub)

	// schematic - both POST and GET require auth
	registerRoute(frontend.router.POST, "/schematics", frontend.handleSchematicCreate)
	registerRoute(frontend.router.GET, "/schematics/:schematic", frontend.handleSchematicGet)

	// meta - public
	registerPublicRoute(frontend.router.GET, "/versions", frontend.handleVersions)
	registerPublicRoute(frontend.router.GET, "/version/:version/extensions/official", frontend.handleOfficialExtensions)
	registerPublicRoute(frontend.router.GET, "/version/:version/overlays/official", frontend.handleOfficialOverlays)

	// secureboot - public
	registerPublicRoute(frontend.router.GET, "/secureboot/signing-cert.pem", frontend.handleSecureBootSigningCert)

	// talosctl - public
	registerPublicRoute(frontend.router.GET, "/talosctl/:version", frontend.handleTalosctlList)
	registerPublicRoute(frontend.router.HEAD, "/talosctl/:version/:path", frontend.handleTalosctl)
	registerPublicRoute(frontend.router.GET, "/talosctl/:version/:path", frontend.handleTalosctl)

	// llms.txt - public
	registerPublicRoute(frontend.router.GET, "/llms.txt", frontend.handleLLMsTxt)

	// UI - require auth (consistent with all other schematic-creating endpoints)
	registerRoute(frontend.router.GET, "/", frontend.handleUI)
	registerRoute(frontend.router.HEAD, "/", frontend.handleUI)
	registerRoute(frontend.router.POST, "/ui/wizard", frontend.handleUIWizard)
	registerRoute(frontend.router.GET, "/ui/version-doc", frontend.handleUIVersionDoc)
	registerRoute(frontend.router.POST, "/ui/extensions-list", frontend.handleUIExtensionsList)
	registerRoute(frontend.router.GET, "/ui/tokens", frontend.handleTokensUI)

	frontend.router.ServeFiles("/css/*filepath", http.FS(ensure.Value(fs.Sub(cssFS, "css"))))
	frontend.router.ServeFiles("/favicons/*filepath", http.FS(ensure.Value(fs.Sub(faviconsFS, "favicons"))))
	frontend.router.ServeFiles("/js/*filepath", http.FS(ensure.Value(fs.Sub(jsFS, "js"))))

	frontend.registerBrowserLogin(registerPublicRoute)

	return frontend, nil
}

// Handler returns the HTTP handler.
func (f *Frontend) Handler() http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins: f.options.AllowedOrigins,
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodOptions,
		},
		AllowedHeaders: []string{"Cache-Control"},
		ExposedHeaders: []string{"Content-Disposition", "Content-Length", "Content-Type"},
	}).Handler(f.router)
}

func (f *Frontend) wrapper(h Handler) httprouter.Handle {
	return f.wrapHandler(h, true)
}

func (f *Frontend) wrapperPublic(h Handler) httprouter.Handle {
	return f.wrapHandler(h, false)
}

func (f *Frontend) wrapHandler(h Handler, requireAuth bool) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		requestID := requestIDFrom(r)
		ctx := ctxlog.WithRequestID(r.Context(), requestID)
		logger := ctxlog.Logger(ctx, f.logger)

		var state responseState

		sw := wrapResponseWriter(w, &state)

		sw.Header().Set("Server", version.ServerString())
		sw.Header().Set(RequestIDHeader, requestID)

		var username string

		handler := f.withAuth(h, requireAuth, &username, &state)

		start := time.Now()
		err := handler(ctx, sw, r, p)
		duration := time.Since(start)

		level, status := MatchError(err, func(message string, code int) {
			if state.status != 0 {
				// The handler already responded; this error is only here to be logged.
				return
			}

			if code == http.StatusUnauthorized {
				// Fallback only: a provider that picked its own challenges keeps them, and one
				// redirecting htmx wants none — a challenge pops the browser's Basic dialog.
				if sw.Header().Get("WWW-Authenticate") == "" && sw.Header().Get("Hx-Redirect") == "" {
					sw.Header().Set("WWW-Authenticate", `Basic realm="Image Factory Enterprise", charset="UTF-8"`)
				}
			}

			http.Error(sw, message, code)
		})

		// A handler that answered for itself returns nil, which would log as a 200.
		if state.status != 0 {
			status = state.status
		} else {
			// Nothing was committed, so net/http sends an implicit 200 that reaches no hook.
			state.applyCacheControlPin(sw)
		}

		logger.Log(
			level, "request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.Error(err),
		)

		if requireAuth {
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

// requestIDFrom returns the incoming request ID, or a freshly generated one.
func requestIDFrom(r *http.Request) string {
	if id := r.Header.Get(RequestIDHeader); id != "" {
		return id
	}

	return uuid.NewString()
}

// downloadTokenKey keys the verified ephemeral API token on the request context.
type downloadTokenKey struct{}

// downloadTokenFromContext returns the ephemeral API token that authenticated the request.
// It is only ever set after Verify succeeded, so a caller may forward it as-is.
func downloadTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(downloadTokenKey{}).(string)

	return token, ok
}

// withAuth wraps h with the auth middleware when authentication is required.
//
// The middleware stores the username on a context it derives internally, which
// never reaches wrapHandler's context; a thin capture layer reads it from inside
// the middleware call stack and writes it to username.
func (f *Frontend) withAuth(h Handler, requireAuth bool, username *string, state *responseState) Handler {
	if !requireAuth || f.options.AuthProvider == nil {
		return h
	}

	authProvider := f.options.AuthProvider

	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		// API token: the JWT subject becomes the authenticated identity, so ownership is then
		// enforced normally by schematicFactory.Get(). An ephemeral token is also put on the
		// context, for handlePXE to forward into the asset URLs it emits.
		if f.options.TokenVerifier != nil {
			if tokenStr, fromQuery := extractAPIToken(r); tokenStr != "" {
				// Verify logs its own rejection reasons; the two checks below are this layer's,
				// and are logged here so an authorization failure is never mistaken for the
				// credential failure the fallback provider goes on to report.
				if claims, ok := f.options.TokenVerifier.Verify(ctx, tokenStr); ok {
					switch {
					case !apitoken.Allows(claims.Scopes, r.Method, r.URL.Path):
						ctxlog.Logger(ctx, f.logger).Warn(
							"API token scopes do not cover this request",
							zap.String("sub", claims.Subject),
							zap.String("jti", claims.ID),
							zap.Strings("scopes", claims.Scopes),
						)
					case fromQuery && (claims.Stored || !apitoken.URLSafe(claims.Scopes)):
						ctxlog.Logger(ctx, f.logger).Warn(
							"API token may not travel in a query string",
							zap.String("sub", claims.Subject),
							zap.String("jti", claims.ID),
							zap.Strings("scopes", claims.Scopes),
							zap.Bool("stored", claims.Stored),
						)
					default:
						*username = claims.Subject
						ctx = authProvider.ContextWithUsername(ctx, claims.Subject)

						ctx = apitoken.ContextWithClaims(ctx, claims)

						if !claims.Stored && apitoken.URLSafe(claims.Scopes) {
							ctx = context.WithValue(ctx, downloadTokenKey{}, tokenStr)
						}

						return h(ctx, w, r, p)
					}
				}
			}
		}

		err := authProvider.Middleware(func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
			*username, _ = authProvider.UsernameFromContext(ctx)

			// The provider has decided by now, so pin the Cache-Control it chose.
			state.pinCacheControl(w)

			return h(ctx, w, r, p)
		})(ctx, w, r, p)

		// A provider can authenticate the caller and then refuse the request, in which case
		// the layer above never ran. The provider leaves the principal on the request so the
		// denial is still attributable.
		if *username == "" {
			*username, _ = authProvider.UsernameFromContext(r.Context()) //nolint:contextcheck // the provider derived this context from ctx
		}

		return err
	}
}

// extractAPIToken pulls an API token off the request, reporting whether it came from the
// query string, which the caller pairs with the token's stored claim and apitoken.URLSafe. An
// ephemeral token is short-lived enough to survive an access log; a stored one is not, and neither
// is a minting credential of any lifetime.
func extractAPIToken(r *http.Request) (token string, fromQuery bool) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if token = r.URL.Query().Get("token"); token != "" {
			return token, true
		}
	}

	return extractBearerOrBasicToken(r), false
}

// extractBearerOrBasicToken pulls a bearer credential from the Authorization header, checking
// both the Bearer scheme and HTTP Basic auth, since OCI/registry clients commonly send a token
// as the Basic password rather than a Bearer header.
func extractBearerOrBasicToken(r *http.Request) string {
	scheme, value, _ := strings.Cut(r.Header.Get("Authorization"), " ")

	// RFC 9110 makes the scheme case-insensitive; some clients send "bearer".
	if strings.EqualFold(scheme, "Bearer") {
		return value
	}

	if _, password, ok := r.BasicAuth(); ok {
		return password
	}

	return ""
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

// MatchError matches the error and returns the appropriate HTTP status code and log level.
// It also calls the callback with the message and code to write the response.
//
// enterrors.RespondedTag is the exception: no callback, and the status is a placeholder that
// callers replace with the one on the response writer.
func MatchError(err error, callback func(message string, code int)) (zapcore.Level, int) {
	status := http.StatusOK
	level := zap.InfoLevel

	switch {
	case err == nil:
		// happy case
	case xerrors.TagIs[enterrors.RespondedTag](err):
		level = zap.WarnLevel
	case xerrors.TagIs[enterrors.NotEnabledTag](err):
		level = zap.WarnLevel
		status = http.StatusPaymentRequired

		callback(err.Error(), http.StatusPaymentRequired)
	case xerrors.TagIs[enterrors.NotReadyTag](err):
		level = zap.WarnLevel
		status = http.StatusServiceUnavailable

		callback("service temporarily unavailable", http.StatusServiceUnavailable)
	case xerrors.TagIs[ProxyUnavailableTag](err):
		level = zap.WarnLevel
		status = http.StatusServiceUnavailable

		callback(err.Error(), http.StatusServiceUnavailable)
	case xerrors.TagIs[storage.ErrNotFoundTag](err),
		xerrors.TagIs[artifacts.ErrNotFoundTag](err),
		xerrors.TagIs[RouteNotFoundTag](err):
		level = zap.WarnLevel
		status = http.StatusNotFound

		callback(err.Error(), http.StatusNotFound)
	case xerrors.TagIs[profile.InvalidErrorTag](err),
		xerrors.TagIs[schematicpkg.InvalidErrorTag](err),
		xerrors.TagIs[enterrors.InvalidErrorTag](err),
		xerrors.TagIs[InvalidImageTag](err):
		level = zap.WarnLevel
		status = http.StatusBadRequest

		callback(err.Error(), http.StatusBadRequest)
	case xerrors.TagIs[schematicpkg.RequiresAuthenticationTag](err):
		level = zap.WarnLevel
		status = http.StatusUnauthorized

		callback("authentication required to access this schematic", http.StatusUnauthorized)
	case xerrors.TagIs[schematicpkg.ForbiddenTag](err):
		level = zap.WarnLevel
		status = http.StatusForbidden

		callback("access denied", http.StatusForbidden)
	case errors.Is(err, context.Canceled):
		status = 499
		// client closed connection
	default:
		status = http.StatusInternalServerError
		level = zap.ErrorLevel

		callback("internal server error", http.StatusInternalServerError)
	}

	return level, status
}

// Use several ways to detect language.
func (f *Frontend) getLocalizer(r *http.Request) *i18n.Localizer {
	lang := r.URL.Query().Get("lang")

	if lang == "" {
		if cookie, err := r.Cookie("lang"); err == nil {
			lang = cookie.Value
		}
	}

	if lang == "" {
		lang = r.Header.Get("Accept-Language")
	}

	return i18n.NewLocalizer(getLocalizerBundle(), lang, "en")
}

// handleReady reports readiness once all enterprise plugins implementing
// ReadinessChecker report ready. Used by orchestration probes to gate traffic
// (e.g. async Grype DB warm-up).
func (f *Frontend) handleReady(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	for _, rc := range f.readinessCheckers {
		if err := rc.Ready(); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)

			return nil //nolint:nilerr
		}
	}

	w.WriteHeader(http.StatusOK)

	return nil
}
