// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package scanner provides an HTTP handler for downloading vulnerability scan reports.
package scanner

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	govexscanner "github.com/siderolabs/go-vex/pkg/scanner"

	"github.com/siderolabs/image-factory/enterprise/scanner/builder"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/schematic"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	enterprisefrontend "github.com/siderolabs/image-factory/pkg/enterprise/frontend"
)

// AuthProvider is a subset of enterprise.AuthProvider used for ownership checks.
// Defined locally to avoid an import cycle with pkg/enterprise.
type AuthProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// routePath is the HTTP route for vulnerability scan downloads.
//
// `:report` is the report filename (e.g. "report.sarif"); the extension selects
// the report format.
const routePath = "/scans/:schematic/:version/:arch/:report"

// availableFrom is the minimum Talos version with VEX data, and therefore the
// minimum version for which a scan can be produced.
var availableFrom = semver.MustParse("1.13.0")

// Frontend serves vulnerability scan reports over HTTP.
type Frontend struct {
	schematicFactory *schematic.Factory
	builder          *builder.Builder
	authProvider     AuthProvider
}

// NewFrontend wires a Frontend around a Builder.
func NewFrontend(schematicFactory *schematic.Factory, b *builder.Builder, auth AuthProvider) *Frontend {
	return &Frontend{
		schematicFactory: schematicFactory,
		builder:          b,
		authProvider:     auth,
	}
}

// Routes implements enterprise.FrontendPlugin.
func (f *Frontend) Routes() []enterprisefrontend.Route {
	return []enterprisefrontend.Route{
		{
			Method:      http.MethodGet,
			Path:        routePath,
			OperationID: "getVulnerabilityScan",
			Access:      enterprisefrontend.RouteAccessAuthenticated,
			Handler:     f.Handle,
		},
		{
			Method:      http.MethodHead,
			Path:        routePath,
			OperationID: "headVulnerabilityScan",
			Access:      enterprisefrontend.RouteAccessAuthenticated,
			Handler:     f.Handle,
		},
	}
}

// Ready implements enterprise.ReadinessChecker. Reports the readiness of the
// underlying Grype scanner DB.
func (f *Frontend) Ready() error {
	return f.builder.Ready()
}

// Handle implements enterprise.FrontendPlugin.
func (f *Frontend) Handle(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")
	if schematicID == "" {
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("missing schematic")
	}

	// Enforce ownership before doing any work. Scan reports and the underlying
	// SBOM bundle are cached server-side, so an unauthorized user must be rejected
	// here rather than relying on a downstream cache miss to trigger the check.
	if _, err := f.schematicFactory.Get(ctx, schematicID, f.authProvider); err != nil {
		return err
	}

	versionTag := p.ByName("version")
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	talosVersion, err := semver.Parse(versionTag[1:])
	if err != nil {
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("invalid version format: %q", versionTag)
	}

	if talosVersion.LT(availableFrom) {
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("scans are only available for Talos versions %s and later", availableFrom)
	}

	arch := p.ByName("arch")
	if !artifacts.ValidArch(arch) {
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("invalid architecture: %q", arch)
	}

	filename := p.ByName("report")

	format, err := parseFormat(filename)
	if err != nil {
		return xerrors.NewTaggedf[enterrors.InvalidErrorTag]("%s", err.Error())
	}

	data, err := f.builder.Build(ctx, schematicID, versionTag, arch, format)
	if err != nil {
		return fmt.Errorf("error building scan report: %w", err)
	}

	w.Header().Set("Content-Type", contentType(format))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s-%s-%s"`, schematicID, versionTag, arch, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	_, err = w.Write(data)

	return err
}

// parseFormat extracts the report format from a filename like "report.sarif".
func parseFormat(filename string) (govexscanner.ReportFormat, error) {
	ext := strings.TrimPrefix(path.Ext(filename), ".")
	if ext == "" {
		return 0, fmt.Errorf("missing report format extension in %q", filename)
	}

	return govexscanner.ParseReportFormat(ext)
}

func contentType(format govexscanner.ReportFormat) string {
	switch format {
	case govexscanner.ReportFormatSARIF:
		return "application/sarif+json"
	case govexscanner.ReportFormatCDX:
		return "application/vnd.cyclonedx+json"
	case govexscanner.ReportFormatJSON:
		return "application/json"
	case govexscanner.ReportFormatTable:
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
