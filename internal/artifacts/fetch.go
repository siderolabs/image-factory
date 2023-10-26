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

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type imageHandler func(ctx context.Context, logger *zap.Logger, img v1.Image) error

// imageExportHandler exports the image for further processing.
func imageExportHandler(exportHandler func(logger *zap.Logger, r io.Reader) error) imageHandler {
	return func(ctx context.Context, logger *zap.Logger, img v1.Image) error {
		logger.Info("extracting the image")

		r, w := io.Pipe()

		var eg errgroup.Group

		eg.Go(func() error {
			defer w.Close() //nolint:errcheck

			return crane.Export(img, w)
		})

		eg.Go(func() error {
			err := exportHandler(logger, r)
			if err != nil {
				r.CloseWithError(err) // signal the exporter to stop
			}

			return err
		})

		if err := eg.Wait(); err != nil {
			return fmt.Errorf("error extracting the image: %w", err)
		}

		return nil
	}
}

// imageOCIHandler exports the image to the OCI format.
func imageOCIHandler(path string) imageHandler {
	return func(ctx context.Context, logger *zap.Logger, img v1.Image) error {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("error removing the directory %q: %w", path, err)
		}

		l, err := layout.Write(path, empty.Index)
		if err != nil {
			return fmt.Errorf("error creating layout: %w", err)
		}

		logger.Info("exporting the image", zap.String("destination", path))

		if err = l.AppendImage(img); err != nil {
			return fmt.Errorf("error exporting the image: %w", err)
		}

		return nil
	}
}

// fetchImageByTag contains combined logic of image handling: heading, downloading, verifying signatures, and exporting.
func (m *Manager) fetchImageByTag(imageName, tag string, architecture Arch, imageHandler imageHandler) error {
	// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	// light check first - if the image exists, and resolve the digest
	// it's important to do further checks by digest exactly
	repoRef := m.imageRegistry.Repo(imageName).Tag(tag)

	m.logger.Debug("heading the image", zap.Stringer("image", repoRef))

	descriptor, err := m.pullers[architecture].Head(ctx, repoRef)
	if err != nil {
		return err
	}

	digestRef := repoRef.Digest(descriptor.Digest.String())

	return m.fetchImageByDigest(digestRef, architecture, imageHandler)
}

// fetchImageByDigest fetches an image by digest, verifies signatures, and exports it to the storage.
func (m *Manager) fetchImageByDigest(digestRef name.Digest, architecture Arch, imageHandler imageHandler) error {
	// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	logger := m.logger.With(zap.Stringer("image", digestRef))

	// verify the image signature, we only accept properly signed images
	logger.Debug("verifying image signature")

	_, bundleVerified, err := cosign.VerifyImageSignatures(ctx, digestRef, &m.options.ImageVerifyOptions)
	if err != nil {
		return fmt.Errorf("failed to verify image signature: %w", err)
	}

	logger.Info("image signature verified", zap.Bool("bundle_verified", bundleVerified))

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

	if err := m.fetchImageByTag(ImagerImage, tag, ArchAmd64, imageExportHandler(func(logger *zap.Logger, r io.Reader) error {
		return untar(logger, r, destinationPath+tmpSuffix)
	})); err != nil {
		return err
	}

	return os.Rename(destinationPath+tmpSuffix, destinationPath)
}

// fetchExtensionImage fetches a specified extension image and exports it to the storage as OCI.
func (m *Manager) fetchExtensionImage(arch Arch, ref ExtensionRef, destPath string) error {
	imageRef := m.imageRegistry.Repo(ref.TaggedReference.RepositoryStr()).Digest(ref.Digest)

	if err := m.fetchImageByDigest(imageRef, arch, imageOCIHandler(destPath+tmpSuffix)); err != nil {
		return err
	}

	return os.Rename(destPath+tmpSuffix, destPath)
}

// fetchInstallerImage fetches a Talos installer image and exports it to the storage.
func (m *Manager) fetchInstallerImage(arch Arch, versionTag string, destPath string) error {
	if err := m.fetchImageByTag(InstallerImage, versionTag, arch, imageOCIHandler(destPath+tmpSuffix)); err != nil {
		return err
	}

	return os.Rename(destPath+tmpSuffix, destPath)
}

func untar(logger *zap.Logger, r io.Reader, destination string) error {
	const usrInstallPrefix = "usr/install/"

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

		if hdr.Typeflag != tar.TypeReg || !strings.HasPrefix(hdr.Name, usrInstallPrefix) { // skip
			_, err = io.Copy(io.Discard, tr)
			if err != nil {
				return fmt.Errorf("error skipping data: %w", err)
			}

			continue
		}

		destPath := filepath.Join(destination, hdr.Name[len(usrInstallPrefix):])

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
