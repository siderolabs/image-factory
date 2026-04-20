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
	"github.com/siderolabs/image-factory/internal/schematic"
)

// FrontendPlugin is the interface that Enterprise code must implement to extend the frontend.
type FrontendPlugin interface {
	Methods() []string
	Path() string
	Handle(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error
}

// SPDXOptions holds configuration options for the SPDX frontend.
type SPDXOptions struct {
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

// ErrNotEnabledTag tags errors that occur when an enterprise feature is
// requested but the enterprise build tag is not active.
type ErrNotEnabledTag struct{}

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
