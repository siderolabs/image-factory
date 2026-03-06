// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package spdx provides an HTTP handler for downloading SPDX bundles for Talos schematics.
package spdx

import (
	"context"
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
)

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
}

// NewFrontend creates a new Frontend instance.
func NewFrontend(schematicFactory *schematic.Factory, spdxBuilder *builder.Builder) *Frontend {
	return &Frontend{
		schematicFactory: schematicFactory,
		spdxBuilder:      spdxBuilder,
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

	// Validate schematic exists
	if _, err := f.schematicFactory.Get(ctx, schematicID); err != nil {
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
