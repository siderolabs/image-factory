// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package registry owns Installer registry orchestration independently of HTTP.
package registry

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	buildversion "github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// Error tags describe domain failures; transports decide how to render them.
type (
	InvalidImageTag     struct{}
	ProxyUnavailableTag struct{}
	NotFoundTag         struct{}
)

// SchematicSource retrieves schematics under the caller's access policy.
type SchematicSource interface {
	Get(context.Context, string) (*schematic.Schematic, error)
}

// VersionSource lists versions available for Installer builds.
type VersionSource interface {
	GetTalosVersions(context.Context) ([]semver.Version, error)
}

// AssetBuilder builds a resolved Installer profile.
type AssetBuilder interface {
	Build(context.Context, talosprofile.Profile, string, string, string) (asset.BootAsset, error)
}

// ProfileEnhancer resolves customization and records consumed dependencies.
type ProfileEnhancer func(context.Context, talosprofile.Profile, *schematic.Schematic, string) (profile.EnhancementResult, error)

// ImageSigner signs and verifies published Installer indexes.
type ImageSigner interface {
	SignImage(context.Context, name.Digest, remotewrap.Pusher) error
	VerifyImage(context.Context, name.Digest, remotewrap.Puller) error
}

// EvidencePublisher publishes and verifies an Installer evidence graph.
type EvidencePublisher interface {
	Publish(context.Context, installer.EvidenceInput) error
	Verify(context.Context, installer.EvidenceInput) error
}

// Options configures registry repositories and publication dependencies.
type Options struct {
	Puller             remotewrap.Puller
	Pusher             remotewrap.Pusher
	Signer             ImageSigner
	EvidencePublisher  EvidencePublisher
	ProxyImages        map[string]string
	Logger             *zap.Logger
	InternalRepository name.Repository
	ExternalRepository name.Repository
	ProxyRegistry      name.Registry
	ProxyNamespace     string
	ProxyInternal      bool
	AuthEnabled        bool
}

// Service owns registry lookup, detached builds, and immutable publication.
type Service struct {
	schematics SchematicSource
	versions   VersionSource
	builder    AssetBuilder
	sf         singleflight.Group
	enhance    ProfileEnhancer
	options    Options
}

// New creates an Installer registry service.
func New(schematics SchematicSource, versions VersionSource, builder AssetBuilder, enhance ProfileEnhancer, options Options) *Service {
	if options.Logger == nil {
		options.Logger = zap.NewNop()
	}

	return &Service{schematics: schematics, versions: versions, builder: builder, enhance: enhance, options: options}
}

func (f *Service) reqLogger(ctx context.Context) *zap.Logger {
	return ctxlog.Logger(ctx, f.options.Logger)
}

// Request identifies a registry object without transport state.
type Request struct {
	Image     string
	Schematic string
	Reference string
	Resource  string
}

// Location selects the backing repository and immutable object to deliver.
type Location struct {
	Scheme     string
	Host       string
	Repository string
	Resource   string
	Reference  string
	Proxy      bool
}

// Referrers contains the resolved OCI index and its digest.
type Referrers struct {
	Digest   v1.Hash
	Manifest []byte
}

type requestedImage struct {
	imageName  string
	platform   string
	secureboot bool
}

func getRequestedImage(image string) (requestedImage, error) {
	switch image {
	case "installer":
		// defaults to metal image
		return requestedImage{imageName: image, secureboot: false}, nil
	case "installer-secureboot":
		return requestedImage{imageName: image, secureboot: true}, nil
	default:
		// newer installer has `-installer` as suffix
		// Eg: metal-installer, metal-installer-secureboot, digital-ocean-installer etc
		// first try `-installer-secureboot` and then `-installer`
		platform, ok := strings.CutSuffix(image, "-installer-secureboot")
		if ok {
			return requestedImage{imageName: image, platform: platform, secureboot: true}, nil
		}

		if platform, ok = strings.CutSuffix(image, "-installer"); ok {
			return requestedImage{imageName: image, platform: platform, secureboot: false}, nil
		}

		return requestedImage{}, xerrors.NewTaggedf[InvalidImageTag]("invalid image: %s", image)
	}
}
func (img requestedImage) Name() string { return img.imageName }

func (f *Service) location(imageName, schematicID, resource, reference string) Location {
	repo := f.options.ExternalRepository
	if f.options.ProxyInternal {
		repo = f.options.InternalRepository
	}

	return Location{
		Scheme: repo.Scheme(), Host: repo.Registry.Name(),
		Repository: repo.RepositoryStr() + "/" + imageName + "/" + schematicID,
		Resource:   resource, Reference: reference, Proxy: f.options.ProxyInternal || f.options.AuthEnabled,
	}
}

// Blob validates ownership and selects the backing blob repository.
func (f *Service) Blob(ctx context.Context, request Request) (Location, error) {
	if _, err := f.schematics.Get(ctx, request.Schematic); err != nil {
		return Location{}, err
	}

	img, err := getRequestedImage(request.Image)
	if err != nil {
		return Location{}, err
	}

	return f.location(img.Name(), request.Schematic, "blobs", request.Reference), nil
}

// DiscoverReferrers validates the subject and discovers its evidence graph.
func (f *Service) DiscoverReferrers(ctx context.Context, request Request, artifactType string) (Referrers, error) {
	if _, err := f.schematics.Get(ctx, request.Schematic); err != nil {
		return Referrers{}, err
	}

	img, err := getRequestedImage(request.Image)
	if err != nil {
		return Referrers{}, err
	}

	if _, err = v1.NewHash(request.Reference); err != nil {
		return Referrers{}, xerrors.NewTaggedf[NotFoundTag]("invalid referrers subject digest: %s", request.Reference)
	}

	repository := f.options.InternalRepository.Repo(f.options.InternalRepository.RepositoryStr(), img.Name(), request.Schematic)

	manifest, digest, err := ResolveInstallerReferrers(ctx, repository.Digest(request.Reference), artifactType, f.options.Puller)
	if err != nil {
		return Referrers{}, fmt.Errorf("failed to resolve Installer referrers: %w", err)
	}

	return Referrers{Manifest: manifest, Digest: digest}, nil
}

// Proxy selects an allowed image repository in the backing registry.
func (f *Service) Proxy(_ context.Context, request Request) (Location, error) {
	if f.options.ProxyRegistry.Scheme() != "http" {
		return Location{}, xerrors.NewTaggedf[ProxyUnavailableTag]("proxying to an authorized/secure backing registry is not possible")
	}

	repository, ok := f.options.ProxyImages[request.Image]
	if !ok {
		return Location{}, xerrors.NewTaggedf[NotFoundTag]("unknown image: %s", request.Image)
	}

	return Location{
		Scheme: f.options.ProxyRegistry.Scheme(), Host: f.options.ProxyRegistry.Name(),
		Repository: f.options.ProxyNamespace + "/" + repository,
		Resource:   request.Resource, Reference: request.Reference, Proxy: true,
	}, nil
}

// Manifest resolves cached content or builds and publishes an Installer index.
func (f *Service) Manifest(ctx context.Context, request Request) (Location, error) {
	schematicID := request.Schematic

	configuration, err := f.schematics.Get(ctx, schematicID)
	if err != nil {
		return Location{}, err
	}

	versionTag := request.Reference

	img, err := getRequestedImage(request.Image)
	if err != nil {
		return Location{}, err
	}
	// if the tag is "latest", replace it with latest known stable version to the factory.
	if versionTag == "latest" {
		versionTag, err = f.resolveLatest(ctx, schematicID)
		if err != nil {
			return Location{}, fmt.Errorf("error resolving latest version: %w", err)
		}
	}
	// if the tag is the digest, or it doesn't look like the version, we just redirect to the external registry
	if strings.HasPrefix(versionTag, "sha256:") || !strings.HasPrefix(versionTag, "v") {
		return f.location(img.Name(), schematicID, "manifests", versionTag), nil
	}

	imageRepository := f.options.InternalRepository.Repo(f.options.InternalRepository.RepositoryStr(), img.Name(), schematicID)
	// check if the asset has already been built
	f.reqLogger(ctx).Info("heading installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID),
		zap.String("version", versionTag), zap.Stringer("ref", imageRepository.Tag(versionTag)))

	extDesc, err := f.options.Puller.Head(ctx, imageRepository.Tag(versionTag))
	if err == nil {
		// The asset has already been built, so verify its completion signature.
		indexRef := imageRepository.Digest(extDesc.Digest.String())
		f.reqLogger(ctx).Info("verifying cached installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID),
			zap.String("version", versionTag), zap.Stringer("ref", indexRef))

		signatureErr := f.options.Signer.VerifyImage(ctx, indexRef, f.options.Puller)
		if signatureErr == nil {
			// Redirect to the external registry using the immutable digest.
			return f.location(img.Name(), schematicID, "manifests", extDesc.Digest.String()), nil
		}
		// Log the signature verification error, but continue to rebuild the image.
		f.reqLogger(ctx).Error("error verifying cached image signature", zap.String("image", img.Name()), zap.String("schematic", schematicID),
			zap.String("version", versionTag), zap.Error(signatureErr))
	}

	if regtransport.IsStatusCodeError(err, 404, 403) {
		// ignore 404/403, it means the image hasn't been built yet
		err = nil
	}

	if err != nil {
		return Location{}, err
	}
	// installer image is not built yet, build it and push it
	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return Location{}, fmt.Errorf("error parsing version: %w", err)
	}
	// build installer images for each architecture, combine them into a single index and push it
	key := fmt.Sprintf("%s-%s-%s", img.Name(), schematicID, versionTag)
	// carry the request ID into the detached build so build logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)
	resultCh := f.sf.DoChan(key, func() (any, error) { //nolint:contextcheck
		// Keep request-scoped authorization values while detaching cancellation so
		// evidence publication can enforce schematic ownership after the request ends.
		detachedCtx := ctxlog.WithRequestID(context.WithoutCancel(ctx), reqID)

		return f.buildInstallImage(detachedCtx, img, configuration, version, schematicID, versionTag)
	})

	var res singleflight.Result
	select {
	case res = <-resultCh:
		if res.Err != nil {
			return Location{}, res.Err
		}
	case <-ctx.Done():
		return Location{}, ctx.Err()
	}

	manifestHash, ok := res.Val.(v1.Hash)
	if !ok {
		return Location{}, fmt.Errorf("unexpected result type: %T", res.Val)
	}
	// now we can redirect to the external registry
	return f.location(img.Name(), schematicID, "manifests", manifestHash.String()), nil
}

func (f *Service) resolveLatest(ctx context.Context, schematicID string) (string, error) {
	ver, err := f.versions.GetTalosVersions(ctx)
	if err != nil {
		return "", err
	}

	semver.Sort(ver)
	slices.Reverse(ver)

	var versionTag string

	for _, v := range ver {
		if len(v.Pre) == 0 {
			versionTag = "v" + v.String()

			break
		}
	}

	f.reqLogger(ctx).Info("resolving latest tag to version", zap.String("schematic", schematicID), zap.String("version", versionTag))

	return versionTag, nil
}

func (f *Service) buildInstallImage(ctx context.Context, img requestedImage, configuration *schematic.Schematic, version semver.Version, schematicID, versionTag string) (v1.Hash, error) {
	f.reqLogger(ctx).Info("building installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag))

	startedOn := time.Now().UTC()
	installerRepo := f.options.InternalRepository.Repo(f.options.InternalRepository.RepositoryStr(), img.Name(), schematicID)

	var imageIndex v1.ImageIndex = empty.Index

	platforms := make([]installer.PlatformArtifact, 0, 2)
	dependencies := make([]installer.ResolvedDependency, 0)

	for _, arch := range []artifacts.Arch{artifacts.ArchAmd64, artifacts.ArchArm64} {
		prof := profile.InstallerProfile(img.secureboot, arch, img.platform)

		enhancement, err := f.enhance(ctx, prof, configuration, versionTag)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error enhancing profile from schematic: %w", err)
		}

		prof = enhancement.Profile

		var bootAsset asset.BootAsset

		bootAsset, err = f.builder.Build(ctx, prof, version.String(), img.Name(), "")
		if err != nil {
			return v1.Hash{}, err
		}

		archImage, err := tarball.Image(bootAsset.Reader, nil)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error creating image from asset: %w", err)
		}

		digest, err := archImage.Digest()
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error getting %s image digest: %w", arch, err)
		}

		size, err := archImage.Size()
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error getting %s image size: %w", arch, err)
		}

		mediaType, err := archImage.MediaType()
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error getting %s image media type: %w", arch, err)
		}

		platform := v1.Platform{Architecture: prof.Arch, OS: "linux"}
		descriptor := v1.Descriptor{Digest: digest, Size: size, MediaType: mediaType, Platform: &platform}
		imageIndex = mutate.AppendManifests(imageIndex, mutate.IndexAddendum{Add: archImage, Descriptor: descriptor})

		platforms = append(platforms, installer.PlatformArtifact{Platform: platform, Ref: installerRepo.Digest(digest.String())})
		for _, dependency := range enhancement.Dependencies {
			dependencies = append(dependencies, resolvedImageDependency(dependency.Kind, dependency.Arch, dependency.Image))
		}
	}

	schematicDigest, err := v1.NewHash("sha256:" + schematicID)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("invalid schematic digest: %w", err)
	}

	dependencies = append(dependencies, installer.ResolvedDependency{
		Name: "schematic", URI: "urn:siderolabs:image-factory:schematic:" + schematicID,
		Digest: map[string]string{schematicDigest.Algorithm: schematicDigest.Hex},
	})

	indexDigest, err := imageIndex.Digest()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("error getting index digest: %w", err)
	}

	indexRef := installerRepo.Digest(indexDigest.String())

	invocationID := ctxlog.RequestID(ctx)
	if invocationID == "" {
		invocationID = uuid.NewString()
	}

	evidenceInput := installer.EvidenceInput{
		IndexRef: indexRef, Platforms: platforms, ImageName: img.Name(), SchematicID: schematicID,
		TalosVersion: versionTag, SecureBoot: img.secureboot, Platform: img.platform,
		InvocationID: invocationID, StartedOn: startedOn, FinishedOn: time.Now().UTC(),
		BuilderVersion:       map[string]string{"tag": buildversion.Tag, "sha": buildversion.SHA},
		ResolvedDependencies: dependencies,
	}
	f.reqLogger(ctx).Info("publishing installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag), zap.Stringer("digest", indexDigest))

	evidencePublisher := f.options.EvidencePublisher
	if !InstallerEvidenceSupported(version) {
		evidencePublisher = nil
	}

	if err = PublishInstallerIndex(ctx, imageIndex, indexRef, installerRepo.Tag(versionTag), evidenceInput, f.options.Pusher, f.options.Puller, f.options.Signer, evidencePublisher); err != nil {
		return v1.Hash{}, err
	}

	return indexDigest, nil
}

func resolvedImageDependency(kind string, arch artifacts.Arch, dependency artifacts.ImageDependency) installer.ResolvedDependency {
	return installer.ResolvedDependency{
		Name: fmt.Sprintf("%s:%s:linux/%s", kind, dependency.Name, arch), URI: dependency.Ref.String(),
		Digest:    map[string]string{dependency.Descriptor.Digest.Algorithm: dependency.Descriptor.Digest.Hex},
		MediaType: string(dependency.Descriptor.MediaType),
	}
}
