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
	"sync"
	"sync/atomic"
	"time"

	"github.com/anchore/clio"
	"github.com/anchore/grype/grype"
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
	scanlogger "github.com/siderolabs/image-factory/enterprise/scanner/logger"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/cache"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/schedule"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
)

// ErrNotReady is returned when the Grype DB has not finished initializing.
var ErrNotReady = xerrors.NewTagged[enterrors.NotReadyTag](errors.New("scanner not ready"))

const (
	// scannerID is the identifier embedded in scan reports.
	scannerID = "image-factory"

	// dbRootDir is where the Grype vulnerability database is installed. The Helm
	// chart mounts a volume here, so with a persistent volume the DB (and
	// therefore the scan results) survive a restart.
	dbRootDir = "/var/lib/grype"
)

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
	PayloadHash(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (string, error)
}

// Options configures the Builder.
type Options struct {
	VEXSource        VEXSource
	SPDXSource       SPDXSource
	DatabaseURL      string
	DatabaseUpdateAt string
	MetricsNamespace string
	CacheTTL         time.Duration
	Capacity         uint64
}

// Builder runs Grype-backed scans of the vanilla Talos SBOM, applies VEX
// suppressions, and caches the resulting Document per Talos version. The
// rendered report is produced on-demand from the cached Document so that
// switching formats does not retrigger a full scan.
type Builder struct {
	scanner  atomic.Pointer[govexscanner.Scanner]
	initErr  atomic.Pointer[error]
	initDone chan struct{}

	// stop signals the DB refresh loop to exit, refreshDone is closed once it has.
	stop        chan struct{}
	refreshDone chan struct{}

	vexSource  VEXSource
	spdxSource SPDXSource
	logger     *zap.Logger
	c          *cache.Cache[string, cachedScan]
	metricDB   prometheus.Gauge

	// dbMu is held for reading while a scan matches against the loaded DB, and
	// for writing while the DB is swapped, so a scheduled update never closes
	// the vulnerability provider from under an in-flight scan. The atomic
	// pointer above stays for lock-free readiness checks.
	dbMu sync.RWMutex

	cacheTTL time.Duration
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
//
// When opts.DatabaseUpdateAt is set, the DB is refreshed daily at that time of
// day (see refreshLoop) instead of only at start-up, so replicas started at
// different times converge on the same DB build rather than each keeping the one
// that was latest when it happened to boot — the reason the same report could
// differ between pods.
func NewBuilder(logger *zap.Logger, opts Options) (*Builder, error) {
	updateAt, err := schedule.ParseTimeOfDay(opts.DatabaseUpdateAt)
	if err != nil {
		return nil, err
	}

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
		metricDB: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: opts.MetricsNamespace,
			Name:      "image_factory_scanner_db_built_timestamp_seconds",
			Help:      "Build timestamp of the Grype vulnerability database currently loaded (0 if none).",
		}),
		logger:      logger.With(zap.String("component", "scanner-builder")),
		initDone:    make(chan struct{}),
		stop:        make(chan struct{}),
		refreshDone: make(chan struct{}),
	}

	grype.SetLogger(scanlogger.New(logger.With(zap.String("component", "grype"))))

	go b.initScanner(opts)
	go b.refreshLoop(opts, updateAt)

	return b, nil
}

// dbConfig returns the Grype distribution and installation configuration.
func dbConfig(opts Options) (distribution.Config, installation.Config) {
	distConfig := distribution.DefaultConfig()
	if opts.DatabaseURL != "" {
		distConfig.LatestURL = opts.DatabaseURL
	}

	instConfig := installation.DefaultConfig(clio.Identification{Name: scannerID})
	instConfig.DBRootDir = dbRootDir

	// We decide when to update, so drop Grype's own low-pass filter: it would
	// silently skip a scheduled update that follows a start-up download too closely.
	instConfig.UpdateCheckMaxFrequency = 0

	return distConfig, instConfig
}

// loadDB loads the vulnerability database, updating it from the configured
// distribution first. It returns the scanner and the build timestamp of the DB,
// which is also what reports carry as descriptor.db.
func loadDB(opts Options) (*govexscanner.Scanner, time.Time, error) {
	distConfig, instConfig := dbConfig(opts)

	sc, err := govexscanner.NewScanner(govexscanner.Options{
		ID:           scannerID,
		Distribution: &distConfig,
		Installation: &instConfig,
	})
	if err != nil {
		return nil, time.Time{}, err
	}

	var built time.Time

	if status := sc.DatabaseStatus().Status; status != nil {
		built = status.Built
	}

	return sc, built, nil
}

func (b *Builder) initScanner(opts Options) {
	defer close(b.initDone)

	b.logger.Info("initializing grype scanner",
		zap.String("databaseURL", opts.DatabaseURL),
		zap.String("updateAt", opts.DatabaseUpdateAt),
	)

	sc, built, err := loadDB(opts)
	if err != nil {
		wrapped := fmt.Errorf("error initializing grype scanner: %w", err)
		b.initErr.Store(&wrapped)
		b.logger.Error("grype scanner init failed", zap.Error(wrapped))

		return
	}

	b.setScanner(sc, built)
	b.logger.Info("grype scanner ready", zap.Time("dbBuilt", built))
}

// setScanner installs a freshly loaded scanner, closing the one it replaces.
func (b *Builder) setScanner(sc *govexscanner.Scanner, built time.Time) {
	b.dbMu.Lock()
	defer b.dbMu.Unlock()

	old := b.scanner.Swap(sc)

	b.metricDB.Set(float64(built.Unix()))

	if old == nil {
		return
	}

	// Cached documents were produced by the DB being replaced, so drop them.
	// Scans already running still write their (old-DB) result afterwards; those
	// age out with the normal cache TTL.
	b.c.TTL.DeleteAll()

	if err := old.Close(); err != nil {
		b.logger.Warn("error closing previous grype DB", zap.Error(err))
	}
}

// refreshLoop updates the vulnerability DB once a day at updateAt, keeping the
// current DB if the update fails. It is a no-op when no schedule is configured.
func (b *Builder) refreshLoop(opts Options, updateAt time.Duration) {
	defer close(b.refreshDone)

	if opts.DatabaseUpdateAt == "" {
		return
	}

	// don't race the initial load for the same DB directory.
	select {
	case <-b.stop:
		return
	case <-b.initDone:
	}

	for {
		delay := schedule.UntilNext(time.Now(), updateAt)

		b.logger.Info("grype DB update scheduled", zap.Duration("in", delay))

		select {
		case <-b.stop:
			return
		case <-time.After(delay):
		}

		sc, built, err := loadDB(opts)
		if err != nil {
			b.logger.Error("grype DB update failed, keeping current database", zap.Error(err))

			continue
		}

		b.setScanner(sc, built)
		b.logger.Info("grype DB updated", zap.Time("dbBuilt", built))
	}
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
// in-flight init and the refresh loop to settle so the underlying handle is not
// leaked on shutdown.
func (b *Builder) Stop() error {
	close(b.stop)

	<-b.initDone
	<-b.refreshDone

	b.c.Stop()

	b.dbMu.Lock()
	defer b.dbMu.Unlock()

	sc := b.scanner.Swap(nil)
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
	// Derive the cache key from only the inputs that affect the SBOM content,
	// so schematics that share the same extension list, version and arch
	// reuse scan results.
	sbomHash, err := b.spdxSource.PayloadHash(ctx, schematicID, versionTag, artifacts.Arch(arch))
	if err != nil {
		return nil, nil, err
	}

	if item := b.c.TTL.Get(sbomHash); item != nil && !item.IsExpired() {
		entry := item.Value()

		return entry.document, entry.sbom, nil
	}

	// Capture the authenticated username from the request context so that the
	// detached singleflight context can carry it forward to downstream ownership
	// checks (the SPDX builder re-verifies access against the schematic owner).
	username, _ := auth.GetAuthUsername(ctx)

	// carry the request ID into the detached scan so its logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)

	resultCh := b.c.SF.DoChan(sbomHash, func() (any, error) { //nolint:contextcheck
		return b.scanAndCache(reqID, username, schematicID, versionTag, arch, sbomHash)
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

// scanAndCache runs under singleflight with a detached context.
//
// reqID is the request ID, carried into the detached context so the scan logs
// (and downstream SPDX/VEX builds) keep the request_id.
func (b *Builder) scanAndCache(reqID, username, schematicID, versionTag, arch, key string) (cachedScan, error) {
	baseCtx := ctxlog.WithRequestID(context.Background(), reqID)
	if username != "" {
		baseCtx = auth.WithAuthUsername(baseCtx, username)
	}

	ctx, cancel := context.WithTimeout(baseCtx, ScanTimeout)
	defer cancel()

	logger := ctxlog.Logger(ctx, b.logger).With(
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

	doc, sbomDoc, err := b.scanSBOM(sbomPath, vexPath)
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

// scanSBOM matches the SBOM against the loaded vulnerability DB, holding the DB
// read lock so a scheduled update cannot close the provider mid-scan.
func (b *Builder) scanSBOM(sbomPath, vexPath string) (*models.Document, *sbom.SBOM, error) {
	b.dbMu.RLock()
	defer b.dbMu.RUnlock()

	sc := b.scanner.Load()
	if sc == nil {
		return nil, nil, ErrNotReady
	}

	now := time.Now()

	return sc.ScanSBOM(sbomPath, &now, vexPath)
}

// Describe implements prom.Collector interface.
func (b *Builder) Describe(ch chan<- *prometheus.Desc) {
	b.c.Describe(ch)
	b.metricDB.Describe(ch)
}

// Collect implements prom.Collector interface.
func (b *Builder) Collect(ch chan<- prometheus.Metric) {
	b.c.Collect(ch)
	b.metricDB.Collect(ch)
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
