// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package auth0 provides an Auth0-backed authentication provider.
package auth0

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Handler is the type of HTTP handlers used by the enterprise frontend.
type Handler = func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

const (
	// verifyCredentialsTimeout bounds VerifyCredentials, which has no context to inherit a deadline from.
	verifyCredentialsTimeout = 10 * time.Second

	// Bounds for the exponential backoff on the startup JWKS fetch.
	jwksRetryMinDelay = time.Second
	jwksRetryMaxDelay = time.Minute
)

// Config is the full configuration for the Auth0 provider.
// It is constructed by pkg/enterprise/enterprise_on.go from the enterprise.Auth0Config
// passed in from service.go, keeping the auth0 package free of cmd/ imports.
type Config struct {
	Domain   string
	Audience string

	// Browser login via authorization code + PKCE, added on top of the bearer-token
	// validation that Domain and Audience always enable.
	// ClientID, ClientSecret, RedirectURL and SessionKey must be set together or left
	// entirely empty; NewProvider rejects a partial set.
	// ExternalURL is optional and only used as the logout returnTo.
	ClientID     string
	ClientSecret string
	RedirectURL  string
	ExternalURL  string

	// IssuerURLOverride replaces the default issuer URL constructed from Domain.
	// It is used for both OIDC discovery and JWT issuer (iss) validation.
	// Intended for testing only; leave empty in production.
	IssuerURLOverride string

	// HTTPClient overrides the client used to reach Auth0 when building the
	// browser-login authentication client, whose construction fetches the
	// tenant JWKS. Intended for testing only; leave nil in production.
	HTTPClient *http.Client

	// SessionKey is the 32-byte AES-256 key for session-cookie encryption.
	// nil disables browser login.
	// It encrypts both the session cookie and the short-lived PKCE state cookie, so
	// every replica must be given the same key or logins and sessions break whenever
	// a request lands on a different replica than the one that issued the cookie.
	SessionKey []byte
}

// customClaims holds Auth0-specific JWT claims beyond the standard registered set.
type customClaims struct {
	OrgID string `json:"org_id"`
}

// Validate implements validator.CustomClaims.
func (c *customClaims) Validate(_ context.Context) error {
	if c.OrgID == "" {
		return errors.New("org_id claim is required")
	}

	return nil
}

// Provider is an authentication provider backed by Auth0 JWTs.
type Provider struct {
	// refreshGroup collapses concurrent refreshes of the same refresh token into one
	// exchange; see doRefresh.
	refreshGroup singleflight.Group

	// authClient is populated by Run once Auth0 is reachable; nil until then, and
	// always nil when browser login is off. Read it through auth().
	authClient atomic.Pointer[authentication.Authentication]

	jwksProvider  *jwks.CachingProvider
	jwtValidator  *validator.Validator
	httpClient    *http.Client // test hook for reaching Auth0; nil in production
	logger        *zap.Logger
	sessionCipher cipher.AEAD // nil when browser login is disabled
	domain        string
	audience      string // retained for browser login authorize URL construction
	clientID      string // needed for browser login URL construction
	clientSecret  string
	redirectURL   string
	callbackPath  string // path component of redirectURL; the route and state cookie both use it
	externalURL   string

	browserLogin      bool // derived from config alone, so routes register before Auth0 is reached
	externalURLSecure bool // externalURL is https, so cookies are Secure whatever the inbound hop looks like
}

// NewProvider creates a new Auth0 authentication provider. It validates config only and
// never reaches the network, so it fails just on operator error; call Run to contact the
// tenant and block until ctx is canceled.
func NewProvider(ctx context.Context, logger *zap.Logger, cfg Config) (*Provider, error) {
	if cfg.Domain == "" {
		return nil, errors.New("auth0: domain must not be empty")
	}

	if cfg.Audience == "" {
		return nil, errors.New("auth0: audience must not be empty")
	}

	if cfg.SessionKey != nil && len(cfg.SessionKey) != 32 {
		return nil, fmt.Errorf("auth0: session key must be exactly 32 bytes (got %d)", len(cfg.SessionKey))
	}

	browserLogin, err := browserLoginConfigured(cfg)
	if err != nil {
		return nil, err
	}

	var callbackPath string

	if browserLogin {
		if callbackPath, err = validateBrowserLogin(cfg); err != nil {
			return nil, err
		}
	}

	externalURLSecure := strings.HasPrefix(cfg.ExternalURL, "https://")

	// Determine the issuer URL used for both OIDC discovery and JWT iss validation.
	issuerURL := "https://" + cfg.Domain + "/"
	if cfg.IssuerURLOverride != "" {
		issuerURL = cfg.IssuerURLOverride
	}

	parsedIssuerURL, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth0: invalid issuer URL %q: %w", issuerURL, err)
	}

	jwksProvider, err := jwks.NewCachingProvider(jwks.WithIssuerURL(parsedIssuerURL))
	if err != nil {
		return nil, fmt.Errorf("auth0: failed to create JWKS provider: %w", err)
	}

	jwtValidator, err := validator.New(
		validator.WithKeyFunc(jwksProvider.KeyFunc),
		validator.WithAlgorithm(validator.RS256),
		validator.WithIssuer(issuerURL),
		validator.WithAudience(cfg.Audience),
		validator.WithCustomClaims(func() *customClaims { return &customClaims{} }),
	)
	if err != nil {
		return nil, fmt.Errorf("auth0: failed to create JWT validator: %w", err)
	}

	p := &Provider{
		jwksProvider:      jwksProvider,
		jwtValidator:      jwtValidator,
		httpClient:        cfg.HTTPClient,
		domain:            cfg.Domain,
		audience:          cfg.Audience,
		clientID:          cfg.ClientID,
		clientSecret:      cfg.ClientSecret,
		redirectURL:       cfg.RedirectURL,
		callbackPath:      callbackPath,
		externalURL:       cfg.ExternalURL,
		browserLogin:      browserLogin,
		externalURLSecure: externalURLSecure,
		logger:            logger.With(zap.String("component", "auth0-provider")),
	}

	// Pre-compute the AES-256-GCM cipher for session cookie encryption.
	if len(cfg.SessionKey) == 32 {
		block, err := aes.NewCipher(cfg.SessionKey)
		if err != nil {
			return nil, fmt.Errorf("auth0: failed to create AES cipher: %w", err)
		}

		p.sessionCipher, err = cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("auth0: failed to create GCM cipher: %w", err)
		}
	}

	return p, nil
}

// browserLoginConfigured reports whether the browser login flow is switched on.
// The four fields it needs are all-or-nothing; a partial set is an error rather
// than a factory that silently never serves /login.
func browserLoginConfigured(cfg Config) (bool, error) {
	fields := []struct {
		name    string
		present bool
	}{
		{"clientID", cfg.ClientID != ""},
		{"clientSecret", cfg.ClientSecret != ""},
		{"redirectURL", cfg.RedirectURL != ""},
		{"sessionKey", len(cfg.SessionKey) > 0},
	}

	var missing []string

	for _, field := range fields {
		if !field.present {
			missing = append(missing, field.name)
		}
	}

	switch len(missing) {
	case 0:
		return true, nil
	case len(fields):
		return false, nil
	default:
		return false, fmt.Errorf(
			"auth0: browser login requires clientID, clientSecret, redirectURL and sessionKey together (missing: %s); "+
				"omit all four to serve machine-to-machine bearer tokens only",
			strings.Join(missing, ", "),
		)
	}
}

// validateBrowserLogin checks the URLs the browser flow is built on and returns
// the callback route derived from redirectURL.
func validateBrowserLogin(cfg Config) (string, error) {
	parsed, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return "", fmt.Errorf("auth0: invalid redirectURL %q: %w", cfg.RedirectURL, err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("auth0: redirectURL must be absolute, e.g. https://factory.example.com/callback (got %q)", cfg.RedirectURL)
	}

	if parsed.Path == "" || parsed.Path == "/" {
		return "", fmt.Errorf("auth0: redirectURL must include a path, e.g. https://factory.example.com/callback (got %q)", cfg.RedirectURL)
	}

	// ":" and "*" mark wildcards to the router, so a path containing either would
	// register a pattern instead of a literal route.
	if strings.ContainsAny(parsed.Path, ":*") {
		return "", fmt.Errorf("auth0: redirectURL path must not contain ':' or '*' (got %q)", parsed.Path)
	}

	// Auth0 only accepts an allow-listed absolute returnTo on logout.
	if cfg.ExternalURL == "" {
		return "", errors.New("auth0: externalURL is required for browser login (used as the Auth0 logout returnTo)")
	}

	return parsed.Path, nil
}

// Run reaches out to the Auth0 tenant, logs readiness, then blocks until ctx is canceled.
//
// Every step is retried rather than fatal, so an outage at startup does not take down the
// routes that need no authentication at all. Until it succeeds tokens fail validation and
// /login returns 503, which is degraded service rather than an open door.
func (p *Provider) Run(ctx context.Context) error {
	delay := jwksRetryMinDelay

	for {
		err := p.warmup(ctx)
		if err == nil {
			break
		}

		if ctx.Err() != nil {
			return nil //nolint:nilerr // shutdown, not a failure
		}

		p.logger.Warn(
			"auth0: tenant unreachable, retrying; authentication will fail until it succeeds",
			zap.Duration("retry_in", delay),
			zap.Error(err),
		)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}

		if delay *= 2; delay > jwksRetryMaxDelay {
			delay = jwksRetryMaxDelay
		}
	}

	p.logger.Info(
		"auth0: provider ready",
		zap.Bool("browser_login", p.BrowserLoginEnabled()),
	)

	<-ctx.Done()

	return nil
}

// warmup runs the startup steps that need to reach Auth0. Both are idempotent, so Run
// can simply call it again after a failure.
func (p *Provider) warmup(ctx context.Context) error {
	if _, err := p.jwksProvider.KeyFunc(ctx); err != nil {
		return fmt.Errorf("JWKS fetch: %w", err)
	}

	if !p.browserLogin || p.authClient.Load() != nil {
		return nil
	}

	// Constructing this fetches the tenant JWKS used to validate ID tokens.
	authOpts := []authentication.Option{
		authentication.WithClientID(p.clientID),
		authentication.WithClientSecret(p.clientSecret),
	}

	if p.httpClient != nil {
		authOpts = append(authOpts, authentication.WithClient(p.httpClient))
	}

	authClient, err := authentication.New(ctx, p.domain, authOpts...)
	if err != nil {
		return fmt.Errorf("authentication client: %w", err)
	}

	p.authClient.Store(authClient)

	return nil
}

// auth returns the Auth0 authentication client, or a NotReadyTag error (503) while the
// startup fetch in Run is still failing.
func (p *Provider) auth() (*authentication.Authentication, error) {
	if authClient := p.authClient.Load(); authClient != nil {
		return authClient, nil
	}

	return nil, xerrors.NewTagged[enterrors.NotReadyTag](errors.New("auth0: browser login not ready"))
}

// Middleware implements enterprise.AuthProvider.
//
// Token resolution order:
//  1. Authorization header, either Bearer or the Basic password field
//  2. Encrypted session cookie (browser users after completing the login flow)
//
// When no token is found:
//   - Browser requests (Accept: text/html) are redirected to /login.
//   - All other clients receive 401 with a WWW-Authenticate challenge.
func (p *Provider) Middleware(next Handler) Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)

		tokenStr := extractToken(r)

		// Session cookie — only checked when browser login is configured.
		if tokenStr == "" && p.BrowserLoginEnabled() {
			t, err := p.sessionToken(ctx, w, r)
			if err != nil {
				logger.Debug("auth0: invalid or expired session cookie", zap.Error(err))
				clearSessionCookie(w)
			} else {
				tokenStr = t
			}
		}

		if tokenStr == "" {
			if p.BrowserLoginEnabled() && isBrowserRequest(r) {
				return p.redirectToLogin(w, r, "no token provided")
			}

			logger.Debug("auth0: authentication required: no token provided")
			setChallenge(w)

			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
		}

		username, err := p.validateToken(ctx, tokenStr)
		if err != nil {
			logger.Warn("auth0: authentication failed", zap.Error(err))

			// For browsers with an invalid/expired session, redirect to login rather
			// than returning a raw 401 the user cannot act on.
			if p.BrowserLoginEnabled() && isBrowserRequest(r) {
				clearSessionCookie(w)

				return p.redirectToLogin(w, r, "invalid token")
			}

			setChallenge(w)

			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("invalid token"))
		}

		logger.Debug("auth0: authenticated", zap.String("username", username))

		ctx = auth.WithAuthUsername(ctx, username)

		return next(ctx, w, r, params)
	}
}

// setChallenge advertises Basic ahead of Bearer, as two header values rather than one
// comma-joined line so a parser cannot read "Bearer" as a parameter of the Basic challenge.
//
// Order matters: containerd and go-containerregistry both take the first scheme they
// recognize. Our Bearer realm is a description, not a token endpoint, so a client that
// picks it has nowhere to go — whereas Basic is the machine path, carrying the Auth0 JWT
// in the password field.
func setChallenge(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="Image Factory Enterprise", charset="UTF-8"`)
	w.Header().Add("WWW-Authenticate", `Bearer realm="Image Factory Enterprise"`)
}

// redirectToLogin sends a browser to /login, preserving the URL it was trying to reach.
// It returns a RedirectedTag error rather than nil so the audit log records a 302 for an
// unauthenticated caller instead of a successful anonymous request.
func (p *Provider) redirectToLogin(w http.ResponseWriter, r *http.Request, reason string) error {
	http.Redirect(w, r, loginURL(r.URL.RequestURI()), http.StatusFound)

	return xerrors.NewTagged[enterrors.RedirectedTag](fmt.Errorf("redirected to login: %s", reason))
}

// VerifyCredentials implements enterprise.AuthProvider.
// The username argument is ignored; identity comes from the JWT claims.
func (p *Provider) VerifyCredentials(_, tokenStr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), verifyCredentialsTimeout)
	defer cancel()

	_, err := p.validateToken(ctx, tokenStr)

	return err == nil
}

// UsernameFromContext implements enterprise.AuthProvider.
func (p *Provider) UsernameFromContext(ctx context.Context) (string, bool) {
	return auth.GetAuthUsername(ctx)
}

// ContextWithUsername implements enterprise.AuthProvider.
func (p *Provider) ContextWithUsername(ctx context.Context, username string) context.Context {
	return auth.WithAuthUsername(ctx, username)
}

// sessionToken extracts the access token from the session cookie, refreshing it
// transparently when the token is expired or within 5 minutes of expiry.
// Returns ("", nil) when no session cookie is present.
func (p *Provider) sessionToken(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, error) {
	payload, ok, err := readSessionPayload(r, p.sessionCipher)
	if err != nil {
		return "", err
	}

	if !ok {
		return "", nil // no cookie present
	}

	now := time.Now()

	// Expired: attempt refresh; fail hard if no refresh token.
	if now.After(payload.Expiry) {
		if payload.RefreshToken == "" {
			return "", errors.New("session expired and no refresh token available")
		}

		return p.doRefresh(ctx, w, r, payload)
	}

	// Proactively refresh within 5 minutes of expiry (best-effort).
	if time.Until(payload.Expiry) < 5*time.Minute && payload.RefreshToken != "" {
		if refreshed, err := p.doRefresh(ctx, w, r, payload); err == nil {
			return refreshed, nil
		}

		// Refresh failed but token is still valid — continue with existing token.
	}

	return payload.AccessToken, nil
}

// extractToken pulls the JWT from the Authorization header, accepting it either
// as a Bearer value or in the Basic password field (used by OCI/Talos registry
// clients, which only speak Basic Auth).
func extractToken(r *http.Request) string {
	scheme, value, _ := strings.Cut(r.Header.Get("Authorization"), " ")

	// RFC 9110 makes the scheme case-insensitive; some clients send "bearer".
	if strings.EqualFold(scheme, "Bearer") {
		return value
	}

	// Only a JWT-shaped password is taken as a token. Anything else — a cached browser
	// credential, an htpasswd password — would fail validation and bounce the browser to
	// /login, which resends the same header ahead of the session cookie: a redirect loop.
	if _, password, ok := r.BasicAuth(); ok && looksLikeJWT(password) {
		return password
	}

	return ""
}

// looksLikeJWT reports whether s has the three non-empty dot-separated segments of
// a JWS compact serialization. It is a shape check, not validation.
func looksLikeJWT(s string) bool {
	parts := strings.Split(s, ".")

	return len(parts) == 3 && !slices.Contains(parts, "")
}

// isBrowserRequest returns true when the client is a browser making a page navigation.
// XHR / fetch from the UI wizard typically sends Accept: application/json or */*,
// neither of which contains "text/html", so those correctly receive 401 not a redirect.
func isBrowserRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// validateToken validates the token and returns the org_id claim as the
// identity principal. This aligns with the htpasswd provider where the
// username maps to an org, allowing the same ownership and audit machinery
// to work without additional context keys or interface changes.
//
// OrgID is guaranteed non-empty by customClaims.Validate.
func (p *Provider) validateToken(ctx context.Context, tokenStr string) (string, error) {
	vc, err := p.jwtValidator.ValidateToken(ctx, tokenStr)
	if err != nil {
		return "", fmt.Errorf("auth0: token validation failed: %w", err)
	}

	validated, ok := vc.(*validator.ValidatedClaims)
	if !ok {
		return "", fmt.Errorf("auth0: unexpected claims type %T", vc)
	}

	claims, ok := validated.CustomClaims.(*customClaims)
	if !ok {
		return "", fmt.Errorf("auth0: unexpected custom claims type %T", validated.CustomClaims)
	}

	return claims.OrgID, nil
}
