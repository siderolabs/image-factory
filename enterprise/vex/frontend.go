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
	"net/http"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-factory/enterprise/vex/builder"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	enterprisefrontend "github.com/siderolabs/image-factory/pkg/enterprise/frontend"
)

const vexJSONMediaType = "application/json"

const routePath = "/vex/:version/vex.json"

var availableFrom = semver.MustParse("1.13.0")

// Frontend serves VEX documents over HTTP. It delegates document generation and caching to a Builder.
type Frontend struct {
	builder *builder.Builder
}

// NewFrontend wires a Frontend around a Builder.
func NewFrontend(b *builder.Builder) *Frontend {
	return &Frontend{builder: b}
}

// Routes implements enterprise.FrontendPlugin.
func (f *Frontend) Routes() []enterprisefrontend.Route {
	return []enterprisefrontend.Route{
		{
			Method:      http.MethodGet,
			Path:        routePath,
			OperationID: "getVEX",
			Access:      enterprisefrontend.RouteAccessAuthenticated,
			Handler:     f.Handle,
		},
		{
			Method:      http.MethodHead,
			Path:        routePath,
			OperationID: "headVEX",
			Access:      enterprisefrontend.RouteAccessAuthenticated,
			Handler:     f.Handle,
		},
	}
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
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("invalid version format: %q", versionTag)
	}

	if talosVersion.LT(availableFrom) {
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("VEX documents are only available for Talos versions %s and later", availableFrom)
	}

	data, err := f.builder.Build(ctx, versionTag)
	if err != nil {
		return fmt.Errorf("error building VEX document: %w", err)
	}

	w.Header().Set("Content-Type", vexJSONMediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.vex.json"`, versionTag))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	_, err = w.Write(data)

	return err
}
