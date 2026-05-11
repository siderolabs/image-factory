// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Package builder runs Grype-backed vulnerability scans against the Talos SBOM.
package builder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anchore/clio"
	"github.com/anchore/grype/grype/db/v6/distribution"
	"github.com/anchore/grype/grype/db/v6/installation"
	"github.com/anchore/grype/grype/presenter/models"
	"github.com/anchore/syft/syft/sbom"
	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/xerrors"
	govexscanner "github.com/siderolabs/go-vex/pkg/scanner"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/cache"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
)

// ErrNotReady is returned when the Grype DB has not finished initializing.
var ErrNotReady = xerrors.NewTagged[enterrors.NotReadyTag](errors.New("scanner not ready"))

// scannerID is the identifier embedded in scan reports.
const scannerID = "image-factory"

// ScanTimeout caps a single end-to-end scan (SBOM fetch + VEX fetch + Grype match).
const ScanTimeout = 15 * time.Minute

// VEXSource produces a VEX JSON document for a given Talos version tag.
type VEXSource interface {
	Build(ctx context.Context, versionTag string) ([]byte, error)
}

// SPDXSource produces a merged SPDX JSON document for a schematic+version+arch.
// It must enforce ownership before returning bytes.
type SPDXSource interface {
	Build(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (io.ReadCloser, error)
}

// Options configures the Builder.
type Options struct {
	VEXSource        VEXSource
	SPDXSource       SPDXSource
	DatabaseURL      string
	MetricsNamespace string
	CacheTTL         time.Duration
	Capacity         uint64
}

// Builder runs Grype-backed scans of the vanilla Talos SBOM, applies VEX
// suppressions, and caches the resulting Document per Talos version. The
// rendered report is produced on-demand from the cached Document so that
// switching formats does not retrigger a full scan.
type Builder struct {
	scanner    atomic.Pointer[govexscanner.Scanner]
	initErr    atomic.Pointer[error]
	initDone   chan struct{}
	vexSource  VEXSource
	spdxSource SPDXSource
	logger     *zap.Logger
	c          *cache.Cache[string, cachedScan]
	cacheTTL   time.Duration
}

type cachedScan struct {
	document *models.Document
	sbom     *sbom.SBOM
}

// NewBuilder constructs a Builder.
//
// The Grype vulnerability database is loaded asynchronously so that startup is
// not blocked by the multi-second DB warm-up. Until initialization completes,
// Build returns ErrNotReady and Ready reports the in-progress state.
func NewBuilder(logger *zap.Logger, opts Options) *Builder {
	b := &Builder{
		vexSource:  opts.VEXSource,
		spdxSource: opts.SPDXSource,
		cacheTTL:   opts.CacheTTL,
		c: cache.New[string, cachedScan](cache.Options{
			MetricsNamespace: opts.MetricsNamespace,
			MetricsName:      "image_factory_scanner_cache_size",
			MetricsHelp:      "Number of vulnerability scan results in in-memory cache.",
			Capacity:         opts.Capacity,
		}),
		logger:   logger.With(zap.String("component", "scanner-builder")),
		initDone: make(chan struct{}),
	}

	go b.initScanner(opts)

	return b
}

func (b *Builder) initScanner(opts Options) {
	defer close(b.initDone)

	scannerOpts := govexscanner.Options{ID: scannerID}

	if opts.DatabaseURL != "" {
		distConfig := distribution.DefaultConfig()
		distConfig.LatestURL = opts.DatabaseURL

		instConfig := installation.DefaultConfig(clio.Identification{Name: scannerID})
		instConfig.DBRootDir = "/var/lib/grype"

		scannerOpts.Distribution = &distConfig
		scannerOpts.Installation = &instConfig
	}

	b.logger.Info("initializing grype scanner", zap.String("databaseURL", opts.DatabaseURL))

	sc, err := govexscanner.NewScanner(scannerOpts)
	if err != nil {
		wrapped := fmt.Errorf("error initializing grype scanner: %w", err)
		b.initErr.Store(&wrapped)
		b.logger.Error("grype scanner init failed", zap.Error(wrapped))

		return
	}

	b.scanner.Store(sc)
	b.logger.Info("grype scanner ready")
}

// Start runs the cache eviction goroutine; should be invoked in a goroutine.
func (b *Builder) Start() error {
	return b.c.Start()
}

// Ready reports whether the underlying Grype scanner has finished initializing.
// It returns nil when ready, ErrNotReady while initialization is in progress,
// and the init error if initialization failed permanently.
func (b *Builder) Ready() error {
	if b.scanner.Load() != nil {
		return nil
	}

	if errPtr := b.initErr.Load(); errPtr != nil {
		return *errPtr
	}

	return ErrNotReady
}

// Stop releases the Grype DB handle and stops cache eviction. Waits for
// in-flight init to settle so the underlying handle is not leaked on shutdown.
func (b *Builder) Stop() error {
	<-b.initDone

	b.c.Stop()

	sc := b.scanner.Load()
	if sc == nil {
		return nil
	}

	return sc.Close()
}

// Build returns a scan report formatted as the requested format for the given
// schematic, Talos version and architecture. SBOM ownership is enforced by the
// underlying SPDXSource before the scan runs, so a caller without access to the
// schematic will see an authorization error rather than a generated report.
//
// Returns ErrNotReady (or the persistent init error) if the Grype DB has not
// yet finished initializing.
func (b *Builder) Build(ctx context.Context, schematicID, versionTag, arch string, format govexscanner.ReportFormat) ([]byte, error) {
	if err := b.Ready(); err != nil {
		return nil, err
	}

	doc, sbomDoc, err := b.scan(ctx, schematicID, versionTag, arch)
	if err != nil {
		return nil, err
	}

	return renderReport(*doc, sbomDoc, format)
}

func (b *Builder) scan(ctx context.Context, schematicID, versionTag, arch string) (*models.Document, *sbom.SBOM, error) {
	key := cacheKey(schematicID, versionTag, arch)

	if item := b.c.TTL.Get(key); item != nil && !item.IsExpired() {
		entry := item.Value()

		return entry.document, entry.sbom, nil
	}

	// Capture the authenticated username from the request context so that the
	// detached singleflight context can carry it forward to downstream ownership
	// checks (the SPDX builder re-verifies access against the schematic owner).
	username, _ := auth.GetAuthUsername(ctx)

	resultCh := b.c.SF.DoChan(key, func() (any, error) { //nolint:contextcheck
		return b.scanAndCache(username, schematicID, versionTag, arch, key)
	})

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case res := <-resultCh:
		if res.Err != nil {
			return nil, nil, res.Err
		}

		entry, ok := res.Val.(cachedScan)
		if !ok {
			return nil, nil, fmt.Errorf("unexpected result type: %T", res.Val)
		}

		return entry.document, entry.sbom, nil
	}
}

func cacheKey(schematicID, versionTag, arch string) string {
	return schematicID + "|" + versionTag + "|" + arch
}

// scanAndCache runs under singleflight with a detached context.
func (b *Builder) scanAndCache(username, schematicID, versionTag, arch, key string) (cachedScan, error) {
	baseCtx := context.Background()
	if username != "" {
		baseCtx = auth.WithAuthUsername(baseCtx, username)
	}

	ctx, cancel := context.WithTimeout(baseCtx, ScanTimeout)
	defer cancel()

	logger := b.logger.With(
		zap.String("schematic", schematicID),
		zap.String("version", versionTag),
		zap.String("arch", arch),
	)
	logger.Info("running vulnerability scan")

	if _, err := semver.Parse(strings.TrimPrefix(versionTag, "v")); err != nil {
		return cachedScan{}, fmt.Errorf("invalid version: %w", err)
	}

	r, err := b.spdxSource.Build(ctx, schematicID, versionTag, artifacts.Arch(arch))
	if err != nil {
		return cachedScan{}, fmt.Errorf("error building Talos SBOM: %w", err)
	}

	sbomBytes, err := io.ReadAll(r)
	if err != nil {
		return cachedScan{}, fmt.Errorf("error reading SBOM bytes: %w", err)
	}

	vexBytes, err := b.vexSource.Build(ctx, versionTag)
	if err != nil {
		return cachedScan{}, fmt.Errorf("error fetching VEX document: %w", err)
	}

	workDir, err := os.MkdirTemp("", "image-factory-scan-*")
	if err != nil {
		return cachedScan{}, fmt.Errorf("error creating temp dir: %w", err)
	}
	defer os.RemoveAll(workDir) //nolint:errcheck

	sbomPath := filepath.Join(workDir, "talos.spdx.json")
	if err = os.WriteFile(sbomPath, sbomBytes, 0o600); err != nil {
		return cachedScan{}, fmt.Errorf("error writing SBOM: %w", err)
	}

	vexPath := filepath.Join(workDir, "vex.json")
	if err = os.WriteFile(vexPath, vexBytes, 0o600); err != nil {
		return cachedScan{}, fmt.Errorf("error writing VEX: %w", err)
	}

	now := time.Now()

	sc := b.scanner.Load()
	if sc == nil {
		return cachedScan{}, ErrNotReady
	}

	doc, sbomDoc, err := sc.ScanSBOM(sbomPath, &now, vexPath)
	if err != nil {
		return cachedScan{}, fmt.Errorf("error scanning SBOM: %w", err)
	}

	logger.Info("scan complete", zap.Int("matches", len(doc.Matches)))

	entry := cachedScan{
		document: doc,
		sbom:     sbomDoc,
	}

	b.c.TTL.Set(key, entry, b.cacheTTL)

	return entry, nil
}

// Describe implements prom.Collector interface.
func (b *Builder) Describe(ch chan<- *prometheus.Desc) {
	b.c.Describe(ch)
}

// Collect implements prom.Collector interface.
func (b *Builder) Collect(ch chan<- prometheus.Metric) {
	b.c.Collect(ch)
}

var _ prometheus.Collector = (*Builder)(nil)

// renderReport formats a scan Document into the requested grype output format,
// returning the encoded bytes ready to be written to the HTTP response.
//
// The Anchore presenters expose only a file-path Present API, so we materialize
// the report into a temporary file and read it back.
func renderReport(doc models.Document, s *sbom.SBOM, format govexscanner.ReportFormat) ([]byte, error) {
	f, err := os.CreateTemp("", "image-factory-report-*")
	if err != nil {
		return nil, fmt.Errorf("error creating temp report: %w", err)
	}

	path := f.Name()
	if err = f.Close(); err != nil {
		return nil, fmt.Errorf("error closing temp report: %w", err)
	}

	defer os.Remove(path) //nolint:errcheck

	if err = govexscanner.FormatReport(doc, path, s, format); err != nil {
		return nil, fmt.Errorf("error formatting report: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading rendered report: %w", err)
	}

	return data, nil
}
