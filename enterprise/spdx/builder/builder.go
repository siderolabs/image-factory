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
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-vex/pkg/kernelversion"
	"github.com/u-root/u-root/pkg/cpio"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/enterprise/spdx/storage"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/artifacts/imagehandler"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/schematic"
	ifconstants "github.com/siderolabs/image-factory/pkg/constants"
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
	sf               singleflight.Group
	authProvider     AuthProvider
	artifactsManager *artifacts.Manager
	assetBuilder     *asset.Builder
	schematicFactory *schematic.Factory
	logger           *zap.Logger
	externalURL      string
}

// Options defines the dependencies for the SPDX builder.
type Options struct {
	Storage          storage.Storage
	AuthProvider     AuthProvider
	ArtifactsManager *artifacts.Manager
	SchematicFactory *schematic.Factory
	AssetBuilder     *asset.Builder
	ExternalURL      string
}

// NewBuilder creates a new SPDX bundle builder.
func NewBuilder(
	logger *zap.Logger,
	opts Options,
) *Builder {
	return &Builder{
		externalURL:      opts.ExternalURL,
		storage:          opts.Storage,
		artifactsManager: opts.ArtifactsManager,
		schematicFactory: opts.SchematicFactory,
		assetBuilder:     opts.AssetBuilder,
		authProvider:     opts.AuthProvider,
		logger:           logger.With(zap.String("component", "spdx-builder")),
	}
}

// PayloadHash returns the SBOM cache key for the given schematic/version/arch.
//
// It fetches the schematic to extract the extension list, then computes a
// content hash that reflects only the inputs that affect the SPDX bundle
// content. Callers should use this hash as a cache key so that schematics
// differing only in non-SBOM fields (kernel args, config, etc.) share
// cached bundles.
func (b *Builder) PayloadHash(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (string, error) {
	sc, err := b.schematicFactory.Get(ctx, schematicID, b.authProvider)
	if err != nil {
		return "", fmt.Errorf("failed to get schematic: %w", err)
	}

	return Hash(sc.Customization.SystemExtensions.OfficialExtensions, versionTag, string(arch)), nil
}

// KernelVersion returns the kernel package version from the canonical Talos SPDX.
// The Talos kernel version is architecture-independent, so one architecture is
// sufficient for a version-scoped VEX document.
func (b *Builder) KernelVersion(ctx context.Context, versionTag string) (string, error) {
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	bundle := &Bundle{}
	if err := b.extractTalosSPDX(ctx, bundle, versionTag, artifacts.ArchAmd64); err != nil {
		return "", fmt.Errorf("failed to extract canonical Talos SPDX: %w", err)
	}

	return kernelVersionFromFiles(bundle.Files, artifacts.ArchAmd64)
}

func kernelVersionFromFiles(files []File, arch artifacts.Arch) (string, error) {
	filename := fmt.Sprintf("talos-%s.spdx.json", arch)

	for _, file := range files {
		if file.Filename != filename {
			continue
		}

		version, err := kernelversion.FromSPDXJSON(file.Content)
		if err != nil {
			return "", fmt.Errorf("failed to read kernel version from %q: %w", filename, err)
		}

		if version == "" {
			return "", fmt.Errorf("kernel package is missing from %q", filename)
		}

		return version, nil
	}

	return "", fmt.Errorf("talos SPDX file %q is missing", filename)
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

	// Fetch schematic first: we need the extension list to derive the cache key.
	// Ownership enforcement happens here with the live request context, before
	// entering singleflight which uses a detached context.
	sc, err := b.schematicFactory.Get(ctx, schematicID, b.authProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get schematic: %w", err)
	}

	// Compute cache key from only the inputs that affect the SBOM content
	// (extensions list, version, architecture), so that schematics differing
	// in other fields share the same cached bundle.
	sbomHash := Hash(sc.Customization.SystemExtensions.OfficialExtensions, versionTag, string(arch))

	// Read and verify the cache entry before declaring a hit. A concurrent
	// builder may have pushed the image but not signed it yet; treating that
	// transient state as a miss makes this request join the same singleflight.
	cachedBundle, err := b.storage.Get(ctx, sbomHash)
	if err == nil {
		ctxlog.Logger(ctx, b.logger).Debug("SPDX bundle cache hit", zap.String("schematic", schematicID), zap.String("version", versionTag), zap.String("arch", string(arch)))

		return b.IdentifyAs(cachedBundle, sbomHash, schematicID, versionTag, arch)
	}

	if !xerrors.TagIs[storage.ErrNotFoundTag](err) {
		return nil, fmt.Errorf("failed to get cached SPDX bundle: %w", err)
	}

	// Build the bundle using singleflight to prevent duplicate work
	// carry the request ID into the detached build so its logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)

	resultCh := b.sf.DoChan(sbomHash, func() (any, error) { //nolint:contextcheck
		return nil, b.buildBundle(reqID, sc, schematicID, sbomHash, versionTag, arch)
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}

		// Retrieve from cache after building
		return b.identifiedBundle(ctx, sbomHash, schematicID, versionTag, arch)
	}
}

// identifiedBundle fetches the cached bundle and re-identifies it as the
// schematic that asked for it.
func (b *Builder) identifiedBundle(ctx context.Context, sbomHash, schematicID, versionTag string, arch artifacts.Arch) (storage.Bundle, error) {
	bundle, err := b.storage.Get(ctx, sbomHash)
	if err != nil {
		return nil, err
	}

	return b.IdentifyAs(bundle, sbomHash, schematicID, versionTag, arch)
}

// IdentifyAs presents a bundle stored under sbomHash as belonging to schematicID,
// substituting the document name and namespace as the bundle is read.
func (b *Builder) IdentifyAs(bundle storage.Bundle, sbomHash, schematicID, versionTag string, arch artifacts.Arch) (storage.Bundle, error) {
	fromNamespace, err := buildDocumentNamespace(b.externalURL, sbomHash, versionTag, string(arch))
	if err != nil {
		return nil, fmt.Errorf("failed to build cached document namespace: %w", err)
	}

	toNamespace, err := buildDocumentNamespace(b.externalURL, schematicID, versionTag, string(arch))
	if err != nil {
		return nil, fmt.Errorf("failed to build document namespace: %w", err)
	}

	return newIdentifiedBundle(bundle, []replacement{
		{from: documentName(sbomHash, versionTag, string(arch)), to: documentName(schematicID, versionTag, string(arch))},
		{from: fromNamespace, to: toNamespace},
	})
}

// newIdentifiedBundle validates that every substitution preserves length before
// wrapping the bundle.
//
// Size() keeps reporting the stored size, which the frontend has already sent as
// Content-Length, so a substitution that changed the length would produce a
// response body that contradicts its own framing. Both IDs are sha256 hex, so
// this holds; if it ever stops holding, fail the request instead of serving a
// truncated document.
func newIdentifiedBundle(bundle storage.Bundle, replacements []replacement) (storage.Bundle, error) {
	for _, rep := range replacements {
		if len(rep.from) != len(rep.to) {
			return nil, fmt.Errorf("document identity substitution %q -> %q would change the bundle size", rep.from, rep.to)
		}
	}

	return &identifiedBundle{
		Bundle:       bundle,
		replacements: replacements,
	}, nil
}

// replacement is a single from/to substitution applied to a served document.
type replacement struct {
	from string
	to   string
}

// identifiedBundle wraps a cached bundle, substituting the document identity as
// it is read.
//
// Only the two full name/namespace strings this package writes are substituted,
// never the bare hash, so a package checksum that happens to share the hash's
// shape cannot be touched. Schematic IDs and SBOM hashes are both 64-character
// hex, so every substitution preserves length and the wrapped Size stays exact —
// which keeps the Content-Length the frontend already sent from HEAD valid.
type identifiedBundle struct {
	storage.Bundle

	replacements []replacement
}

// Reader implements storage.Bundle.
func (b *identifiedBundle) Reader() (io.ReadCloser, error) {
	r, err := b.Bundle.Reader()
	if err != nil {
		return nil, err
	}

	defer r.Close() //nolint:errcheck

	// ponytail: buffers the document (~0.5-5 MB) to substitute in one pass. Swap
	// for a streaming replacer carrying len(from)-1 bytes between chunks if this
	// shows up in memory profiles.
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached SPDX bundle: %w", err)
	}

	for _, rep := range b.replacements {
		content = bytes.ReplaceAll(content, []byte(rep.from), []byte(rep.to))
	}

	return io.NopCloser(bytes.NewReader(content)), nil
}

// buildBundle creates and stores an SPDX bundle for a single architecture.
// sc must be pre-fetched by the caller (Build) using the live request context,
// since this function runs inside singleflight with context.Background().
func (b *Builder) buildBundle(reqID string, sc *schematicpkg.Schematic, schematicID, sbomHash, versionTag string, arch artifacts.Arch) error {
	// Use a fresh context since we're in singleflight, but carry the
	// request ID so build logs keep the request_id.
	ctx := ctxlog.WithRequestID(context.Background(), reqID)

	logger := ctxlog.Logger(ctx, b.logger).With(zap.String("schematic", schematicID), zap.String("version", versionTag), zap.String("arch", string(arch)))

	logger.Info("building SPDX bundle")

	bundle := &Bundle{
		// Name the document by the cache key, not the schematic: this bundle is
		// shared by every schematic with the same extensions, version and arch.
		ID:           sbomHash,
		TalosVersion: versionTag,
		Arch:         string(arch),
		ExternalURL:  b.externalURL,
		Files:        []File{},
	}

	logger.Debug("extracting SPDX from Talos")

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

	// Store the bundle keyed by the SBOM content hash
	if err := b.storage.Put(ctx, sbomHash, jsonReader, size); err != nil {
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

	// We are manually setting the profile version and name here because after enhancing the profile
	// the produced initramfs' magic number does not match what's expected by the decompression
	// library, causing it to fail to decompress and preventing us from extracting the embedded SPDX files.
	// That's why we are getting SBOM from each extension individually instead of relying on the
	// one embedded in the initramfs.
	prof.Version = talosVersion.String()
	prof.Name = ifconstants.TalosName

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
// inside the compressed CPIO initramfs asset, adding them to the bundle.
//
//nolint:gocognit
func (b *Builder) extractSPDXFromInitramfs(bundle *Bundle, bootAsset asset.BootAsset) error {
	// 1. Obtain an io.Reader from the BootAsset.
	assetReader, err := bootAsset.Reader()
	if err != nil {
		return fmt.Errorf("failed to get reader for boot asset: %w", err)
	}
	defer assetReader.Close() //nolint:errcheck

	// 2. Initialize the decompressor used by this Talos version.
	compressedReader, err := zstd.NewReader(assetReader)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer compressedReader.Close()

	// 3. Decompress the entire CPIO archive into memory.
	// Since both u-root's cpio and standard squashfs parsers require io.ReaderAt
	// for random access, we must load the uncompressed stream.
	uncompressedCPIO, err := io.ReadAll(compressedReader)
	if err != nil {
		return fmt.Errorf("failed to decompress initramfs: %w", err)
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

				if !strings.HasSuffix(path, imagehandler.SPDXFileSuffix) {
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
	logger := ctxlog.Logger(ctx, b.logger)

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
			logger.Warn("extension not found, skipping SPDX extraction",
				zap.String("extension", extensionName),
				zap.String("version", versionTag))

			continue
		}

		// Extract SPDX for the requested architecture
		files, err := b.artifactsManager.ExtractExtensionSPDX(ctx, arch, extensionRef)
		if err != nil {
			logger.Warn("failed to extract SPDX from extension",
				zap.String("extension", extensionName),
				zap.String("arch", string(arch)),
				zap.Error(err))

			continue
		}

		if len(files) == 0 {
			logger.Debug("no SPDX files in extension",
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
