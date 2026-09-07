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
	"strings"
	"time"

	"github.com/getkin/kin-openapi/routers"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/cors"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/api"
	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/secureboot"
	"github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// Frontend is the HTTP frontend.
type Frontend struct {
	handler           http.Handler
	contract          *api.Contract
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
type Handler = transport.Handler

// InvalidRequestTag marks requests rejected by the OpenAPI contract.
type InvalidRequestTag = transport.InvalidRequestTag

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

	for _, p := range enterprisePlugins {
		if rc, ok := p.(enterprise.ReadinessChecker); ok {
			frontend.readinessCheckers = append(frontend.readinessCheckers, rc)
		}
	}

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

	router := httprouter.New()
	frontend.handler = router

	if err = frontend.registerRoutes(router, enterprisePlugins); err != nil {
		return nil, fmt.Errorf("register HTTP routes: %w", err)
	}

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
	}).Handler(f.handler)
}

func (f *Frontend) wrapper(h Handler) httprouter.Handle {
	return f.wrapHandlerProtocol(h, true, transport.ProtocolAPI)
}

func (f *Frontend) wrapHandler(h Handler, requireAuth bool) httprouter.Handle {
	return f.wrapHandlerProtocol(h, requireAuth, transport.ProtocolAPI)
}

func (f *Frontend) wrapHandlerProtocol(h Handler, requireAuth bool, protocol transport.Protocol) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		requestID := requestIDFrom(r)
		ctx := ctxlog.WithRequestID(r.Context(), requestID)
		logger := ctxlog.Logger(ctx, f.logger)

		var state responseState

		sw := wrapResponseWriter(w, &state)

		sw.Header().Set("Server", version.ServerString())
		sw.Header().Set(RequestIDHeader, requestID)

		var username string

		handler := f.withContractValidation(f.withAuth(h, requireAuth, &username, &state))

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

// downloadTokenKey keys the verified URL-safe API token on the request context.
type downloadTokenKey struct{}

// downloadTokenFromContext returns the URL-safe API token that authenticated the request.
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
		// enforced normally by schematicFactory.Get(). A URL-safe token is also put on the
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
					case fromQuery && !apitoken.URLSafe(claims.Scopes):
						ctxlog.Logger(ctx, f.logger).Warn(
							"API token may not travel in a query string",
							zap.String("sub", claims.Subject),
							zap.String("jti", claims.ID),
							zap.Strings("scopes", claims.Scopes),
						)
					default:
						*username = claims.Subject
						ctx = authProvider.ContextWithUsername(ctx, claims.Subject)

						ctx = apitoken.ContextWithClaims(ctx, claims)

						if apitoken.URLSafe(claims.Scopes) {
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
			state.PinCacheControl(w)

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
// query string, which the caller pairs with apitoken.URLSafe.
//
// A token that arrives in a query string has already been written to whatever access logs sit in
// front of the factory, and refusing it un-leaks nothing; the operator took that risk knowingly,
// and a stored token is the better credential to have taken it with, being both revocable and
// expiring. A minting credential is the exception URLSafe encodes: leaking one yields more
// credentials rather than just itself, and no PXE flow needs one.
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

// MatchError is a compatibility adapter around transport.ClassifyError.
func MatchError(err error, callback func(message string, code int)) (zapcore.Level, int) {
	classification := transport.ClassifyError(err)
	if classification.Render {
		callback(classification.Message, classification.Status)
	}

	return classification.Level, classification.Status
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
