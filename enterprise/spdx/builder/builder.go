// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package builder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/enterprise/spdx/storage"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/schematic"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Builder orchestrates SPDX extraction and caching.
type Builder struct {
	storage          storage.Storage
	artifactsManager *artifacts.Manager
	schematicFactory *schematic.Factory
	logger           *zap.Logger
	sf               singleflight.Group
	generatedAt      func() time.Time
}

// Options defines the dependencies for the SPDX builder.
type Options struct {
	Storage          storage.Storage
	ArtifactsManager *artifacts.Manager
	SchematicFactory *schematic.Factory
	GeneratedAt      func() time.Time
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
		generatedAt:      opts.GeneratedAt,
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

	// Build the bundle using singleflight to prevent duplicate work
	cacheKey := CacheTag(schematicID, versionTag, string(arch))

	resultCh := b.sf.DoChan(cacheKey, func() (any, error) {
		return b.buildBundle(schematicID, versionTag, arch) //nolint:contextcheck
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
func (b *Builder) buildBundle(schematicID, versionTag string, arch artifacts.Arch) (any, error) {
	// Use a fresh context since we're in singleflight
	ctx := context.Background()

	logger := b.logger.With(zap.String("schematic", schematicID), zap.String("version", versionTag), zap.String("arch", string(arch)))

	logger.Info("building SPDX bundle")

	// Get the schematic to find extensions
	schematicData, err := b.schematicFactory.Get(ctx, schematicID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schematic: %w", err)
	}

	bundle := &Bundle{
		SchematicID:  schematicID,
		TalosVersion: versionTag,
		Arch:         string(arch),
		Files:        []File{},
	}

	// Extract SPDX from Talos installer for the requested architecture
	files, err := b.artifactsManager.ExtractInstallerSPDX(ctx, arch, versionTag)
	if err != nil {
		logger.Warn("failed to extract SPDX from Talos installer",
			zap.Error(err))
	} else {
		for _, f := range files {
			logger.Debug("adding SPDX file from Talos installer",
				zap.String("filename", f.Filename))

			bundle.Files = append(bundle.Files, File{
				Filename: f.Filename,
				Source:   fmt.Sprintf("talos-%s", arch),
				Content:  f.Content,
			})
		}
	}

	logger.Debug("building SPDX bundle from extensions",
		zap.Int("extensions", len(schematicData.Customization.SystemExtensions.OfficialExtensions)))

	// Extract SPDX from extensions
	if len(schematicData.Customization.SystemExtensions.OfficialExtensions) > 0 {
		if err := b.extractExtensionsSPDX(ctx, bundle, schematicData, versionTag, arch); err != nil {
			logger.Warn("failed to extract SPDX from some extensions", zap.Error(err))
		}
	}

	// Check if we have any files
	if len(bundle.Files) == 0 {
		return nil, xerrors.NewTaggedf[storage.ErrNotFoundTag]("no SPDX files found for schematic %q version %q arch %q", schematicID, versionTag, arch)
	}

	// Create merged SPDX JSON document
	jsonReader, size, err := BundleToJSON(bundle, b.generatedAt())
	if err != nil {
		return nil, fmt.Errorf("failed to create SPDX JSON document: %w", err)
	}

	// Store the bundle
	if err := b.storage.Put(ctx, schematicID, versionTag, string(arch), jsonReader, size); err != nil {
		return nil, fmt.Errorf("failed to store SPDX bundle: %w", err)
	}

	logger.Info("SPDX bundle created", zap.Int("files", len(bundle.Files)))

	return nil, nil
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
