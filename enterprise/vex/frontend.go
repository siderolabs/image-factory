// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package vex provides an HTTP handler for downloading Vulnerability Exploitability eXchange (VEX) documents.
package vex

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

	"github.com/siderolabs/image-factory/internal/profile"
)

// vexJSONMediaType is the IANA-registered media type for VEX JSON documents.
const vexJSONMediaType = "application/json"

// routePath is the HTTP route for VEX document downloads.
const routePath = "/vex/:version/vex.json"

// availableFrom is the minimum Talos version that supports VEX downloads.
var availableFrom = semver.MustParse("1.11.0")

// Frontend is the HTTP handler for VEX downloads.
type Frontend struct {
	// TODO: add TTL-bound cache for generated VEX bundles, with configurable TTL.
	// Thread safe, size limited, and with LRU eviction policy.
}

// NewFrontend creates a new Frontend instance.
func NewFrontend() *Frontend {
	return &Frontend{}
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
// It handles VEX document download requests for a specific Talos version.
//
// The document can be consumed directly by vulnerability scanners such as grype:
//
//	grype sbom:talos.spdx.json --vex v1.13.0.vex.json
func (f *Frontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
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

	// TODO: generate vex bundle
	var vex = struct {
		Size   func() int64
		Reader func() (io.ReadCloser, error)
	}{
		Size: func() int64 {
			return 0
		},
		Reader: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("{}")), nil
		},
	}

	// Set response headers
	w.Header().Set("Content-Type", vexJSONMediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.vex.json"`, versionTag))
	w.Header().Set("Content-Length", strconv.FormatInt(vex.Size(), 10))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	// Stream response
	reader, err := vex.Reader()
	if err != nil {
		return err
	}

	defer reader.Close() //nolint:errcheck

	_, err = io.Copy(w, reader)

	return err
}
