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
	"strings"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Handler is the type of HTTP handlers used by the enterprise frontend.
type Handler = func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

// Config is the full configuration for the Auth0 provider.
// It is constructed by pkg/enterprise/enterprise_on.go from the enterprise.Auth0Config
// passed in from service.go, keeping the auth0 package free of cmd/ imports.
type Config struct {
	Domain   string
	Audience string

	// Optional — required for browser login via authorization code + PKCE.
	ClientID     string
	ClientSecret string
	RedirectURL  string
	ExternalURL  string

	// IssuerURLOverride replaces the default issuer URL constructed from Domain.
	// It is used for both OIDC discovery and JWT issuer (iss) validation.
	// Intended for testing only; leave empty in production.
	IssuerURLOverride string

	// SessionKey is the 32-byte AES-256 key for session-cookie encryption.
	// nil disables browser login.
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
	jwksProvider  *jwks.CachingProvider
	jwtValidator  *validator.Validator
	authClient    *authentication.Authentication // nil in M2M-only mode
	sessionCipher cipher.AEAD                    // nil when browser login is disabled
	domain        string
	audience      string // retained for browser login authorize URL construction
	clientID      string // needed for browser login URL construction
	redirectURL   string
	externalURL   string
	logger        *zap.Logger
}

// NewProvider creates a new Auth0 authentication provider.
// Call Run to perform the initial OIDC/JWKS fetch and block until ctx is canceled.
func NewProvider(logger *zap.Logger, cfg Config) (*Provider, error) {
	if cfg.Domain == "" {
		return nil, errors.New("auth0: domain must not be empty")
	}

	if cfg.Audience == "" {
		return nil, errors.New("auth0: audience must not be empty")
	}

	if cfg.SessionKey != nil && len(cfg.SessionKey) != 32 {
		return nil, fmt.Errorf("auth0: session key must be exactly 32 bytes (got %d)", len(cfg.SessionKey))
	}

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
		jwksProvider: jwksProvider,
		jwtValidator: jwtValidator,
		domain:       cfg.Domain,
		audience:     cfg.Audience,
		clientID:     cfg.ClientID,
		redirectURL:  cfg.RedirectURL,
		externalURL:  cfg.ExternalURL,
		logger:       logger.With(zap.String("component", "auth0-provider")),
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

	// Create the authentication client for browser login when all four fields are present.
	if cfg.ClientID != "" && cfg.ClientSecret != "" && cfg.RedirectURL != "" && p.sessionCipher != nil {
		authClient, authErr := authentication.New(
			context.Background(),
			cfg.Domain,
			authentication.WithClientID(cfg.ClientID),
			authentication.WithClientSecret(cfg.ClientSecret),
		)
		if authErr != nil {
			return nil, fmt.Errorf("auth0: failed to create authentication client: %w", authErr)
		}

		p.authClient = authClient
	}

	return p, nil
}

// Run performs the initial OIDC discovery and JWKS fetch, logs readiness,
// then blocks until ctx is canceled.
func (p *Provider) Run(ctx context.Context) error {
	// Pre-warm: trigger OIDC discovery and the initial JWKS fetch.
	if _, err := p.jwksProvider.KeyFunc(ctx); err != nil {
		return fmt.Errorf("auth0: initial OIDC/JWKS fetch failed: %w", err)
	}

	p.logger.Info(
		"auth0: JWKS loaded, provider ready",
		zap.Bool("browser_login", p.BrowserLoginEnabled()),
	)

	<-ctx.Done()

	return nil
}

// Middleware implements enterprise.AuthProvider.
//
// Token resolution order:
//  1. Authorization: Bearer <token>
//  2. Basic Auth password field (OCI/curl clients that only speak Basic Auth)
//  3. Encrypted session cookie (browser users after completing the login flow)
//
// When no token is found:
//   - Browser requests (Accept: text/html) are redirected to /login.
//   - All other clients receive 401 with WWW-Authenticate: Bearer.
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
				returnTo := url.QueryEscape(r.URL.RequestURI())
				http.Redirect(w, r, "/login?return_to="+returnTo, http.StatusFound)

				return nil
			}

			logger.Debug("auth0: authentication required: no token provided")
			w.Header().Set("WWW-Authenticate", `Bearer realm="Image Factory Enterprise"`)

			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
		}

		username, err := p.validateToken(ctx, tokenStr)
		if err != nil {
			logger.Warn("auth0: authentication failed", zap.Error(err))

			// For browsers with an invalid/expired session, redirect to login rather
			// than returning a raw 401 the user cannot act on.
			if p.BrowserLoginEnabled() && isBrowserRequest(r) {
				clearSessionCookie(w)

				returnTo := url.QueryEscape(r.URL.RequestURI())
				http.Redirect(w, r, "/login?return_to="+returnTo, http.StatusFound)

				return nil
			}

			w.Header().Set("WWW-Authenticate", `Bearer realm="Image Factory Enterprise"`)

			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("invalid token"))
		}

		logger.Debug("auth0: authenticated", zap.String("username", username))

		ctx = auth.WithAuthUsername(ctx, username)

		return next(ctx, w, r, params)
	}
}

// VerifyCredentials implements enterprise.AuthProvider.
// The username argument is ignored; identity comes from the JWT claims.
func (p *Provider) VerifyCredentials(_, tokenStr string) bool {
	_, err := p.validateToken(context.Background(), tokenStr)

	return err == nil
}

// UsernameFromContext implements enterprise.AuthProvider.
func (p *Provider) UsernameFromContext(ctx context.Context) (string, bool) {
	return auth.GetAuthUsername(ctx)
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

// extractToken pulls the JWT from the request.
// Prefers Authorization: Bearer, falls back to Basic Auth password field
// (used by OCI/Talos registry auth).
func extractToken(r *http.Request) string {
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	_, password, ok := r.BasicAuth()
	if ok && password != "" {
		return password
	}

	return ""
}

// isBrowserRequest returns true when the client is a browser making a page navigation.
// XHR / fetch from the UI wizard typically sends Accept: application/json or */*,
// neither of which contains "text/html", so those correctly receive 401 not a redirect.
func isBrowserRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// validateToken validates the token and returns the org_id claim as the
// identity principal.  This aligns with the htpasswd provider where the
// username maps to an org, allowing the same ownership and audit machinery
// to work without additional context keys or interface changes.
//
// The type assertions are unconditional: ValidateToken always returns
// *validator.ValidatedClaims on success, CustomClaims is always *customClaims
// (set via WithCustomClaims), and OrgID is guaranteed non-empty by Validate().
func (p *Provider) validateToken(ctx context.Context, tokenStr string) (string, error) {
	vc, err := p.jwtValidator.ValidateToken(ctx, tokenStr)
	if err != nil {
		return "", fmt.Errorf("auth0: token validation failed: %w", err)
	}

	return vc.(*validator.ValidatedClaims).CustomClaims.(*customClaims).OrgID, nil
}
