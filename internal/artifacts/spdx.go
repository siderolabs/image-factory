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
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// SPDXFileSuffix is the file suffix for SPDX files.
	SPDXFileSuffix = ".spdx.json"
)

// SPDXFile represents an extracted SPDX file.
type SPDXFile struct {
	// Filename is the original filename (e.g., "extension-name.spdx.json").
	Filename string

	// Source is the source identifier (extension name or "talos").
	Source string

	// Content is the raw JSON content.
	Content []byte
}

// ExtractExtensionSPDX extracts SPDX files from an extension image.
func (m *Manager) ExtractExtensionSPDX(ctx context.Context, arch Arch, ref ExtensionRef) ([]SPDXFile, error) {
	imageRef := m.imageRegistry.Repo(ref.TaggedReference.RepositoryStr()).Digest(ref.Digest)

	var files []SPDXFile

	handler := spdxExportHandler(&files, ref.TaggedReference.RepositoryStr())

	if err := m.fetchImageByDigest(imageRef, arch, handler); err != nil { //nolint:contextcheck
		return nil, fmt.Errorf("failed to extract SPDX from extension %s: %w", ref.TaggedReference.RepositoryStr(), err)
	}

	return files, nil
}

// spdxExportHandler creates an image handler that extracts SPDX files.
func spdxExportHandler(files *[]SPDXFile, source string) imageHandler {
	return func(_ context.Context, logger *zap.Logger, img v1.Image) error {
		logger.Info("extracting SPDX files from image")

		r, w := io.Pipe()

		var eg errgroup.Group

		eg.Go(func() error {
			defer w.Close() //nolint:errcheck

			return crane.Export(img, w)
		})

		eg.Go(func() error {
			extracted, err := extractSPDXFromTar(r, source)
			if err != nil {
				r.CloseWithError(err)

				return err
			}

			*files = extracted

			return nil
		})

		if err := eg.Wait(); err != nil {
			return fmt.Errorf("error extracting SPDX files: %w", err)
		}

		logger.Info("extracted SPDX files", zap.Int("count", len(*files)))

		return nil
	}
}

// extractSPDXFromTar extracts SPDX files from a tar stream.
func extractSPDXFromTar(r io.Reader, source string) ([]SPDXFile, error) {
	tr := tar.NewReader(r)

	var files []SPDXFile

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// Check if the file is an SPDX file
		if !strings.HasSuffix(hdr.Name, SPDXFileSuffix) {
			continue
		}

		// Read the file content
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("error reading SPDX file %q: %w", hdr.Name, err)
		}

		files = append(files, SPDXFile{
			Filename: filepath.Base(hdr.Name),
			Source:   source,
			Content:  content,
		})
	}

	return files, nil
}
