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

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/image/verify"
	"github.com/siderolabs/image-factory/internal/schematic"
)

// FrontendPlugin is the interface that Enterprise code must implement to extend the frontend.
type FrontendPlugin interface {
	Methods() []string
	Path() string
	Handle(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error
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

	// PayloadHash returns a content-hash describing the inputs that determine
	// the SPDX bundle content (extension list, version, architecture). Schematics
	// with the same SBOM-relevant inputs share the same hash. Callers should use
	// this hash as a cache key.
	PayloadHash(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (string, error)
}

// ScannerOptions holds configuration options for the Scanner frontend.
type ScannerOptions struct {
	VEXSource        VEXSource
	SPDXSource       SPDXSource
	SchematicFactory *schematic.Factory
	AuthProvider     AuthProvider
	DatabaseURL      string
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

// Handler is the type of HTTP handlers used by the enterprise frontend.
type Handler = func(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error

// AuthProvider defines an authentication provider.
type AuthProvider interface {
	// Run starts the background reload loop and blocks until ctx is canceled.
	Run(ctx context.Context) error

	// Middleware returns an HTTP middleware that enforces authentication on the provided handler.
	Middleware(Handler) Handler

	// VerifyCredentials checks if the username/password pair is valid.
	VerifyCredentials(username, password string) bool

	// UsernameFromContext retrieves the authenticated username stored by the middleware.
	UsernameFromContext(ctx context.Context) (string, bool)
}
