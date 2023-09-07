// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/image-service/pkg/flavor"
)

// GetFlavorExtension returns a path to the tarball with "virtual" extension matching a specified flavor.
func (m *Manager) GetFlavorExtension(ctx context.Context, flavor *flavor.Flavor) (string, error) {
	flavorID, err := flavor.ID()
	if err != nil {
		return "", err
	}

	extensionPath := filepath.Join(m.flavorsPath, flavorID+".tar")

	resultCh := m.flavorsSingleFlight.DoChan(flavorID, func() (any, error) {
		return nil, m.buildFlavorExtension(flavorID, extensionPath)
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return "", result.Err
		}

		return extensionPath, nil
	}
}

// flavorExtension builds a "virtual" extension matching a specified flavor.
func flavorExtension(flavorID string) (io.Reader, error) {
	manifest := extensions.Manifest{
		Version: "v1alpha1",
		Metadata: extensions.Metadata{
			Name:        "flavor",
			Version:     flavorID,
			Author:      "Image Service",
			Description: "Virtual extension which specifies the flavor of the image built with Image Service.",
			Compatibility: extensions.Compatibility{
				Talos: extensions.Constraint{
					Version: ">= 1.0.0",
				},
			},
		},
	}

	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	if err = tw.WriteHeader(&tar.Header{
		Name:     "manifest.yaml",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(manifestBytes)),
	}); err != nil {
		return nil, fmt.Errorf("failed to write manifest header: %w", err)
	}

	if _, err = tw.Write(manifestBytes); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	for _, path := range []string{
		"rootfs/",
		"rootfs/usr/",
		"rootfs/usr/local/",
		"rootfs/usr/local/share/",
		"rootfs/usr/local/share/flavor/",
	} {
		if err = tw.WriteHeader(&tar.Header{
			Name:     path,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		}); err != nil {
			return nil, fmt.Errorf("failed to write rootfs header: %w", err)
		}
	}

	if err = tw.WriteHeader(&tar.Header{
		Name:     filepath.Join("rootfs/usr/local/share/flavor", flavorID), // empty file
		Typeflag: tar.TypeReg,
		Mode:     0o755,
	}); err != nil {
		return nil, fmt.Errorf("failed to write rootfs header: %w", err)
	}

	if err = tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	return &buf, nil
}

// buildFlavorExtension builds a flavor extension tarball.
func (m *Manager) buildFlavorExtension(flavorID, extensionPath string) error {
	tarball, err := flavorExtension(flavorID)
	if err != nil {
		return fmt.Errorf("failed to build flavor layer: %w", err)
	}

	f, err := os.Create(extensionPath + ".tmp")
	if err != nil {
		return fmt.Errorf("failed to create extension tarball: %w", err)
	}

	defer f.Close() //nolint:errcheck

	_, err = io.Copy(f, tarball)
	if err != nil {
		return fmt.Errorf("failed to write extension tarball: %w", err)
	}

	if err = os.Rename(extensionPath+".tmp", extensionPath); err != nil {
		return fmt.Errorf("failed to rename extension tarball: %w", err)
	}

	return f.Close()
}
