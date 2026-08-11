// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anchore/clio"
	"github.com/anchore/grype/grype"
	v6 "github.com/anchore/grype/grype/db/v6"
	v6build "github.com/anchore/grype/grype/db/v6/build"
	"github.com/anchore/grype/grype/db/v6/distribution"
	"github.com/anchore/grype/grype/db/v6/installation"
	"github.com/anchore/grype/grype/vulnerability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

// A synthetic Grype vulnerability database, served locally, plus the two tests
// built on it: that the mirror is something Grype accepts at all, and that the
// scanner rotates onto a newly published database on schedule.

// grypeDBMirror serves a synthetic Grype vulnerability database over HTTP, in
// the layout the Grype curator expects: latest.json under /v6, naming an archive
// alongside it.
//
// Serving our own database keeps the rotation test hermetic and quick — no
// several-hundred-megabyte download from Anchore — and, more importantly, lets
// the test control what "the database changed" means, which is impossible
// against upstream because two consecutive checks return the same build.
type grypeDBMirror struct {
	*httptest.Server

	// lastBuilt is the build timestamp of the currently published database.
	// SetDBMetadata rounds to the second and the curator only installs a strictly
	// newer candidate, so two publishes within the same second are indistinguishable.
	lastBuilt time.Time

	dir string
}

// newGrypeDBMirror starts a mirror serving a database whose sole provider is
// named providerID.
//
// The provider name is the assertable payload: it travels into the scan report
// as descriptor.db.providers, so a report proves which database produced it
// without needing a synthetic vulnerability to match a real package.
func newGrypeDBMirror(t *testing.T, providerID string) *grypeDBMirror {
	t.Helper()

	dir := t.TempDir()

	m := &grypeDBMirror{dir: dir}

	mux := http.NewServeMux()
	mux.Handle("/v6/", http.StripPrefix("/v6/", http.FileServer(http.Dir(dir))))

	m.Server = httptest.NewServer(mux)
	t.Cleanup(m.Close)

	m.publish(t, providerID)

	return m
}

// publish builds a database and makes it the one latest.json points at, which is
// what a scheduled refresh will pick up.
//
// Returns the build timestamp the curator will report for it.
func (m *grypeDBMirror) publish(t *testing.T, providerID string) time.Time {
	t.Helper()

	staging := t.TempDir()

	// keep every publish in its own second so the curator sees a newer build.
	for !time.Now().UTC().Truncate(time.Second).After(m.lastBuilt) {
		time.Sleep(100 * time.Millisecond)
	}

	writer, err := v6.NewWriter(v6.Config{DBDirPath: staging})
	require.NoError(t, err)

	captured := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, writer.AddProvider(v6.Provider{
		ID:           providerID,
		Version:      "1",
		Processor:    "image-factory-integration",
		DateCaptured: &captured,
		InputDigest:  "sha256:" + providerID,
	}))

	// SetDBMetadata stamps the build timestamp from the wall clock, rounded to the
	// second, and the curator only installs a candidate that is strictly newer
	// than what it already has.
	require.NoError(t, writer.SetDBMetadata())
	require.NoError(t, writer.Close())

	require.NoError(t, v6build.CreateArchive(staging, "", nil))

	description, err := v6.ReadDescription(filepath.Join(staging, v6.VulnerabilityDBFileName))
	require.NoError(t, err)
	require.NotNil(t, description)

	// CreateArchive writes the archive and a matching latest.json (checksum and
	// description included) next to the database, so publishing is a copy of
	// everything except the database itself.
	entries, err := os.ReadDir(staging)
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == v6.VulnerabilityDBFileName {
			continue
		}

		content, err := os.ReadFile(filepath.Join(staging, entry.Name()))
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(m.dir, entry.Name()), content, 0o644))
	}

	require.FileExists(t, filepath.Join(m.dir, distribution.LatestFileName))

	m.lastBuilt = description.Built.UTC()

	return m.lastBuilt
}

// databaseURL is the value to configure as the scanner's database URL: the
// curator appends /v6/latest.json to it.
func (m *grypeDBMirror) databaseURL() string {
	return m.URL
}

// loadFromMirror installs and opens the database the mirror currently publishes,
// the same way the scanner does.
func loadFromMirror(t *testing.T, mirror *grypeDBMirror, dbRootDir string) (vulnerability.Provider, *vulnerability.ProviderStatus) {
	t.Helper()

	distConfig := distribution.DefaultConfig()
	distConfig.LatestURL = mirror.databaseURL()

	// surface update failures instead of silently falling back to the installed
	// database, which is what makes a broken mirror look like a working one.
	distConfig.RequireUpdateCheck = true

	instConfig := installation.DefaultConfig(clio.Identification{Name: "image-factory-mirror-test"})
	instConfig.DBRootDir = dbRootDir
	instConfig.UpdateCheckMaxFrequency = 0

	provider, status, err := grype.LoadVulnerabilityDB(distConfig, instConfig, true)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.NoError(t, status.Error)

	// Close is idempotent enough for a test: callers that close early (to reopen the
	// same directory) leave this as a no-op guard.
	t.Cleanup(func() {
		_ = provider.Close() //nolint:errcheck
	})

	return provider, status
}

// providerIDsOf reads the data provenance the report exposes as
// descriptor.db.providers.
func providerIDsOf(t *testing.T, provider vulnerability.Provider) []string {
	t.Helper()

	metadata, ok := provider.(vulnerability.StoreMetadataProvider)
	require.True(t, ok, "provider %T does not expose data provenance", provider)

	provenance, err := metadata.DataProvenance()
	require.NoError(t, err)

	ids := make([]string, 0, len(provenance))

	for id := range provenance {
		ids = append(ids, id)
	}

	return ids
}

// TestGrypeDBMirror checks the synthetic database mirror against Grype's own
// curator: that what it publishes installs at all, and that republishing is
// picked up as an update.
//
// This is the part of TestIntegrationScannerDBRotation that can fail for reasons
// having nothing to do with Image Factory — the archive layout, the latest.json
// contract and the checksum all belong to Grype, and a dependency bump can move
// them. Asserting them here means the rotation test's failure is about rotation.
func testGrypeDBMirror(t *testing.T) {
	mirror := newGrypeDBMirror(t, "mirror-first")
	dbRootDir := t.TempDir()

	provider, status := loadFromMirror(t, mirror, dbRootDir)

	assert.Equal(t, []string{"mirror-first"}, providerIDsOf(t, provider))
	require.False(t, status.Built.IsZero())
	assert.NotEmpty(t, status.SchemaVersion)

	first := status.Built

	// the curator installs a candidate only when it is newer than what it has, so
	// this also covers the mirror keeping its builds monotonic.
	published := mirror.publish(t, "mirror-second")
	require.True(t, published.After(first), "published=%s first=%s", published, first)

	// Close before reopening. Grype serves a reader opened alongside a live one the
	// pre-update contents, which is why Builder.refresh closes the loaded DB first;
	// reopening without closing would read "mirror-first" back even though the
	// curator logs an update.
	require.NoError(t, provider.Close())

	rotated, rotatedStatus := loadFromMirror(t, mirror, dbRootDir)

	assert.Equal(t, []string{"mirror-second"}, providerIDsOf(t, rotated))
	assert.True(t, rotatedStatus.Built.After(first),
		"expected the reloaded database to be newer, first=%s rotated=%s", first, rotatedStatus.Built)
}

// testScannerDB runs the vulnerability database checks. It is reached from
// commonTest: the integration jobs each select a single top-level test by name
// (RUN_TESTS_* in the Makefile), so a test that is not a subtest of one of those
// is compiled and never run.
func testScannerDB(t *testing.T, options cmd.Options) {
	t.Run("mirror", func(t *testing.T) {
		t.Parallel()

		testGrypeDBMirror(t)
	})

	t.Run("rotation", func(t *testing.T) {
		t.Parallel()

		testScannerDBRotation(t, options)
	})
}

// The scheduled-rotation test proper.

const (
	// rotationScheduleLead is how far ahead the daily database update is
	// scheduled. Everything the test must do before the update fires — warm the
	// SBOM, take the "before" report, publish the second database — has to fit in
	// it, since a missed window means the next occurrence is a day later.
	rotationScheduleLead = 3 * time.Minute

	// rotationRefreshTimeout is how long after the scheduled minute to keep polling
	// for the reload and the re-scan that follows it.
	rotationRefreshTimeout = 2 * time.Minute

	rotationProviderBefore = "rotation-before"
	rotationProviderAfter  = "rotation-after"
)

// scanReportDB is the descriptor.db of a scan report: which vulnerability
// database produced it.
type scanReportDB struct {
	Providers map[string]struct {
		Captured time.Time `json:"captured"`
	} `json:"providers"`
	Status struct {
		Built         time.Time `json:"built"`
		SchemaVersion string    `json:"schemaVersion"`
	} `json:"status"`
}

// reportDB fetches a scan report and returns the database it names.
func reportDB(ctx context.Context, t *testing.T, baseURL, schematicID string) scanReportDB {
	t.Helper()

	resp := downloadScan(ctx, t, baseURL, schematicID, scanTestTalosVersion, scanTestArch, "report.json", http.MethodGet)
	defer resp.Body.Close() //nolint:errcheck // also closed by downloadScan's cleanup

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var report struct {
		Descriptor struct {
			DB scanReportDB `json:"db"`
		} `json:"descriptor"`
	}

	require.NoError(t, json.Unmarshal(body, &report))

	return report.Descriptor.DB
}

// providerIDs returns the provider names the report attributes its data to.
func (db scanReportDB) providerIDs() []string {
	ids := make([]string, 0, len(db.Providers))

	for id := range db.Providers {
		ids = append(ids, id)
	}

	return ids
}

// TestIntegrationScannerDBRotation asserts the scheduled vulnerability database
// update replaces the loaded database, and that reports served afterwards come
// from the new one.
//
// Two Image Factory replicas that started on different days each pinned whatever
// Grype called latest at their own boot, so the same report could differ between
// them (issue #515). The fix refreshes the database at a fixed time of day; this
// test drives that schedule.
//
// The database is served from a local mirror rather than Anchore: two consecutive
// upstream checks return the same build, so a rotation would be indistinguishable
// from no rotation, and a real download would cost hundreds of megabytes.
func testScannerDBRotation(t *testing.T, options cmd.Options) {
	if !enterprise.Enabled() {
		t.Skip("scanner is an enterprise feature")
	}

	mirror := newGrypeDBMirror(t, rotationProviderBefore)

	// the update fires at a minute boundary, so the lead is the truncation plus
	// whatever is left of the current minute.
	updateAt := time.Now().Add(rotationScheduleLead).Truncate(time.Minute)

	// This is a second factory in the same process, and RunFactory registers its
	// collectors on the default Prometheus registry. Every collector name is
	// prefixed with this namespace, so a distinct one keeps the registration from
	// colliding with the factory commonTest is already running.
	options.Metrics.Namespace = "rotation"

	options.Enterprise.Scanner.DatabaseURL = mirror.databaseURL()
	options.Enterprise.Scanner.DatabaseUpdateAt = updateAt.Format("15:04")

	// keep the synthetic database out of the shared installation directory, which
	// holds the real one the other enterprise tests scan against.
	options.Enterprise.Scanner.DatabaseRootDir = t.TempDir()

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	c, err := client.New(baseURL, clientAuthCredentials()...)
	require.NoError(t, err)

	schematicID := createSchematicGetID(ctx, t, c, *testSchematics[emptySchematicID])

	// Build the SBOM before the clock matters: on a cold cache this pulls and
	// unpacks an initramfs, which can outlast the lead. Once cached, the scans
	// below only re-run the Grype match.
	resp := downloadSPDX(ctx, t, baseURL, schematicID, scanTestTalosVersion, scanTestArch, http.MethodGet)
	defer resp.Body.Close() //nolint:errcheck // also closed by downloadSPDX's cleanup

	require.Equal(t, http.StatusOK, resp.StatusCode)

	before := reportDB(ctx, t, baseURL, schematicID)
	assert.Equal(t, []string{rotationProviderBefore}, before.providerIDs())
	require.False(t, before.Status.Built.IsZero())

	// Publish the second database only now: until this point a refresh could only
	// reinstall the first one, so the report above cannot have raced it.
	built := mirror.publish(t, rotationProviderAfter)
	require.True(t, built.After(before.Status.Built),
		"the published database must be newer than the loaded one, or the curator will not install it")

	require.True(t, time.Now().Before(updateAt),
		"the scheduled update at %s already passed before the second database was published; "+
			"the next one is a day away, so raise rotationScheduleLead", updateAt.Format(time.RFC3339))

	// Wait out the schedule, then the reload and re-scan.
	//
	// Polling here rather than in require.Eventually: its condition runs on another
	// goroutine, where the require calls inside reportDB could not fail the test
	// properly.
	deadline := time.Now().Add(time.Until(updateAt) + rotationRefreshTimeout)

	var after scanReportDB

	for {
		after = reportDB(ctx, t, baseURL, schematicID)

		if _, rotated := after.Providers[rotationProviderAfter]; rotated {
			break
		}

		require.False(t, time.Now().After(deadline),
			"report still names %v after the update scheduled at %s",
			after.providerIDs(), updateAt.Format(time.RFC3339))

		time.Sleep(5 * time.Second)
	}

	assert.Equal(t, []string{rotationProviderAfter}, after.providerIDs())
	assert.True(t, after.Status.Built.After(before.Status.Built),
		"report should name the newer database build, before=%s after=%s", before.Status.Built, after.Status.Built)
}
