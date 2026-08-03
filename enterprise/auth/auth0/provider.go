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
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/internal/cache"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Handler is the type of HTTP handlers used by the enterprise frontend.
type Handler = auth.Handler

const (
	// jwksFetchTimeout bounds the JWKS fetch, which happens inline on the request path.
	jwksFetchTimeout = 5 * time.Second

	// tokenExchangeTimeout bounds the Auth0 token endpoint, which the middleware
	// reaches on the request path when refreshing a session.
	tokenExchangeTimeout = 10 * time.Second

	// clockSkew tolerates the factory's clock running ahead of the Auth0 tenant's.
	// go-oidc runs both checks off one Now hook, so moving the clock back widens the exp
	// window and narrows go-oidc's own 5m nbf tolerance by the same amount.
	// Keep this well under 5m or freshly issued tokens carrying nbf start failing.
	clockSkew = 30 * time.Second

	// jwksRefetchInterval and jwksRefetchBurst bound how often the JWKS endpoint is refetched.
	// Auth0 rotates keys rarely, so this only has to keep up with real rotations.
	jwksRefetchInterval = 5 * time.Second
	jwksRefetchBurst    = 3

	// refreshResultTTL is how long a spent refresh token keeps answering with the payload
	// it produced. It only has to outlast the spread of a page load's parallel requests.
	//
	// For this long a replayed refresh token is served rather than passed to Auth0, so it
	// does not trip rotation-reuse detection: a stolen session cookie buys a working
	// session instead of revoking the grant family. That is a fair trade at a minute,
	// since the same cookie already carries a live access token, but widening this
	// widens that window too.
	refreshResultTTL = time.Minute

	// refreshResultCapacity bounds the entries above; each holds one session's tokens.
	refreshResultCapacity = 4096
)

// jwksTransport bounds JWKS refetches. go-oidc refetches whenever no cached key verifies
// the signature, so without this any well-formed JWT costs one request to the Auth0 tenant,
// which rate-limits that endpoint. Concurrent verifications coalesce into a single fetch
// inside go-oidc, so this only has to bound the sequential case.
type jwksTransport struct {
	next    http.RoundTripper
	limiter *rate.Limiter
	logger  *zap.Logger
}

func (t *jwksTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Queue rather than fail: during a key rotation a valid token needs this fetch, and
	// dropping it would 401 a legitimate caller. Wait returns immediately when the delay
	// would outlive the request, so the client timeout bounds how many callers pile up.
	if err := t.limiter.Wait(r.Context()); err != nil {
		// Warn because the caller only sees a 401, indistinguishable from bad credentials:
		// during a rotation under load this log is the only signal that it was throttling.
		// Safe to log loudly: the limiter caps this well below the request rate.
		t.logger.Warn("auth0: jwks refetch throttled, key rotation may be delayed", zap.Error(err))

		return nil, fmt.Errorf("jwks refetch rate exceeded: %w", err)
	}

	return t.next.RoundTrip(r)
}

// Config is the full configuration for the Auth0 provider.
type Config struct {
	Domain   string
	Audience string

	// MachineScope, when set, marks tokens carrying it as machine credentials, limited to
	// artifact fetches. Leave empty to give every token full access.
	MachineScope string

	// Browser login via authorization code + PKCE, on top of the bearer-token validation
	// that Domain and Audience always enable.
	// These three and SessionKey must be set together or left entirely empty;
	// NewProvider rejects a partial set.
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// ExternalURL is the factory's own base URL, used as the Auth0 logout returnTo
	// and as the floor for the cookie Secure attribute.
	ExternalURL string

	// IssuerURLOverride replaces the default issuer URL constructed from Domain.
	// It sets the expected iss claim and the JWKS, authorize and token endpoints.
	// Intended for testing only; leave empty in production.
	IssuerURLOverride string

	// SessionKey is the 32-byte AES-256 key for the session and PKCE state cookies.
	SessionKey []byte
}

// normalizeDomain accepts the tenant domain with or without a scheme or trailing slash,
// so that a copy-paste from the Auth0 console fails at startup rather than on every token.
// The result is the trust anchor for every token, so anything but a bare host is rejected
// rather than trimmed into something that resolves elsewhere.
func normalizeDomain(domain string) (string, error) {
	// Lowercased first because the result is compared byte-for-byte against the iss claim;
	// a console copy-paste with any capitals would otherwise fail on every token instead.
	lowered := strings.ToLower(domain)
	trimmed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(lowered, "https://"), "http://"), "/")

	// Empty is checked explicitly: trimming reduces "/" and "https://" to "", which
	// url.Parse then accepts as a bare (empty) host.
	u, err := url.Parse("https://" + trimmed)
	if trimmed == "" || err != nil || u.Host != trimmed || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("auth0: domain must be a bare tenant host, got %q", domain)
	}

	return trimmed, nil
}

// customClaims holds Auth0-specific JWT claims beyond the standard registered set.
type customClaims struct {
	OrgID string `json:"org_id"`

	// Auth0 puts granted scopes in scope for plain client-credentials tokens and in
	// permissions when RBAC is enabled on the API, so both have to be consulted.
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

// hasScope reports whether the token was granted scope.
func (c *customClaims) hasScope(scope string) bool {
	return slices.Contains(strings.Fields(c.Scope), scope) || slices.Contains(c.Permissions, scope)
}

// Validate rejects claims that cannot produce an identity principal.
// Every token path goes through validateToken, so this is enforced everywhere.
func (c *customClaims) Validate() error {
	if c.OrgID == "" {
		return errors.New("org_id claim is required")
	}

	return nil
}

// Provider is an authentication provider backed by Auth0 JWTs.
type Provider struct {
	// refreshes deduplicates refreshes of the same refresh token, both while one is in
	// flight and for a short while after; see doRefresh. Nil when browser login is off.
	refreshes *cache.Cache[string, sessionPayload]

	verifier   *oidc.IDTokenVerifier // access tokens
	idVerifier *oidc.IDTokenVerifier // ID tokens from the browser flow; nil when browser login is off

	// oauth2 is nil when browser login is off, which is what BrowserLoginEnabled reports.
	oauth2 *oauth2.Config

	logger        *zap.Logger
	sessionCipher cipher.AEAD
	tenantURL     *url.URL
	audience      string
	callbackPath  string // path component of RedirectURL; the route and the state cookie both use it
	externalURL   string
	machineScope  string
}

// NewProvider creates a new Auth0 authentication provider.
// It validates config only and never reaches the network; ctx scopes the JWKS HTTP client.
func NewProvider(ctx context.Context, logger *zap.Logger, cfg Config) (*Provider, error) {
	if cfg.Domain == "" && cfg.IssuerURLOverride == "" {
		return nil, errors.New("auth0: domain must not be empty")
	}

	if cfg.Audience == "" {
		return nil, errors.New("auth0: audience must not be empty")
	}

	// Issuer URL doubles as the expected iss claim and as the base for every tenant endpoint.
	issuer := cfg.IssuerURLOverride
	if issuer == "" {
		domain, err := normalizeDomain(cfg.Domain)
		if err != nil {
			return nil, err
		}

		issuer = "https://" + domain + "/"
	}

	tenantURL, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("auth0: invalid issuer URL %q: %w", issuer, err)
	}

	providerLogger := logger.With(zap.String("component", "auth0-provider"))

	// Cloned so the JWKS client doesn't inherit later mutations of the process-wide pool.
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("auth0: http.DefaultTransport is not an *http.Transport")
	}

	// The key set is fetched lazily on the first token, so a tenant outage at startup is not fatal.
	keyCtx := oidc.ClientContext(ctx, &http.Client{
		Timeout: jwksFetchTimeout,
		Transport: &jwksTransport{
			next:    baseTransport.Clone(),
			limiter: rate.NewLimiter(rate.Every(jwksRefetchInterval), jwksRefetchBurst),
			logger:  providerLogger,
		},
	})
	keySet := oidc.NewRemoteKeySet(keyCtx, tenantURL.JoinPath("/.well-known/jwks.json").String())

	p := &Provider{
		verifier: oidc.NewVerifier(issuer, keySet, &oidc.Config{
			ClientID:             cfg.Audience,
			SupportedSigningAlgs: []string{oidc.RS256},
			Now:                  func() time.Time { return time.Now().Add(-clockSkew) },
		}),
		tenantURL:    tenantURL,
		audience:     cfg.Audience,
		externalURL:  cfg.ExternalURL,
		machineScope: cfg.MachineScope,
		logger:       providerLogger,
	}

	browserLogin, err := browserLoginConfigured(cfg)
	if err != nil {
		return nil, err
	}

	if browserLogin {
		if err = p.enableBrowserLogin(issuer, keySet, cfg); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// enableBrowserLogin validates and installs the authorization code + PKCE flow.
func (p *Provider) enableBrowserLogin(issuer string, keySet oidc.KeySet, cfg Config) error {
	if len(cfg.SessionKey) != 32 {
		return fmt.Errorf("auth0: session key must be exactly 32 bytes (got %d)", len(cfg.SessionKey))
	}

	// Auth0 only honors an absolute, allow-listed logout returnTo.
	if cfg.ExternalURL == "" {
		return errors.New("auth0: externalURL is required when browser login is enabled")
	}

	redirectURL, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return fmt.Errorf("auth0: invalid redirectURL %q: %w", cfg.RedirectURL, err)
	}

	if redirectURL.Scheme == "" || redirectURL.Host == "" || redirectURL.Path == "" || redirectURL.Path == "/" {
		return fmt.Errorf("auth0: redirectURL must be absolute and include a path, e.g. https://factory.example.com/callback (got %q)", cfg.RedirectURL)
	}

	// ":" and "*" mark wildcards to the router, so such a path would register a
	// pattern instead of the literal route Auth0 redirects to.
	if strings.ContainsAny(redirectURL.Path, ":*") {
		return fmt.Errorf("auth0: redirectURL path must not contain ':' or '*' (got %q)", redirectURL.Path)
	}

	block, err := aes.NewCipher(cfg.SessionKey)
	if err != nil {
		return fmt.Errorf("auth0: failed to create AES cipher: %w", err)
	}

	// Random-nonce GCM prepends the nonce to the ciphertext for us.
	if p.sessionCipher, err = cipher.NewGCMWithRandomNonce(block); err != nil {
		return fmt.Errorf("auth0: failed to create GCM cipher: %w", err)
	}

	p.callbackPath = redirectURL.Path
	p.idVerifier = oidc.NewVerifier(issuer, keySet, &oidc.Config{
		ClientID:             cfg.ClientID,
		SupportedSigningAlgs: []string{oidc.RS256},
		Now:                  func() time.Time { return time.Now().Add(-clockSkew) },
	})
	p.oauth2 = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		// offline_access is what makes Auth0 return a refresh token, letting the
		// middleware renew an expired session without bouncing the htmx requests
		// the UI wizard is built on through /login.
		Scopes: []string{oidc.ScopeOpenID, "offline_access"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  p.tenantURL.JoinPath("/authorize").String(),
			TokenURL: p.tenantURL.JoinPath("/oauth/token").String(),
			// Auth0 takes the client credentials in the form body; saying so avoids
			// the probe request x/oauth2 otherwise makes to find out.
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	p.refreshes = cache.New[string, sessionPayload](cache.Options{
		MetricsName: "image_factory_auth0_refresh_cache_size",
		MetricsHelp: "Number of in-flight and recently spent refresh tokens.",
		Capacity:    refreshResultCapacity,
	})

	return nil
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

// machineAllowed reports whether a machine-scoped token may make this request.
//
// Nodes pull installers through the OCI registry and boot artifacts from /image, and need
// nothing else. In particular the schematic body stays out of reach, so a credential sitting
// on a node cannot enumerate how the org's images are built.
func machineAllowed(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	return strings.HasPrefix(r.URL.Path, "/image/") ||
		r.URL.Path == "/v2" ||
		strings.HasPrefix(r.URL.Path, "/v2/")
}

// Run blocks until ctx is canceled, satisfying the enterprise.AuthProvider lifecycle.
// The key set is fetched on demand, so the only thing to run is the refresh cache's
// eviction loop, which drops spent refresh tokens rather than holding them to capacity.
func (p *Provider) Run(ctx context.Context) error {
	if p.refreshes != nil {
		go p.refreshes.Start() //nolint:errcheck // Start only returns a recovered panic, already logged by panicsafe

		defer p.refreshes.Stop()
	}

	<-ctx.Done()

	return nil
}

// Middleware implements enterprise.AuthProvider.
// The token comes from the Authorization header, then from the session cookie.
func (p *Provider) Middleware(next Handler) Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		logger := ctxlog.Logger(ctx, p.logger)

		tokenStr := extractToken(r)

		// Session cookie — only checked when browser login is configured.
		if tokenStr == "" && p.BrowserLoginEnabled() {
			token, err := p.sessionToken(ctx, w, r)
			if err != nil {
				// Left in place: an undecryptable cookie is overwritten by the next
				// login, and a failed refresh is often just a blip at the tenant.
				logger.Debug("auth0: unusable session cookie", zap.Error(err))
			}

			tokenStr = token
		}

		if tokenStr == "" {
			logger.Debug("auth0: authentication required: no token provided")

			return p.deny(w, r, "no token provided")
		}

		claims, err := p.validateToken(ctx, tokenStr)
		if err != nil {
			// Debug, not Warn: an unauthenticated caller controls how often this fires.
			logger.Debug("auth0: authentication failed", zap.Error(err))

			return p.deny(w, r, "invalid token")
		}

		ctx = auth.WithAuthUsername(ctx, claims.OrgID)

		// Written back to the request so the principal survives a denial below: next never
		// runs then, and the request context is the only channel the caller still sees.
		*r = *r.WithContext(ctx)

		// Forbidden rather than a challenge: the token is valid, it just isn't allowed here,
		// and re-authenticating with the same credential would not change that.
		if p.machineScope != "" && claims.hasScope(p.machineScope) && !machineAllowed(r) {
			logger.Debug("auth0: machine-scoped token denied",
				zap.String("org_id", claims.OrgID), zap.String("path", r.URL.Path))

			return xerrors.NewTagged[schematicpkg.ForbiddenTag](errors.New("machine-scoped token"))
		}

		logger.Debug("auth0: authenticated", zap.String("org_id", claims.OrgID))

		return next(ctx, w, r, params)
	}
}

// setChallenge advertises Basic ahead of Bearer, as separate header values so a parser
// cannot read "Bearer" as a parameter of the Basic challenge.
// Order matters: OCI clients take the first scheme they recognize, and only Basic carries
// a usable token here.
func setChallenge(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="Image Factory Enterprise", charset="UTF-8"`)
	w.Header().Add("WWW-Authenticate", `Bearer realm="Image Factory Enterprise"`)
}

// deny sends browsers to /login, preserving the URL they were trying to reach, and
// answers every other client with a 401 challenge.
func (p *Provider) deny(w http.ResponseWriter, r *http.Request, reason string) error {
	if p.BrowserLoginEnabled() {
		// htmx swaps a redirect's target into the page rather than navigating to it, so
		// it is told to navigate explicitly. It reads the header before the status, so
		// the 401 still stands; no challenge, or the browser pops its Basic auth dialog.
		if r.Header.Get("Hx-Request") == "true" {
			// The current URL is the page the user is on. The request URL is the XHR
			// endpoint, which is no use to come back to.
			w.Header().Set("Hx-Redirect", loginURL(r.Header.Get("Hx-Current-Url")))

			return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New(reason))
		}

		if isBrowserRequest(r) {
			// See Other rather than Found so a denied POST is not replayed at /login.
			// The two are equivalent for the GET that everything else here is.
			http.Redirect(w, r, loginURL(r.URL.RequestURI()), http.StatusSeeOther)

			// Tagged rather than nil so the audit log records a redirect for an
			// unauthenticated caller instead of a successful anonymous request.
			return xerrors.NewTagged[enterrors.RedirectedTag](fmt.Errorf("redirected to login: %s", reason))
		}
	}

	setChallenge(w)

	return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New(reason))
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

// validateToken validates the token and returns its claims, whose org_id is the identity
// principal, matching the htpasswd provider where the username maps to an org.
// This is the only path that turns a token into an identity, so every caller gets the
// signature, issuer, audience, expiry and org_id checks.
func (p *Provider) validateToken(ctx context.Context, tokenStr string) (*customClaims, error) {
	token, err := p.verifier.Verify(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("auth0: token validation failed: %w", err)
	}

	var claims customClaims

	if err = token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("auth0: failed to parse claims: %w", err)
	}

	if err = claims.Validate(); err != nil {
		return nil, fmt.Errorf("auth0: %w", err)
	}

	return &claims, nil
}
