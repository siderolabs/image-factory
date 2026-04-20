// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package spdx provides an HTTP handler for downloading SPDX bundles for Talos schematics.
package spdx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/schematic"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// authProvider is a subset of enterprise.AuthProvider used for ownership checks.
// Defined locally to avoid an import cycle with pkg/enterprise.
type authProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// spdxJSONMediaType is the IANA-registered media type for SPDX JSON documents.
const spdxJSONMediaType = "application/spdx+json"

// routePath is the HTTP route for SPDX bundle downloads.
const routePath = "/spdx/:schematic/:version/:arch"

// availableFrom is the minimum Talos version that supports SPDX bundle downloads.
var availableFrom = semver.MustParse("1.11.0")

// Frontend is the HTTP handler for SPDX bundle downloads.
type Frontend struct {
	schematicFactory *schematic.Factory
	spdxBuilder      *builder.Builder
	authProvider     authProvider
}

// NewFrontend creates a new Frontend instance.
func NewFrontend(schematicFactory *schematic.Factory, spdxBuilder *builder.Builder, auth authProvider) *Frontend {
	return &Frontend{
		schematicFactory: schematicFactory,
		spdxBuilder:      spdxBuilder,
		authProvider:     auth,
	}
}

// Path implements enterprise.FrontendExtension.
func (f *Frontend) Path() string {
	return routePath
}

// Methods implements enterprise.FrontendExtension.
func (f *Frontend) Methods() []string {
	return []string{http.MethodGet, http.MethodHead}
}

// checkOwnership verifies the context user owns the schematic.
// Returns nil if schematic has no owner or the authenticated user matches the owner.
func (f *Frontend) checkOwnership(ctx context.Context, s *schematicpkg.Schematic) error {
	if s.Owner == "" {
		return nil
	}

	if f.authProvider == nil {
		return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
	}

	username, ok := f.authProvider.UsernameFromContext(ctx)
	if !ok {
		return xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
	}

	if username != s.Owner {
		return xerrors.NewTagged[schematicpkg.ForbiddenTag](errors.New("access denied"))
	}

	return nil
}

// Handle implements enterprise.FrontendExtension.
// It handles SPDX bundle download requests.
//
// The endpoint returns a merged SPDX 2.3 JSON document containing all packages
// from the Talos installer and extensions for the given schematic and version.
// The document can be consumed directly by vulnerability scanners such as grype:
//
//	grype sbom:response.spdx.json
func (f *Frontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

	// Validate schematic exists and check ownership.
	schematic, err := f.schematicFactory.Get(ctx, schematicID)
	if err != nil {
		return err
	}

	if err = f.checkOwnership(ctx, schematic); err != nil {
		return err
	}

	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	// Validate version format
	talosVersion, err := semver.Parse(versionTag[1:])
	if err != nil {
		return xerrors.NewTaggedf[profile.InvalidErrorTag]("invalid version format: %q", versionTag)
	}

	if talosVersion.LT(availableFrom) {
		return xerrors.NewTaggedf[profile.InvalidErrorTag]("SPDX bundles are only available for Talos versions %s and later", availableFrom)
	}

	// Validate architecture
	arch := p.ByName("arch")
	if !artifacts.ValidArch(arch) {
		return xerrors.NewTaggedf[profile.InvalidErrorTag]("invalid architecture: %q", arch)
	}

	// Build/retrieve SPDX bundle
	bundle, err := f.spdxBuilder.Build(ctx, schematicID, versionTag, artifacts.Arch(arch))
	if err != nil {
		return err
	}

	// Set response headers
	w.Header().Set("Content-Type", spdxJSONMediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s-%s.spdx.json"`, schematicID, versionTag, arch))
	w.Header().Set("Content-Length", strconv.FormatInt(bundle.Size(), 10))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	// Stream response
	reader, err := bundle.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	_, err = io.Copy(w, reader)

	return err
}
