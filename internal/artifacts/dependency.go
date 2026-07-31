// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

// ImageDependency identifies the exact platform manifest consumed from an OCI image.
type ImageDependency struct {
	OCIPath    string
	Name       string
	Ref        name.Digest
	Descriptor v1.Descriptor
}

// GetInstallerDependency resolves the exact base Installer platform manifest used by a build.
func (m *Manager) GetInstallerDependency(ctx context.Context, arch Arch, versionString string) (ImageDependency, error) {
	path, err := m.GetInstallerImage(ctx, arch, versionString)
	if err != nil {
		return ImageDependency{}, err
	}

	descriptor, err := DescriptorFromOCIPath(path, arch)
	if err != nil {
		return ImageDependency{}, fmt.Errorf("failed to inspect base Installer OCI layout: %w", err)
	}

	version, err := semver.ParseTolerant(versionString)
	if err != nil {
		return ImageDependency{}, fmt.Errorf("failed to parse Talos version: %w", err)
	}

	imageName := m.InstallerImageName("v" + version.String())
	repository := m.imageRegistry.Repo(imageName)

	return ImageDependency{
		OCIPath:    path,
		Name:       imageName,
		Ref:        repository.Digest(descriptor.Digest.String()),
		Descriptor: descriptor,
	}, nil
}

// GetExtensionDependency resolves the exact extension platform manifest used by a build.
func (m *Manager) GetExtensionDependency(ctx context.Context, arch Arch, ref ExtensionRef) (ImageDependency, error) {
	path, err := m.GetExtensionImage(ctx, arch, ref)
	if err != nil {
		return ImageDependency{}, err
	}

	descriptor, err := DescriptorFromOCIPath(path, arch)
	if err != nil {
		return ImageDependency{}, fmt.Errorf("failed to inspect extension OCI layout: %w", err)
	}

	return ImageDependency{
		OCIPath:    path,
		Name:       ref.TaggedReference.RepositoryStr(),
		Ref:        ref.pullReference.Context().Digest(descriptor.Digest.String()),
		Descriptor: descriptor,
	}, nil
}

// GetOverlayDependency resolves the exact overlay platform manifest used by a build.
func (m *Manager) GetOverlayDependency(ctx context.Context, arch Arch, ref OverlayRef) (ImageDependency, error) {
	path, err := m.GetOverlayImage(ctx, arch, ref)
	if err != nil {
		return ImageDependency{}, err
	}

	descriptor, err := DescriptorFromOCIPath(path, arch)
	if err != nil {
		return ImageDependency{}, fmt.Errorf("failed to inspect overlay OCI layout: %w", err)
	}

	repository := m.imageRegistry.Repo(ref.TaggedReference.RepositoryStr())

	return ImageDependency{
		OCIPath:    path,
		Name:       ref.Name,
		Ref:        repository.Digest(descriptor.Digest.String()),
		Descriptor: descriptor,
	}, nil
}

// DescriptorFromOCIPath resolves the manifest descriptor for arch from an OCI image layout.
func DescriptorFromOCIPath(path string, arch Arch) (v1.Descriptor, error) {
	ociPath, err := layout.FromPath(path)
	if err != nil {
		return v1.Descriptor{}, fmt.Errorf("failed to open OCI layout: %w", err)
	}

	index, err := ociPath.ImageIndex()
	if err != nil {
		return v1.Descriptor{}, fmt.Errorf("failed to read OCI index: %w", err)
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return v1.Descriptor{}, fmt.Errorf("failed to read OCI index manifest: %w", err)
	}

	for _, descriptor := range manifest.Manifests {
		if descriptor.Platform != nil && descriptor.Platform.OS == "linux" && descriptor.Platform.Architecture == string(arch) {
			return descriptor, nil
		}
	}

	return v1.Descriptor{}, fmt.Errorf("OCI layout has no linux/%s manifest", arch)
}
