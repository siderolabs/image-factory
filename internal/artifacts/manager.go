// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/cache"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// registryWithNamespace couples a registry with an optional repository path prefix
// (e.g. a Harbor proxy-cache project) prepended to every repository pulled from it.
type registryWithNamespace struct {
	registry  name.Registry
	namespace string
}

// Repo returns a repository in the registry with the namespace prefix applied.
func (r registryWithNamespace) Repo(repoPath string) name.Repository {
	return r.registry.Repo(path.Join(r.namespace, repoPath))
}

// Manager supports loading, caching and serving Talos release artifacts.
type Manager struct { //nolint:govet
	options                 Options
	storagePath             string
	schematicsPath          string
	logger                  *zap.Logger
	imageRegistry           registryWithNamespace
	extraExtensionsRegistry registryWithNamespace
	pullers                 map[Arch]remotewrap.Puller

	sf singleflight.Group

	officialExtensions *cache.SingleFlightCache[[]ExtensionRef]
	officialOverlays   *cache.SingleFlightCache[[]OverlayRef]
	talosctlTuples     *cache.SingleFlightCache[[]TalosctlTuple]

	talosVersionsMu        sync.Mutex
	talosVersions          []semver.Version
	talosVersionsTimestamp time.Time
}

// NewManager creates a new artifacts manager.
//
//nolint:gocognit
func NewManager(logger *zap.Logger, options Options) (*Manager, error) {
	tmpDir, err := os.MkdirTemp("", "image-factory")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	schematicsPath := filepath.Join(tmpDir, "schematics")

	if err = os.Mkdir(schematicsPath, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create schematics directory: %w", err)
	}

	opts := []name.Option{}
	if options.InsecureImageRegistry {
		opts = append(opts, name.Insecure)
	}

	registry, err := name.NewRegistry(options.ImageRegistry, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image registry: %w", err)
	}

	imageRegistry := registryWithNamespace{
		registry:  registry,
		namespace: options.ImageRegistryNamespace,
	}

	var extraExtensionsImageRegistry registryWithNamespace

	if options.ExtraExtensionsImageRegistry != "" {
		opts = []name.Option{}
		if options.InsecureExtraExtensionsRegistry {
			opts = append(opts, name.Insecure)
		}

		extraExtensionsImageRegistry.registry, err = name.NewRegistry(options.ExtraExtensionsImageRegistry, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse extra extensions image registry: %w", err)
		}
	}

	pullers := make(map[Arch]remotewrap.Puller, 2)

	for _, arch := range []Arch{ArchAmd64, ArchArm64} {
		pullers[arch], err = remotewrap.NewPuller(
			options.RegistryRefreshInterval,
			append(
				[]remote.Option{
					remote.WithPlatform(v1.Platform{
						Architecture: string(arch),
						OS:           "linux",
					}),
				},
				options.RemoteOptions...,
			)...,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create puller: %w", err)
		}
	}

	m := &Manager{
		options:                 options,
		storagePath:             tmpDir,
		schematicsPath:          schematicsPath,
		logger:                  logger,
		imageRegistry:           imageRegistry,
		extraExtensionsRegistry: extraExtensionsImageRegistry,
		pullers:                 pullers,
	}

	m.officialExtensions = cache.NewSingleFlightCache(func(tag string) ([]ExtensionRef, error) {
		officialExtensions, err := m.fetchExtensionList(m.options.ExtensionManifestImage, tag, m.imageRegistry)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch official extensions: %w", err)
		}

		if m.options.ExtraExtensionManifestImage == "" {
			return officialExtensions, nil
		}

		extraExtensions, err := m.fetchExtensionList(m.options.ExtraExtensionManifestImage, tag, m.extraExtensionsRegistry)
		if err != nil {
			if regtransport.IsStatusCodeError(err, http.StatusNotFound) {
				logger.Sugar().Warnf("extra extensions not published for talos version %s", tag)

				return officialExtensions, nil
			}

			return nil, fmt.Errorf("failed to fetch extra extensions: %w", err)
		}

		allExtensions := slices.Concat(extraExtensions)

		// prioritize extra extensions if there's a name conflict
		for _, official := range officialExtensions {
			extraOverride := false

			for _, extra := range extraExtensions {
				if official.TaggedReference.RepositoryStr() == extra.TaggedReference.RepositoryStr() {
					extraOverride = true

					break
				}
			}

			if extraOverride {
				continue
			}

			allExtensions = append(allExtensions, official)
		}

		return allExtensions, nil
	})

	m.officialOverlays = cache.NewSingleFlightCache(func(tag string) ([]OverlayRef, error) {
		return m.fetchOverlayList(tag)
	})

	m.talosctlTuples = cache.NewSingleFlightCache(func(tag string) ([]TalosctlTuple, error) {
		return m.fetchTalosctlTuples(tag)
	})

	return m, nil
}

// Close the manager.
func (m *Manager) Close() error {
	return os.RemoveAll(m.storagePath)
}

func (m *Manager) validateTalosVersion(ctx context.Context, version semver.Version) error {
	availableVersion, err := m.GetTalosVersions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get available Talos versions: %w", err)
	}

	if !slices.ContainsFunc(availableVersion, version.Equals) {
		return xerrors.NewTaggedf[ErrNotFoundTag]("version %s is not available", version)
	}

	return nil
}

// Get returns the artifact path for the given version, arch and kind.
func (m *Manager) Get(ctx context.Context, versionString string, arch Arch, kind Kind) (string, error) {
	version, err := semver.Parse(versionString)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	if err = m.validateTalosVersion(ctx, version); err != nil {
		return "", err
	}

	tag := "v" + version.String()

	// check if already extracted
	if _, err = os.Stat(filepath.Join(m.storagePath, tag)); err != nil {
		resultCh := m.sf.DoChan(tag, func() (any, error) { //nolint:contextcheck
			return nil, m.fetchImager(tag)
		})

		// wait for the fetch to finish
		select {
		case result := <-resultCh:
			if result.Err != nil {
				return "", result.Err
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// build the path
	path := filepath.Join(m.storagePath, tag, string(arch), string(kind))

	_, err = os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to find artifact: %w", err)
	}

	return path, nil
}

// GetBrokenTalosVersions returns the list of Talos versions marked as broken in the configuration.
func (m *Manager) GetBrokenTalosVersions() []semver.Version {
	return slices.Clone(m.options.BrokenVersions)
}

// GetTalosVersions returns a list of Talos versions available.
func (m *Manager) GetTalosVersions(ctx context.Context) ([]semver.Version, error) {
	m.talosVersionsMu.Lock()
	versions, timestamp := m.talosVersions, m.talosVersionsTimestamp
	m.talosVersionsMu.Unlock()

	if time.Since(timestamp) < m.options.TalosVersionRecheckInterval {
		return versions, nil
	}

	resultCh := m.sf.DoChan("talos-versions", m.fetchTalosVersions)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
	}

	m.talosVersionsMu.Lock()
	versions = m.talosVersions
	m.talosVersionsMu.Unlock()

	return versions, nil
}

// GetOfficialExtensions returns a list of Talos extensions per Talos version available.
func (m *Manager) GetOfficialExtensions(ctx context.Context, versionString string) ([]ExtensionRef, error) {
	tag, err := m.parseTag(ctx, versionString)
	if err != nil {
		return nil, err
	}

	return m.officialExtensions.Get(ctx, tag)
}

// GetOfficialOverlays returns a list of overlays per Talos version available.
func (m *Manager) GetOfficialOverlays(ctx context.Context, versionString string) ([]OverlayRef, error) {
	tag, err := m.parseTag(ctx, versionString)
	if err != nil {
		return nil, err
	}

	return m.officialOverlays.Get(ctx, tag)
}

// GetInstallerImage pulls and stores in OCI layout installer-base image.
func (m *Manager) GetInstallerImage(ctx context.Context, arch Arch, versionString string) (string, error) {
	version, err := semver.ParseTolerant(versionString)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	if err = m.validateTalosVersion(ctx, version); err != nil {
		return "", err
	}

	tag := "v" + version.String()

	ociPath := filepath.Join(m.storagePath, string(arch)+"-installer-"+tag)

	// check if already fetched
	if _, err := os.Stat(ociPath); err != nil {
		resultCh := m.sf.DoChan(ociPath, func() (any, error) { //nolint:contextcheck
			return nil, m.fetchInstallerImage(arch, tag, ociPath)
		})

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case result := <-resultCh:
			if result.Err != nil {
				return "", result.Err
			}
		}
	}

	return ociPath, nil
}

// GetExtensionImage pulls and stores in OCI layout an extension image.
func (m *Manager) GetExtensionImage(ctx context.Context, arch Arch, ref ExtensionRef) (string, error) {
	ociPath := filepath.Join(m.storagePath, string(arch)+"-"+ref.Digest)

	// check if already fetched
	if _, err := os.Stat(ociPath); err != nil {
		resultCh := m.sf.DoChan(ociPath, func() (any, error) { //nolint:contextcheck
			return nil, m.fetchExtensionImage(arch, ref, ociPath)
		})

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case result := <-resultCh:
			if result.Err != nil {
				return "", result.Err
			}
		}
	}

	return ociPath, nil
}

// GetOverlayImage pulls and stores in OCI layout an overlay image.
func (m *Manager) GetOverlayImage(ctx context.Context, arch Arch, ref OverlayRef) (string, error) {
	ociPath := filepath.Join(m.storagePath, string(arch)+"-"+ref.Digest)

	// check if already fetched
	if _, err := os.Stat(ociPath); err != nil {
		resultCh := m.sf.DoChan(ociPath, func() (any, error) { //nolint:contextcheck
			return nil, m.fetchOverlayImage(arch, ref, ociPath)
		})

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case result := <-resultCh:
			if result.Err != nil {
				return "", result.Err
			}
		}
	}

	return ociPath, nil
}

// GetOverlayArtifact returns the artifact path for the given version, arch and kind.
func (m *Manager) GetOverlayArtifact(ctx context.Context, arch Arch, ref OverlayRef, kind OverlayKind) (string, error) {
	extractedPath := filepath.Join(m.storagePath, string(arch)+"-"+ref.Digest+"-overlay")

	// check if already extracted
	if _, err := os.Stat(extractedPath); err != nil {
		resultCh := m.sf.DoChan(extractedPath, func() (any, error) { //nolint:contextcheck
			return nil, m.extractOverlay(arch, ref)
		})

		// wait for the fetch to finish
		select {
		case result := <-resultCh:
			if result.Err != nil {
				return "", result.Err
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// build the path
	path := filepath.Join(extractedPath, string(kind))

	_, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to find overlay artifact: %w", err)
	}

	return path, nil
}

// GetTalosctlImage pulls and stores in OCI layout talosctl-all image.
func (m *Manager) GetTalosctlImage(ctx context.Context, versionString string) (string, error) {
	version, err := semver.ParseTolerant(versionString)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	if err = m.validateTalosVersion(ctx, version); err != nil {
		return "", err
	}

	tag := "v" + version.String()

	ociPath := filepath.Join(m.storagePath, "talosctl-all-"+tag)

	// check if already fetched
	if _, err := os.Stat(ociPath); err != nil {
		resultCh := m.sf.DoChan(ociPath, func() (any, error) { //nolint:contextcheck
			return nil, m.fetchTalosctlImage(tag, ociPath)
		})

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case result := <-resultCh:
			if result.Err != nil {
				var terr *transport.Error
				if errors.As(result.Err, &terr) && terr.StatusCode == http.StatusNotFound {
					return "", xerrors.NewTaggedf[ErrNotFoundTag]("version %s is not available", version)
				}

				return "", result.Err
			}
		}
	}

	return ociPath, nil
}

// GetTalosctlTuples returns a list of Talosctl tuples for the given version.
func (m *Manager) GetTalosctlTuples(ctx context.Context, versionString string) ([]TalosctlTuple, error) {
	tag, err := m.parseTag(ctx, versionString)
	if err != nil {
		return nil, err
	}

	if !quirks.New(versionString).SupportsFactoryTalosctlDownload() {
		return nil, nil
	}

	return m.talosctlTuples.Get(ctx, tag)
}

func (m *Manager) parseTag(ctx context.Context, versionString string) (string, error) {
	version, err := semver.ParseTolerant(versionString)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	if err = m.validateTalosVersion(ctx, version); err != nil {
		return "", err
	}

	return "v" + version.String(), nil
}
