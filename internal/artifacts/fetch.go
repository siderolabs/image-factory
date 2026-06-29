// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/artifacts/imagehandler"
	"github.com/siderolabs/image-factory/internal/image/verify"
)

// taggedRef builds a tagged reference for "<image>:<tag>". A bare repository path
// (e.g. "siderolabs/extensions") defaults to the factory's core registry, while a
// reference carrying its own registry (e.g. the extra-extensions manifest) is
// honored as-is.
func (m *Manager) taggedRef(image, tag string, insecure bool) (TaggedReference, error) {
	nameOptions := []name.Option{name.WithDefaultRegistry(m.options.ImageRegistry)}
	if insecure {
		nameOptions = append(nameOptions, name.Insecure)
	}

	ref, err := name.NewTag(image+":"+tag, nameOptions...)
	if err != nil {
		return TaggedReference{}, fmt.Errorf("failed to parse image reference %q: %w", image+":"+tag, err)
	}

	return TaggedReference{Ref: ref, Insecure: insecure}, nil
}

// fetchImageByTag contains combined logic of image handling: heading, downloading, verifying signatures, and exporting.
func (m *Manager) fetchImageByTag(ref TaggedReference, architecture Arch, imageHandler imagehandler.Handler) error {
	// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	// light check first - if the image exists, and resolve the digest
	// it's important to do further checks by digest exactly
	repoRef := ref.Ref

	m.logger.Debug("heading the image", zap.Stringer("image", repoRef))

	descriptor, err := m.pullers[architecture].Head(ctx, repoRef)
	if err != nil {
		return err
	}

	digestRef := repoRef.Digest(descriptor.Digest.String())

	return m.fetchImageByDigest(digestRef, ref.Insecure, architecture, imageHandler)
}

// fetchImageByDigest fetches an image by digest, verifies signatures, and exports it to the storage.
func (m *Manager) fetchImageByDigest(digestRef name.Digest, insecure bool, architecture Arch, imageHandler imagehandler.Handler) error {
	var err error
	// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	logger := m.logger.With(zap.Stringer("image", digestRef))

	// verify the image signature, we only accept properly signed images
	logger.Debug("verifying image signature")

	var nameOptions []name.Option

	if insecure {
		nameOptions = append(nameOptions, name.Insecure)
	}

	verifyResult, err := verify.VerifySignatures(ctx, digestRef, m.options.ImageVerifyOptions, nameOptions...)
	if err != nil {
		return fmt.Errorf("failed to verify image signature for %s: %w", digestRef.Name(), err)
	}

	logger.Info("image signature verified", zap.String("verification_method", verifyResult.Method), zap.Bool("bundle_verified", verifyResult.Verified))

	// pull down the image and extract the necessary parts
	logger.Info("pulling the image")

	desc, err := m.pullers[architecture].Get(ctx, digestRef)
	if err != nil {
		return fmt.Errorf("error pulling image %s: %w", digestRef, err)
	}

	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("error creating image from descriptor: %w", err)
	}

	return imageHandler(ctx, logger, img)
}

// fetchImager fetches 'imager' container, and saves to the storage path.
func (m *Manager) fetchImager(tag string) error {
	destinationPath := filepath.Join(m.storagePath, tag)

	ref, err := m.taggedRef(m.options.ImagerImage, tag, m.options.InsecureImageRegistry)
	if err != nil {
		return err
	}

	if err := m.fetchImageByTag(ref, ArchAmd64, imagehandler.Export(func(logger *zap.Logger, r io.Reader) error {
		return untarWithPrefix(logger, r, usrInstallPrefix, destinationPath+tmpSuffix)
	})); err != nil {
		return err
	}

	return os.Rename(destinationPath+tmpSuffix, destinationPath)
}

// extractOverlay fetches 'overlay' container, and saves to the storage path.
func (m *Manager) extractOverlay(arch Arch, ref OverlayRef) error {
	imageRef := ref.TaggedReference.Ref.Context().Digest(ref.Digest)

	destinationPath := filepath.Join(m.storagePath, string(arch)+"-"+ref.Digest+"-overlay")

	if err := m.fetchImageByDigest(imageRef, ref.TaggedReference.Insecure, arch, imagehandler.Export(func(logger *zap.Logger, r io.Reader) error {
		return untarWithPrefix(logger, r, overlaysPrefix, destinationPath+tmpSuffix)
	})); err != nil {
		return err
	}

	return os.Rename(destinationPath+tmpSuffix, destinationPath)
}

// fetchExtensionImage fetches a specified extension image and exports it to the storage as OCI.
func (m *Manager) fetchExtensionImage(arch Arch, ref ExtensionRef, destPath string) error {
	imageRef := ref.TaggedReference.Ref.Context().Digest(ref.Digest)

	if err := m.fetchImageByDigest(imageRef, ref.TaggedReference.Insecure, arch, imagehandler.OCI(destPath+tmpSuffix)); err != nil {
		return err
	}

	return os.Rename(destPath+tmpSuffix, destPath)
}

// fetchOverlayImage fetches a specified overlay image and exports it to the storage as OCI.
func (m *Manager) fetchOverlayImage(arch Arch, ref OverlayRef, destPath string) error {
	imageRef := ref.TaggedReference.Ref.Context().Digest(ref.Digest)

	if err := m.fetchImageByDigest(imageRef, ref.TaggedReference.Insecure, arch, imagehandler.OCI(destPath+tmpSuffix)); err != nil {
		return err
	}

	return os.Rename(destPath+tmpSuffix, destPath)
}

// InstallerImageName returns an installer image name based on Talos version.
func (m *Manager) InstallerImageName(versionTag string) string {
	if quirks.New(versionTag).SupportsUnifiedInstaller() {
		return m.options.InstallerBaseImage
	}

	return m.options.InstallerImage
}

// fetchInstallerImage fetches a Talos installer image and exports it to the storage.
func (m *Manager) fetchInstallerImage(arch Arch, versionTag string, destPath string) error {
	ref, err := m.taggedRef(m.InstallerImageName(versionTag), versionTag, m.options.InsecureImageRegistry)
	if err != nil {
		return err
	}

	if err := m.fetchImageByTag(ref, arch, imagehandler.OCI(destPath+tmpSuffix)); err != nil {
		return err
	}

	return os.Rename(destPath+tmpSuffix, destPath)
}

// fetchTalosctlImage fetches a Talosctl image and exports it to the storage.
func (m *Manager) fetchTalosctlImage(versionTag string, destPath string) error {
	ref, err := m.taggedRef(m.options.TalosctlImage, versionTag, m.options.InsecureImageRegistry)
	if err != nil {
		return err
	}

	if err := m.fetchImageByTag(ref, ArchAmd64, imagehandler.Export(func(logger *zap.Logger, r io.Reader) error {
		return untarWithPrefix(logger, r, "", destPath+tmpSuffix)
	})); err != nil {
		return err
	}

	return os.Rename(destPath+tmpSuffix, destPath)
}

const (
	usrInstallPrefix = "usr/install/"
	overlaysPrefix   = ""
)

func untarWithPrefix(logger *zap.Logger, r io.Reader, prefix, destination string) error {
	tr := tar.NewReader(r)

	size := int64(0)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg || !strings.HasPrefix(hdr.Name, prefix) { // skip
			_, err = io.Copy(io.Discard, tr)
			if err != nil {
				return fmt.Errorf("error skipping data: %w", err)
			}

			continue
		}

		destPath := filepath.Join(destination, hdr.Name[len(prefix):])

		if err = os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("error creating directory %q: %w", filepath.Dir(destPath), err)
		}

		f, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("error creating file %q: %w", destPath, err)
		}

		_, err = io.Copy(f, tr)
		if err != nil {
			return fmt.Errorf("error copying data to %q: %w", destPath, err)
		}

		if err = f.Close(); err != nil {
			return fmt.Errorf("error closing %q: %w", destPath, err)
		}

		size += hdr.Size
	}

	logger.Info("extracted the image", zap.Int64("size", size), zap.String("destination", destination))

	return nil
}
