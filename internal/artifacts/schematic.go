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
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"gopkg.in/yaml.v3"

	"github.com/skyssolutions/siderolabs-image-factory/pkg/constants"
	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

// GetSchematicExtension returns a path to the tarball with "virtual" extension matching a specified schematic.
func (m *Manager) GetSchematicExtension(ctx context.Context, versionTag string, schematic *schematic.Schematic) (string, error) {
	schematicID, err := schematic.ID()
	if err != nil {
		return "", err
	}

	cacheID := fmt.Sprintf("%s-%v", schematicID, quirks.New(versionTag).SupportsOverlay())
	extensionPath := filepath.Join(m.schematicsPath, cacheID+".tar")

	if _, err = os.Stat(extensionPath); err == nil {
		// already built
		return extensionPath, nil
	}

	var schematicInfo []byte

	if quirks.New(versionTag).SupportsOverlay() {
		schematicInfo, err = yaml.Marshal(schematic)
		if err != nil {
			return "", fmt.Errorf("failed to marshal schematic overlay info: %w", err)
		}
	}

	resultCh := m.sf.DoChan(cacheID, func() (any, error) {
		return nil, m.buildSchematicExtension(schematicID, extensionPath, schematicInfo)
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

// schematicExtension builds a "virtual" extension matching a specified schematic.
func schematicExtension(schematicID string, schematicInfo []byte) (io.Reader, error) {
	manifest := extensions.Manifest{
		Version: "v1alpha1",
		Metadata: extensions.Metadata{
			Name:        constants.SchematicIDExtensionName,
			Version:     schematicID,
			Author:      "Image Factory",
			Description: "Virtual extension which specifies the schematic of the image built with Image Factory.",
			Compatibility: extensions.Compatibility{
				Talos: extensions.Constraint{
					Version: ">= 1.0.0",
				},
			},
		},
	}

	if len(schematicInfo) > 0 {
		manifest.Metadata.ExtraInfo = string(schematicInfo)
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
		"rootfs/usr/local/share/schematic/",
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
		Name:     filepath.Join("rootfs/usr/local/share/schematic", schematicID), // empty file
		Typeflag: tar.TypeReg,
		Mode:     0o644,
	}); err != nil {
		return nil, fmt.Errorf("failed to write rootfs header: %w", err)
	}

	if err = tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	return &buf, nil
}

// buildSchematicExtension builds a schematic extension tarball.
func (m *Manager) buildSchematicExtension(schematicID, extensionPath string, schematicInfo []byte) error {
	tarball, err := schematicExtension(schematicID, schematicInfo)
	if err != nil {
		return fmt.Errorf("failed to build schematic layer: %w", err)
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
