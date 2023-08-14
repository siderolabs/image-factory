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

	"github.com/blang/semver/v4"
	"go.uber.org/zap"
)

// Manager supports loading, caching and serving Talos release artifacts.
type Manager struct { //nolint:govet
	options     Options
	storagePath string
	logger      *zap.Logger

	fetcherMu sync.Mutex
	fetchers  map[string]*fetcher
}

// NewManager creates a new artifacts manager.
func NewManager(logger *zap.Logger, options Options) (*Manager, error) {
	tmpDir, err := os.MkdirTemp("", "image-service")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	return &Manager{
		options:     options,
		storagePath: tmpDir,
		logger:      logger,
		fetchers:    map[string]*fetcher{},
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
	errCh := m.fetch(tag) //nolint:contextcheck

	// wait for the fetch to finish
	select {
	case err = <-errCh:
		if err != nil {
			return "", err
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// build the path
	path := filepath.Join(m.storagePath, tag, string(arch), string(kind))

	_, err = os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to find artifact: %w", err)
	}

	return path, nil
}

// GetInstallerImageRef returns the installer image reference for the given version.
func (m *Manager) GetInstallerImageRef(versionString string) string {
	return m.options.ImagePrefix + "installer:v" + versionString
}

// fetch a version of Talos artifacts.
func (m *Manager) fetch(tag string) <-chan error {
	m.fetcherMu.Lock()
	defer m.fetcherMu.Unlock()

	fetcher, ok := m.fetchers[tag]
	if !ok {
		fetcher = newFetcher()
		m.fetchers[tag] = fetcher

		fetcher.Fetch(m.logger, tag, m.options, m.storagePath)

		// cleanup fetcher if it fails
		cleanupCh := fetcher.Subscribe()

		go func() {
			if err := <-cleanupCh; err != nil {
				m.fetcherMu.Lock()
				defer m.fetcherMu.Unlock()

				if m.fetchers[tag] == fetcher {
					delete(m.fetchers, tag)
				}
			}
		}()
	}

	return fetcher.Subscribe()
}
