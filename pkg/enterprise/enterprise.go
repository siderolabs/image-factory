// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package enterprise provide glue to Enterprise code.
package enterprise

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/julienschmidt/httprouter"

	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	assetcache "github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/image/verify"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/schematic"
)

// FrontendPlugin is the interface that Enterprise code must implement to extend the frontend.
type FrontendPlugin interface {
	Methods() []string
	Path() string
	Handle(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error
}

// PublicRoute is implemented by FrontendPlugin instances whose routes should
// be registered without authentication. Plugins that do not implement this
// interface are registered as auth-protected routes.
type PublicRoute interface {
	PublicRoute()
}

// ReadinessChecker is implemented by FrontendPlugin instances whose readiness
// must factor into the /readyz response. Plugins that do not implement this
// interface are considered always ready.
type ReadinessChecker interface {
	// Ready reports nil when the plugin is ready to serve requests, or a
	// non-nil error describing why it is not.
	Ready() error
}

// SPDXOptions holds configuration options for the SPDX frontend.
type SPDXOptions struct {
	ExternalURL             string
	CacheImageSigner        signer.Signer
	SchematicFactory        *schematic.Factory
	ArtifactsManager        *artifacts.Manager
	AssetBuilder            *asset.Builder
	AuthProvider            AuthProvider
	CacheRepository         string
	RemoteOptions           []remote.Option
	RegistryRefreshInterval time.Duration
	CacheInsecure           bool
}

// VEXOptions holds configuration options for the VEX frontend.
type VEXOptions struct {
	Data             string
	MetricsNamespace string
	RemoteOptions    []remote.Option
	VerifyOptions    verify.VerifyOptions
	RefreshInterval  time.Duration
	CacheTTL         time.Duration
	CacheCapacity    uint64
	DataInsecure     bool
}

// VEXSource produces a VEX JSON document for a given Talos version tag.
//
// The VEX builder satisfies this interface and is reused by the scanner frontend
// to suppress vulnerabilities classified as "fixed"/"not_affected" upstream.
type VEXSource interface {
	Build(ctx context.Context, versionTag string) ([]byte, error)
}

// SPDXSource produces a merged SPDX JSON document for the requested schematic,
// Talos version and architecture, applying ownership enforcement.
//
// The SPDX builder satisfies this interface and is reused by the scanner frontend
// so the SBOM extraction and access control live in one place.
type SPDXSource interface {
	Build(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (io.ReadCloser, error)

	// BuildBytes returns the canonical SPDX 2.3 JSON document used as an
	// Installer image attestation predicate.
	BuildBytes(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) ([]byte, error)

	// PayloadHash returns a content-hash describing the inputs that determine
	// the SPDX bundle content (extension list, version, architecture). Schematics
	// with the same SBOM-relevant inputs share the same hash. Callers should use
	// this hash as a cache key.
	PayloadHash(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (string, error)
}

// InstallerEvidencePublisher publishes and verifies the mandatory evidence graph for an Installer index.
type InstallerEvidencePublisher interface {
	Publish(ctx context.Context, input installer.EvidenceInput) error
	Verify(ctx context.Context, input installer.EvidenceInput) error
}

// ScannerOptions holds configuration options for the Scanner frontend.
type ScannerOptions struct {
	VEXSource        VEXSource
	SPDXSource       SPDXSource
	SchematicFactory *schematic.Factory
	AuthProvider     AuthProvider
	DatabaseURL      string
	DatabaseUpdateAt string
	DatabaseRootDir  string
	MetricsNamespace string
	CacheTTL         time.Duration
	CacheCapacity    uint64
}

// Checksummer computes a checksum for a boot asset and writes the result to
// the HTTP response.  The implementation lives behind the enterprise build tag;
// when enterprise is not enabled the Frontend receives a nil Checksummer and
// returns 402 for checksum requests.
//
// suffix is the file-extension that triggered checksum mode (e.g. ".sha512",
// ".sha256", ".md5") and determines both the algorithm and the output filename.
type Checksummer interface {
	WriteChecksum(ctx context.Context, w http.ResponseWriter, r *http.Request, reader io.ReadCloser, size int64, filename, suffix string) error
}

// SignatureWriter signs an asset and writes its detached Sigstore bundle to the HTTP response.
// The implementation is enterprise-only and supports any configured blob signer.
type SignatureWriter interface {
	WriteSignature(ctx context.Context, w http.ResponseWriter, r *http.Request, asset assetcache.BootAsset, assetKey, filename string) error
}

// TokenTTL configures the CLI bootstrap credential lifetime.
type TokenTTL = apitoken.TTL

// TokenStorageTTL configures ordinary token lifetimes by whether the factory records them.
type TokenStorageTTL = apitoken.StorageTTL

// MintedToken is a freshly issued API token: the signed string, plus the lifetime and jti the
// issuer settled on.
type MintedToken = apitoken.Token

// TokenClaims are the verified contents of an API token: who it authenticates, what it reaches,
// and whether the factory keeps a revocable record of it.
type TokenClaims = apitoken.Claims

// TokenVerifier reports the claims a bearer credential authenticates, or ok=false if it isn't a
// currently valid, self-issued API token.
type TokenVerifier interface {
	Verify(ctx context.Context, tokenStr string) (claims TokenClaims, ok bool)
}

// TokenOptions holds configuration for self-issued API token issuance, storage, and
// verification.
type TokenOptions struct {
	StorageRepository string
	KeyPaths          []string
	RemoteOptions     []remote.Option
	BootstrapTTL      TokenTTL
	StorageTTL        TokenStorageTTL
	RefreshInterval   time.Duration
	MaxPerOrg         int
	StorageInsecure   bool
}

// Handler is the type of HTTP handlers used by the enterprise frontend.
type Handler = func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

// AuthProvider defines an authentication provider.
type AuthProvider interface {
	// Run starts the background reload loop and blocks until ctx is canceled.
	Run(ctx context.Context) error

	// Middleware returns an HTTP middleware that enforces authentication on the provided handler.
	//
	// A provider that authenticates a caller and then refuses the request must leave the
	// principal on the request context, since the wrapped handler never runs and the audit
	// record would otherwise attribute the denial to nobody.
	Middleware(Handler) Handler

	// UsernameFromContext retrieves the authenticated username stored by the middleware.
	UsernameFromContext(ctx context.Context) (string, bool)

	// ContextWithUsername returns a context carrying the given username as if
	// the middleware had set it. Used by the API token path to inject the
	// JWT subject so that downstream ownership checks work normally.
	ContextWithUsername(ctx context.Context, username string) context.Context
}

// Auth0SessionKeySize is the AES-256 key length the Auth0 session cookies are sealed with.
// Restated here rather than imported because this package imports the provider, not the
// other way round.
const Auth0SessionKeySize = 32

// Auth0Config holds configuration for the Auth0 authentication provider.
//
// This restates cmd.Auth0Options rather than reusing it because auth0.Config lives behind
// the enterprise build tag, so cmd cannot name it; this struct is the seam between them.
type Auth0Config struct {
	Domain   string
	Audience string

	// Browser login, additive on top of the bearer-token validation Domain and Audience
	// always enable. These two and SessionKey are all-or-nothing; a partial set fails at
	// startup.
	ClientID     string
	ClientSecret string // inject via IF_AUTHENTICATION_AUTH0_CLIENTSECRET

	// ExternalURL is http.externalURL, required regardless of the group above. The callback
	// URL and the logout returnTo are derived from it.
	ExternalURL string

	// IssuerURLOverride replaces the default issuer URL constructed from Domain.
	// It sets the expected iss claim and the JWKS, authorize and token endpoints.
	// Intended for testing only; leave empty in production.
	IssuerURLOverride string

	// SessionKey is a 32-byte AES-256 key for the session and PKCE state cookies.
	// Must be shared by all replicas, since cookies issued by one are read by another.
	SessionKey []byte
}

// BrowserLoginProvider is an optional extension of AuthProvider for providers that can sign
// a browser in. The frontend registers its routes as public.
type BrowserLoginProvider interface {
	// BrowserLoginEnabled reports whether the flow is configured.
	BrowserLoginEnabled() bool

	// LoginHandler returns the handler for GET /login.
	LoginHandler() Handler

	// CallbackHandler returns the handler the identity provider redirects back to.
	CallbackHandler() Handler

	// CallbackPath returns the route CallbackHandler must be registered on, fixed by the
	// allow-listed redirect URL.
	CallbackPath() string

	// LogoutHandler returns the handler for /logout, registered on GET and POST.
	LogoutHandler() Handler
}
