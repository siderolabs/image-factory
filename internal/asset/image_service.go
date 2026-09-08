// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/xerrors"
	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"go.uber.org/zap"

	factoryprofile "github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/secureboot"
	enterrors "github.com/siderolabs/image-factory/pkg/enterprise/errors"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// ImageSchematicSource retrieves schematics under the caller's access policy.
type ImageSchematicSource interface {
	Get(ctx context.Context, id string) (*schematicpkg.Schematic, error)
}

// ImageAssetBuilder builds the resolved boot asset.
type ImageAssetBuilder interface {
	Build(ctx context.Context, profile talosprofile.Profile, version, path, filename string) (BootAsset, error)
}

// ImageProfileEnhancer applies schematic customization to an image profile.
type ImageProfileEnhancer func(
	ctx context.Context,
	profile talosprofile.Profile,
	configuration *schematicpkg.Schematic,
	versionTag string,
) (talosprofile.Profile, error)

// NewImageProfileEnhancer adapts profile-domain dependencies to image orchestration.
func NewImageProfileEnhancer(
	artifacts factoryprofile.ArtifactProducer,
	secureBoot *secureboot.Service,
) ImageProfileEnhancer {
	return func(
		ctx context.Context,
		imageProfile talosprofile.Profile,
		configuration *schematicpkg.Schematic,
		versionTag string,
	) (talosprofile.Profile, error) {
		return factoryprofile.EnhanceFromSchematic(ctx, imageProfile, configuration, artifacts, secureBoot, versionTag)
	}
}

// ImageServiceOptions describes optional image response capabilities.
type ImageServiceOptions struct {
	ChecksumGenerator  ChecksumGenerator
	SignatureGenerator SignatureGenerator
	Logger             *zap.Logger
}

// GeneratedArtifact is a transport-neutral generated sidecar.
type GeneratedArtifact struct {
	ContentType string
	Filename    string
	Content     []byte
}

// ChecksumGenerator computes a checksum sidecar from an asset stream.
type ChecksumGenerator interface {
	GenerateChecksum(ctx context.Context, reader io.Reader, filename, suffix string) (GeneratedArtifact, error)
}

// SignatureGenerator creates a detached signature sidecar for a boot asset.
type SignatureGenerator interface {
	GenerateSignature(ctx context.Context, asset BootAsset, assetKey, filename string) (GeneratedArtifact, error)
}

// ImageRequest identifies an image application request.
type ImageRequest struct {
	SchematicID   string
	Version       string
	Path          string
	Filename      string
	AllowRedirect bool
}

// ImageDelivery describes how a resolved image is delivered to a caller.
type ImageDelivery uint8

const (
	ImageDeliveryAsset ImageDelivery = iota
	ImageDeliveryChecksum
	ImageDeliverySignature
)

// RedirectableBootAsset can provide a temporary download URL.
type RedirectableBootAsset interface {
	BootAsset
	Redirect(ctx context.Context, filename string) (string, error)
}

// ResolvedImage is the domain result consumed by an HTTP or other transport adapter.
type ResolvedImage struct {
	Asset             BootAsset
	Generated         *GeneratedArtifact
	Path              string
	Filename          string
	AssetKey          string
	ChecksumAlgorithm string
	RedirectURL       string
	Delivery          ImageDelivery
}

// PXERequest identifies a PXE application request.
type PXERequest struct {
	SchematicID string
	Version     string
	Path        string
}

// ResolvedPXE is the application data needed to render a PXE script.
type ResolvedPXE struct {
	Profile talosprofile.Profile
	Version string
	Cmdline []byte
}

// ImageService owns image retrieval, profile construction, and build orchestration.
type ImageService struct {
	schematics ImageSchematicSource
	builder    ImageAssetBuilder
	enhance    ImageProfileEnhancer
	options    ImageServiceOptions
	logger     *zap.Logger
}

// NewImageService creates an image application service.
func NewImageService(
	schematics ImageSchematicSource,
	builder ImageAssetBuilder,
	enhance ImageProfileEnhancer,
	options ImageServiceOptions,
) *ImageService {
	logger := options.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ImageService{
		schematics: schematics,
		builder:    builder,
		enhance:    enhance,
		options:    options,
		logger:     logger,
	}
}

// Resolve retrieves and builds an image asset.
func (service *ImageService) Resolve(ctx context.Context, request ImageRequest) (ResolvedImage, error) {
	path, sidecar := factoryprofile.SplitArtifactPath(request.Path)
	if sidecar.IsChecksum() && service.options.ChecksumGenerator == nil {
		return ResolvedImage{}, xerrors.NewTaggedf[enterrors.NotEnabledTag]("enterprise not enabled: checksum endpoint is not available")
	}

	if sidecar == factoryprofile.ArtifactSidecarSignature && service.options.SignatureGenerator == nil {
		return ResolvedImage{}, xerrors.NewTaggedf[enterrors.NotEnabledTag]("enterprise signing is not enabled: signature endpoint is not available")
	}

	configuration, err := service.schematics.Get(ctx, request.SchematicID)
	if err != nil {
		return ResolvedImage{}, err
	}

	versionTag := request.Version
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return ResolvedImage{}, fmt.Errorf("error parsing version: %w", err)
	}

	imageProfile, err := factoryprofile.ParseArtifactPath(path, version.String())
	if err != nil {
		return ResolvedImage{}, fmt.Errorf("error parsing profile from path: %w", err)
	}

	imageProfile, err = service.enhance(ctx, imageProfile, configuration, versionTag)
	if err != nil {
		return ResolvedImage{}, fmt.Errorf("error enhancing profile from schematic: %w", err)
	}

	filename := request.Filename
	if filename == "" {
		filename = path
	}

	builtAsset, err := service.builder.Build(ctx, imageProfile, version.String(), path, filename)
	if err != nil {
		return ResolvedImage{}, err
	}

	resolved := ResolvedImage{
		Asset:    builtAsset,
		Path:     path,
		Filename: filename,
	}

	if err = service.resolveDelivery(ctx, request, sidecar, imageProfile, &resolved); err != nil {
		return ResolvedImage{}, err
	}

	return resolved, nil
}

func (service *ImageService) resolveDelivery(
	ctx context.Context,
	request ImageRequest,
	sidecar factoryprofile.ArtifactSidecar,
	imageProfile talosprofile.Profile,
	resolved *ResolvedImage,
) error {
	switch {
	case sidecar == factoryprofile.ArtifactSidecarSignature:
		resolved.Delivery = ImageDeliverySignature

		assetKey, err := factoryprofile.Hash(imageProfile)
		if err != nil {
			return fmt.Errorf("error hashing asset profile: %w", err)
		}

		resolved.AssetKey = assetKey

		generated, generateErr := service.options.SignatureGenerator.GenerateSignature(ctx, resolved.Asset, assetKey, resolved.Filename)
		if generateErr != nil {
			return generateErr
		}

		resolved.Generated = &generated
	case sidecar.IsChecksum():
		resolved.Delivery = ImageDeliveryChecksum
		resolved.ChecksumAlgorithm = string(sidecar)

		reader, readerErr := resolved.Asset.Reader()
		if readerErr != nil {
			return readerErr
		}
		defer reader.Close() //nolint:errcheck

		generated, generateErr := service.options.ChecksumGenerator.GenerateChecksum(ctx, reader, resolved.Filename, string(sidecar))
		if generateErr != nil {
			return generateErr
		}

		resolved.Generated = &generated
	case request.AllowRedirect:
		if redirectable, ok := resolved.Asset.(RedirectableBootAsset); ok {
			redirectURL, err := redirectable.Redirect(ctx, resolved.Filename)
			if err != nil {
				service.logger.Warn("asset does not support redirection, serving directly", zap.Error(err))

				return nil
			}

			resolved.RedirectURL = redirectURL
		}
	}

	return nil
}

// ResolvePXE retrieves and builds the kernel command line for a PXE script.
func (service *ImageService) ResolvePXE(ctx context.Context, request PXERequest) (ResolvedPXE, error) {
	configuration, err := service.schematics.Get(ctx, request.SchematicID)
	if err != nil {
		return ResolvedPXE{}, err
	}

	versionTag := request.Version
	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return ResolvedPXE{}, fmt.Errorf("error parsing version: %w", err)
	}

	path := "cmdline-" + request.Path

	imageProfile, err := factoryprofile.ParseFromPath(path, version.String())
	if err != nil {
		return ResolvedPXE{}, fmt.Errorf("error parsing profile from path: %w", err)
	}

	imageProfile, err = service.enhance(ctx, imageProfile, configuration, versionTag)
	if err != nil {
		return ResolvedPXE{}, fmt.Errorf("error enhancing profile from schematic: %w", err)
	}

	builtAsset, err := service.builder.Build(ctx, imageProfile, version.String(), path, "")
	if err != nil {
		return ResolvedPXE{}, err
	}

	reader, err := builtAsset.Reader()
	if err != nil {
		return ResolvedPXE{}, err
	}

	defer reader.Close() //nolint:errcheck

	cmdline, err := io.ReadAll(reader)
	if err != nil {
		return ResolvedPXE{}, err
	}

	return ResolvedPXE{
		Profile: imageProfile,
		Cmdline: cmdline,
		Version: versionTag,
	}, nil
}
