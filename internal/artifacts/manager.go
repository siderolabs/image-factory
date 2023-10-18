// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Manager supports loading, caching and serving Talos release artifacts.
type Manager struct { //nolint:govet
	options        Options
	storagePath    string
	schematicsPath string
	logger         *zap.Logger
	imageRegistry  name.Registry
	pullers        map[Arch]*remote.Puller

	sf singleflight.Group

	officialExtensionsMu sync.Mutex
	officialExtensions   map[string][]ExtensionRef

	talosVersionsMu        sync.Mutex
	talosVersions          []semver.Version
	talosVersionsTimestamp time.Time
}

// NewManager creates a new artifacts manager.
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

	imageRegistry, err := name.NewRegistry(options.ImageRegistry, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image registry: %w", err)
	}

	pullers := make(map[Arch]*remote.Puller, 2)

	for _, arch := range []Arch{ArchAmd64, ArchArm64} {
		pullers[arch], err = remote.NewPuller(
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

	return &Manager{
		options:        options,
		storagePath:    tmpDir,
		schematicsPath: schematicsPath,
		logger:         logger,
		imageRegistry:  imageRegistry,
		pullers:        pullers,
	}, nil
}

// Close the manager.
func (m *Manager) Close() error {
	return os.RemoveAll(m.storagePath)
}

// Get returns the artifact path for the given version, arch and kind.
func (m *Manager) Get(ctx context.Context, versionString string, arch Arch, kind Kind) (string, error) {
	version, err := semver.Parse(versionString)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	if version.LT(m.options.MinVersion) {
		return "", fmt.Errorf("version %s is not supported, minimum is %s", version, m.options.MinVersion)
	}

	tag := "v" + version.String()

	// check if already extracted
	if _, err = os.Stat(filepath.Join(m.storagePath, tag)); err != nil {
		resultCh := m.sf.DoChan(tag, func() (any, error) {
			return nil, m.fetchImager(tag)
		})

		// wait for the fetch to finish
		select {
		case result := <-resultCh:
			if result.Err != nil {
				return "", err
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
	version, err := semver.ParseTolerant(versionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %w", err)
	}

	if version.LT(m.options.MinVersion) {
		return nil, fmt.Errorf("version %s is not supported, minimum is %s", version, m.options.MinVersion)
	}

	tag := "v" + version.String()

	m.officialExtensionsMu.Lock()
	extensions, ok := m.officialExtensions[tag]
	m.officialExtensionsMu.Unlock()

	if ok {
		return extensions, nil
	}

	resultCh := m.sf.DoChan("extensions-"+tag, func() (any, error) {
		return nil, m.fetchOfficialExtensions(tag)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
	}

	m.officialExtensionsMu.Lock()
	extensions = m.officialExtensions[tag]
	m.officialExtensionsMu.Unlock()

	return extensions, nil
}

// GetInstallerImage pulls and stoers in OCI layout installer image.
func (m *Manager) GetInstallerImage(ctx context.Context, arch Arch, versionString string) (string, error) {
	version, err := semver.ParseTolerant(versionString)
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	if version.LT(m.options.MinVersion) {
		return "", fmt.Errorf("version %s is not supported, minimum is %s", version, m.options.MinVersion)
	}

	tag := "v" + version.String()

	ociPath := filepath.Join(m.storagePath, string(arch)+"-installer-"+tag)

	// check if already fetched
	if _, err := os.Stat(ociPath); err != nil {
		resultCh := m.sf.DoChan(ociPath, func() (any, error) {
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
		resultCh := m.sf.DoChan(ociPath, func() (any, error) {
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
