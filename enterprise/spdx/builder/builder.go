// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/CalebQ42/squashfs"
	"github.com/blang/semver/v4"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/value"
	"github.com/u-root/u-root/pkg/cpio"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/enterprise/spdx/storage"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/schematic"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// AuthProvider is a subset of enterprise.AuthProvider used for ownership checks.
// Defined locally to avoid an import cycle with pkg/enterprise.
type AuthProvider interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// Builder orchestrates SPDX extraction and caching.
type Builder struct {
	storage          storage.Storage
	artifactsManager *artifacts.Manager
	assetBuilder     *asset.Builder
	schematicFactory *schematic.Factory
	logger           *zap.Logger
	sf               singleflight.Group
	authProvider     AuthProvider
}

// Options defines the dependencies for the SPDX builder.
type Options struct {
	Storage          storage.Storage
	ArtifactsManager *artifacts.Manager
	SchematicFactory *schematic.Factory
	AssetBuilder     *asset.Builder
	AuthProvider     AuthProvider
}

// NewBuilder creates a new SPDX bundle builder.
func NewBuilder(
	logger *zap.Logger,
	opts Options,
) *Builder {
	return &Builder{
		storage:          opts.Storage,
		artifactsManager: opts.ArtifactsManager,
		schematicFactory: opts.SchematicFactory,
		assetBuilder:     opts.AssetBuilder,
		authProvider:     opts.AuthProvider,
		logger:           logger.With(zap.String("component", "spdx-builder")),
	}
}

// Build returns an SPDX bundle, building and caching if necessary.
func (b *Builder) Build(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (storage.Bundle, error) {
	// Normalize version tag
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	// Validate version format
	if _, err := semver.Parse(versionTag[1:]); err != nil {
		return nil, fmt.Errorf("invalid version: %w", err)
	}

	// Check cache first
	if err := b.storage.Head(ctx, schematicID, versionTag, string(arch)); err == nil {
		b.logger.Debug("SPDX bundle cache hit", zap.String("schematic", schematicID), zap.String("version", versionTag), zap.String("arch", string(arch)))

		return b.storage.Get(ctx, schematicID, versionTag, string(arch))
	}

	// Verify access and fetch schematic data before entering singleflight.
	// buildBundle runs with context.Background() (request context may be canceled),
	// so ownership enforcement must happen here with the live request context.
	sc, err := b.schematicFactory.Get(ctx, schematicID, b.authProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get schematic: %w", err)
	}

	// Build the bundle using singleflight to prevent duplicate work
	cacheKey := CacheTag(schematicID, versionTag, string(arch))

	resultCh := b.sf.DoChan(cacheKey, func() (any, error) { //nolint:contextcheck
		return nil, b.buildBundle(sc, schematicID, versionTag, arch)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}

		// Retrieve from cache after building
		return b.storage.Get(ctx, schematicID, versionTag, string(arch))
	}
}

// buildBundle creates and stores an SPDX bundle for a single architecture.
// sc must be pre-fetched by the caller (Build) using the live request context,
// since this function runs inside singleflight with context.Background().
func (b *Builder) buildBundle(sc *schematicpkg.Schematic, schematicID, versionTag string, arch artifacts.Arch) error {
	// Use a fresh context since we're in singleflight
	ctx := context.Background()

	logger := b.logger.With(zap.String("schematic", schematicID), zap.String("version", versionTag), zap.String("arch", string(arch)))

	logger.Info("building SPDX bundle")

	bundle := &Bundle{
		SchematicID:  schematicID,
		TalosVersion: versionTag,
		Arch:         string(arch),
		Files:        []File{},
	}

	logger.Debug("extracting SPDX from Talos",
		zap.String("schematic", schematicID),
		zap.String("version", versionTag),
		zap.String("arch", string(arch)))

	// Extract SPDX from Talos
	var err error
	if err = b.extractTalosSPDX(ctx, bundle, versionTag, arch); err != nil {
		return fmt.Errorf("failed to extract SPDX from Talos: %w", err)
	}

	logger.Debug("building SPDX bundle from extensions",
		zap.Int("extensions", len(sc.Customization.SystemExtensions.OfficialExtensions)))

	// Extract SPDX from extensions
	if len(sc.Customization.SystemExtensions.OfficialExtensions) > 0 {
		if err = b.extractExtensionsSPDX(ctx, bundle, sc, versionTag, arch); err != nil {
			logger.Warn("failed to extract SPDX from some extensions", zap.Error(err))
		}
	}

	// Create merged SPDX JSON document
	jsonReader, size, err := BundleToJSON(bundle)
	if err != nil {
		return fmt.Errorf("failed to create SPDX JSON document: %w", err)
	}

	// Store the bundle
	if err := b.storage.Put(ctx, schematicID, versionTag, string(arch), jsonReader, size); err != nil {
		return fmt.Errorf("failed to store SPDX bundle: %w", err)
	}

	logger.Info("SPDX bundle created", zap.Int("files", len(bundle.Files)))

	return nil
}

// extractTalosSPDX extracts SPDX from the Talos initramfs for a single architecture.
func (b *Builder) extractTalosSPDX(ctx context.Context, bundle *Bundle, versionTag string, arch artifacts.Arch) error {
	path := fmt.Sprintf("initramfs-%s.xz", arch)

	prof, err := profile.ParseFromPath(path, versionTag)
	if err != nil {
		return fmt.Errorf("error parsing profile from path: %w", err)
	}

	// Validate version format
	talosVersion, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}

	asset, err := b.assetBuilder.Build(ctx, prof, talosVersion.String(), path, path)
	if err != nil {
		return err
	}

	// asset is zstd compressed CPIO archive
	// It additionallt contains SquashFS root filesystem.
	// We need to exract SPDX files from the root filesystem inside.extractTalosSPDX
	return b.extractSPDXFromInitramfs(bundle, asset)
}

// extractSPDXFromInitramfs extracts SPDX files from the embedded SquashFS
// inside the zstd-compressed CPIO initramfs asset, adding them to the bundle.
//
//nolint:gocognit
func (b *Builder) extractSPDXFromInitramfs(bundle *Bundle, bootAsset asset.BootAsset) error {
	// 1. Obtain an io.Reader from the BootAsset.
	assetReader, err := bootAsset.Reader()
	if err != nil {
		return fmt.Errorf("failed to get reader for boot asset: %w", err)
	}
	defer assetReader.Close() //nolint:errcheck

	// 2. Initialize the zstd decompressor
	zr, err := zstd.NewReader(assetReader)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zr.Close() //nolint:errcheck

	// 3. Decompress the entire CPIO archive into memory.
	// Since both u-root's cpio and standard squashfs parsers require io.ReaderAt
	// for random access, we must load the uncompressed stream.
	uncompressedCPIO, err := io.ReadAll(zr)
	if err != nil {
		return fmt.Errorf("failed to decompress zstd initramfs: %w", err)
	}

	br := bytes.NewReader(uncompressedCPIO)
	cr := cpio.Newc.Reader(br)

	// 4. Iterate over the CPIO records to locate the SquashFS root filesystem
	for {
		rec, err := cr.ReadRecord()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("failed to read cpio record: %w", err)
		}

		name := strings.TrimPrefix(rec.Name, "/")

		// Talos embeds the rootfs as a .sqsh file
		if strings.HasSuffix(name, ".sqsh") {
			// Create a section reader strictly bound to the exact bytes of the SquashFS payload
			sqfsReader := io.NewSectionReader(rec, 0, int64(rec.FileSize))

			// 5. Initialize the SquashFS reader
			sqfs, err := squashfs.NewReader(sqfsReader)
			if err != nil {
				return fmt.Errorf("failed to parse squashfs %q: %w", name, err)
			}

			// 6. Dynamically obtain an standard fs.FS interface
			var fsys fs.FS

			if fsProvider, ok := any(sqfs).(fs.FS); ok {
				fsys = fsProvider
			} else {
				return fmt.Errorf("squashfs reader does not support standard fs.FS interface")
			}

			// 7. Walk the SquashFS to find and extract SPDX files
			err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Skip unreadable paths quietly
				}

				if d.IsDir() {
					return nil
				}

				if !strings.HasSuffix(path, artifacts.SPDXFileSuffix) {
					return nil
				}

				// Extract the SPDX JSON
				file, err := fsys.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open spdx file %q in squashfs: %w", path, err)
				}
				defer file.Close() //nolint:errcheck

				content, err := io.ReadAll(file)
				if err != nil {
					return fmt.Errorf("failed to read spdx file %q in squashfs: %w", path, err)
				}

				bundle.Files = append(bundle.Files, File{
					Filename: filepath.Base(path),
					Source:   "talos",
					Content:  content,
				})

				return nil
			})
			if err != nil {
				return fmt.Errorf("error walking squashfs for spdx files: %w", err)
			}

			// There is only one target squashfs file in the initramfs,
			// we can break out early to save CPU cycles.
			break
		}
	}

	return nil
}

// extractExtensionsSPDX extracts SPDX from all extensions in the schematic for a single architecture.
func (b *Builder) extractExtensionsSPDX(ctx context.Context, bundle *Bundle, schematicData *schematicpkg.Schematic, versionTag string, arch artifacts.Arch) error {
	availableExtensions, err := b.artifactsManager.GetOfficialExtensions(ctx, versionTag)
	if err != nil {
		return fmt.Errorf("failed to get official extensions: %w", err)
	}

	for _, extensionName := range schematicData.Customization.SystemExtensions.OfficialExtensions {
		extensionRef := findExtension(availableExtensions, extensionName)

		if value.IsZero(extensionRef) {
			// Try with aliases
			if aliasedName, ok := profile.ExtensionNameAlias(extensionName); ok {
				extensionRef = findExtension(availableExtensions, aliasedName)
			}
		}

		if value.IsZero(extensionRef) {
			b.logger.Warn("extension not found, skipping SPDX extraction",
				zap.String("extension", extensionName),
				zap.String("version", versionTag))

			continue
		}

		// Extract SPDX for the requested architecture
		files, err := b.artifactsManager.ExtractExtensionSPDX(ctx, arch, extensionRef)
		if err != nil {
			b.logger.Warn("failed to extract SPDX from extension",
				zap.String("extension", extensionName),
				zap.String("arch", string(arch)),
				zap.Error(err))

			continue
		}

		if len(files) == 0 {
			b.logger.Debug("no SPDX files in extension",
				zap.String("extension", extensionName),
				zap.String("arch", string(arch)))

			continue
		}

		// Set the source to the extension name with arch
		shortName := extensionName
		if idx := strings.LastIndex(extensionName, "/"); idx >= 0 {
			shortName = extensionName[idx+1:]
		}

		for _, f := range files {
			bundle.Files = append(bundle.Files, File{
				Filename: f.Filename,
				Source:   fmt.Sprintf("%s-%s", shortName, arch),
				Content:  f.Content,
			})
		}
	}

	return nil
}

// findExtension finds an extension by name in the available extensions list.
func findExtension(availableExtensions []artifacts.ExtensionRef, extensionName string) artifacts.ExtensionRef {
	for _, availableExtension := range availableExtensions {
		if availableExtension.TaggedReference.RepositoryStr() == extensionName {
			return availableExtension
		}
	}

	return artifacts.ExtensionRef{}
}
