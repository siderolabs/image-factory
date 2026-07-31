// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-retry/retry"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/installer"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	buildversion "github.com/siderolabs/image-factory/internal/version"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

var minimumInstallerEvidenceVersion = semver.MustParse("1.13.0")

const publishedVerificationTimeout = 30 * time.Second

// InstallerEvidenceSupported reports whether Talos uses the initramfs format
// required by Installer evidence generation.
func InstallerEvidenceSupported(version semver.Version) bool {
	return version.GTE(minimumInstallerEvidenceVersion)
}

// InvalidImageTag is an error tag for invalid image names.
type InvalidImageTag struct{}

// ProxyUnavailableTag is an error tag for when the backing registry cannot be proxied to.
type ProxyUnavailableTag struct{}

// handleHealth handles registry health and auth.
func (f *Frontend) handleHealth(_ context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	// always healthy, yay!
	return nil
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
		return requestedImage{
			imageName:  image,
			secureboot: false,
		}, nil
	case "installer-secureboot":
		return requestedImage{
			imageName:  image,
			secureboot: true,
		}, nil
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

func (img requestedImage) Name() string {
	return img.imageName
}

// handleBlob handles image blob download.
//
// We always redirect to the external registry, as we assume the image has already been pushed.
func (f *Frontend) handleBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, route V2Route) error {
	schematicID := route.Schematic

	// verify that schematic exists
	_, err := f.schematicFactory.Get(ctx, schematicID, f.options.AuthProvider)
	if err != nil {
		return err
	}

	img, err := getRequestedImage(route.Image)
	if err != nil {
		return err
	}

	digest := route.Reference

	return f.handleExternalRegistry(ctx, w, req, img.Name(), schematicID, "blobs", digest)
}

// handleReferrers serves OCI referrer discovery for generated Installer subjects.
func (f *Frontend) handleReferrers(ctx context.Context, w http.ResponseWriter, req *http.Request, route V2Route) error {
	schematicID := route.Schematic

	if _, err := f.schematicFactory.Get(ctx, schematicID, f.options.AuthProvider); err != nil {
		return err
	}

	img, err := getRequestedImage(route.Image)
	if err != nil {
		return err
	}

	if _, err = v1.NewHash(route.Reference); err != nil {
		return xerrors.NewTaggedf[RouteNotFoundTag]("invalid referrers subject digest: %s", route.Reference)
	}

	repository := f.options.InstallerInternalRepository.Repo(
		f.options.InstallerInternalRepository.RepositoryStr(),
		img.Name(),
		schematicID,
	)

	artifactType := req.URL.Query().Get("artifactType")

	manifest, digest, err := ResolveInstallerReferrers(
		ctx,
		repository.Digest(route.Reference),
		artifactType,
		f.puller,
	)
	if err != nil {
		return fmt.Errorf("failed to resolve Installer referrers: %w", err)
	}

	w.Header().Set("Content-Type", string(types.OCIImageIndex))
	w.Header().Set("Docker-Content-Digest", digest.String())
	w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
	applyReferrersFilterHeader(w.Header(), artifactType)

	_, err = w.Write(manifest)

	return err
}

func applyReferrersFilterHeader(header http.Header, artifactType string) {
	if artifactType != "" {
		header.Set("Oci-Filters-Applied", "artifactType")
	}
}

// ResolveInstallerReferrers discovers native referrers or the OCI referrers-tag fallback.
func ResolveInstallerReferrers(
	ctx context.Context,
	subject name.Digest,
	artifactType string,
	puller remotewrap.Puller,
) ([]byte, v1.Hash, error) {
	remoteOptions, err := puller.RemoteOptions()
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to get remote options: %w", err)
	}

	remoteOptions = append(remoteOptions, remote.WithContext(ctx))
	if artifactType != "" {
		remoteOptions = append(remoteOptions, remote.WithFilter("artifactType", artifactType))
	}

	referrers, err := remote.Referrers(subject, remoteOptions...)
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to discover OCI referrers: %w", err)
	}

	manifest, err := referrers.RawManifest()
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to read OCI referrers index: %w", err)
	}

	digest, err := referrers.Digest()
	if err != nil {
		return nil, v1.Hash{}, fmt.Errorf("failed to digest OCI referrers index: %w", err)
	}

	return manifest, digest, nil
}

// handleImageProxy proxies image requests to the backing registry.
func (f *Frontend) handleImageProxy(ctx context.Context, w http.ResponseWriter, req *http.Request, route V2Route) error {
	if f.options.ImageProxy.BackingRegistry.Scheme() != "http" {
		return xerrors.NewTaggedf[ProxyUnavailableTag]("proxying to an authorized/secure backing registry is not possible")
	}

	repository, ok := f.options.ImageProxy.Images[route.Image]
	if !ok {
		return xerrors.NewTaggedf[RouteNotFoundTag]("unknown image: %s", route.Image)
	}

	var redirectURL url.URL

	redirectURL.Scheme = f.options.ImageProxy.BackingRegistry.Scheme()
	redirectURL.Host = f.options.ImageProxy.BackingRegistry.Name()
	redirectURL.Path = "/"

	location := redirectURL.JoinPath("v2", f.options.ImageProxy.Namespace, repository, route.Resource, route.Reference)

	f.proxyRegistryRequest(ctx, location, w, req)

	return nil
}

func (f *Frontend) handleExternalRegistry(ctx context.Context, w http.ResponseWriter, req *http.Request, imageName, schematicID, manifestsOrBlobs, tagOrDigest string) error {
	repo := f.options.InstallerExternalRepository
	if f.options.ProxyInstallerInternalRepository {
		repo = f.options.InstallerInternalRepository
	}

	location := craftRedirectURL(repo, imageName, schematicID, manifestsOrBlobs, tagOrDigest)

	// When auth is active, always proxy - redirecting to the backing registry would bypass the auth boundary.
	if f.options.ProxyInstallerInternalRepository || f.options.AuthProvider != nil {
		f.proxyRegistryRequest(ctx, location, w, req)

		return nil
	}

	f.reqLogger(ctx).Info("redirecting manifest/blob", zap.Stringer("location", location))

	w.Header().Add("Location", location.String())
	w.WriteHeader(http.StatusTemporaryRedirect)

	return nil
}

func (f *Frontend) proxyRegistryRequest(ctx context.Context, location *url.URL, w http.ResponseWriter, req *http.Request) {
	f.reqLogger(ctx).Info("proxying registry request", zap.Stringer("location", location))

	proxy := &httputil.ReverseProxy{
		Director: func(out *http.Request) {
			out.URL.Scheme = location.Scheme
			out.URL.Host = location.Host
			out.URL.Path = location.Path
			out.URL.RawPath = ""

			if location.RawQuery != "" {
				out.URL.RawQuery = location.RawQuery
			}
			// we don't forward the host header to avoid TLS issues with some registries
			out.Host = ""
			out.Header.Del("Authorization")
		},
		Transport: remotewrap.GetTransport(),
	}

	proxy.ServeHTTP(w, req)
}

func craftRedirectURL(repo name.Repository, imageName string, schematicID string, manifestsOrBlobs string, tagOrDigest string) *url.URL {
	var redirectURL url.URL

	redirectURL.Scheme = repo.Scheme()
	redirectURL.Host = repo.Registry.Name()
	redirectURL.Path = "/"

	location := redirectURL.JoinPath("v2", repo.RepositoryStr(), imageName, schematicID, manifestsOrBlobs, tagOrDigest)

	return location
}

// handleManifest handles image manifest download.
//
// If the manifest is for the tag, we check if the image already exists, and either redirect, or build, push and redirect.
func (f *Frontend) handleManifest(ctx context.Context, w http.ResponseWriter, req *http.Request, route V2Route) error {
	schematicID := route.Schematic

	schematic, err := f.schematicFactory.Get(ctx, schematicID, f.options.AuthProvider)
	if err != nil {
		return err
	}

	versionTag := route.Reference

	img, err := getRequestedImage(route.Image)
	if err != nil {
		return err
	}

	// if the tag is "latest", replace it with latest known stable version to the factory.
	if versionTag == "latest" {
		versionTag, err = f.resolveLatest(ctx, schematicID)
		if err != nil {
			return fmt.Errorf("error resolving latest version: %w", err)
		}
	}

	// if the tag is the digest, or it doesn't look like the version, we just redirect to the external registry
	if strings.HasPrefix(versionTag, "sha256:") || !strings.HasPrefix(versionTag, "v") {
		return f.handleExternalRegistry(ctx, w, req, img.Name(), schematicID, "manifests", versionTag)
	}

	imageRepository := f.options.InstallerInternalRepository.Repo(
		f.options.InstallerInternalRepository.RepositoryStr(),
		img.Name(),
		schematicID,
	)

	// check if the asset has already been built
	f.reqLogger(ctx).Info(
		"heading installer image",
		zap.String("image", img.Name()),
		zap.String("schematic", schematicID),
		zap.String("version", versionTag),
		zap.Stringer("ref", imageRepository.Tag(versionTag)),
	)

	extDesc, err := f.puller.Head(
		ctx,
		imageRepository.Tag(versionTag),
	)
	if err == nil {
		// The asset has already been built, so verify its completion signature.
		indexRef := imageRepository.Digest(extDesc.Digest.String())
		f.reqLogger(ctx).Info(
			"verifying cached installer image",
			zap.String("image", img.Name()),
			zap.String("schematic", schematicID),
			zap.String("version", versionTag),
			zap.Stringer("ref", indexRef),
		)

		signatureErr := f.imageSigner.VerifyImage(ctx, indexRef, f.puller)
		if signatureErr == nil {
			// Redirect to the external registry using the immutable digest.
			return f.handleExternalRegistry(ctx, w, req, img.Name(), schematicID, "manifests", extDesc.Digest.String())
		}

		// Log the signature verification error, but continue to rebuild the image.
		f.reqLogger(ctx).Error(
			"error verifying cached image signature",
			zap.String("image", img.Name()),
			zap.String("schematic", schematicID),
			zap.String("version", versionTag),
			zap.Error(signatureErr),
		)
	}

	if regtransport.IsStatusCodeError(err, http.StatusNotFound, http.StatusForbidden) {
		// ignore 404/403, it means the image hasn't been built yet
		err = nil
	}

	if err != nil {
		// something is wrong
		return err
	}

	// installer image is not built yet, build it and push it
	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	// build installer images for each architecture, combine them into a single index and push it
	key := fmt.Sprintf("%s-%s-%s", img.Name(), schematicID, versionTag)

	// carry the request ID into the detached build so build logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)

	resultCh := f.sf.DoChan(key, func() (any, error) { //nolint:contextcheck
		// Keep request-scoped authorization values while detaching cancellation so
		// evidence publication can enforce schematic ownership after the request ends.
		detachedCtx := ctxlog.WithRequestID(context.WithoutCancel(ctx), reqID)

		return f.buildInstallImage(detachedCtx, img, schematic, version, schematicID, versionTag)
	})

	var res singleflight.Result

	select {
	case res = <-resultCh:
		if res.Err != nil {
			return res.Err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	manifestHash, ok := res.Val.(v1.Hash)
	if !ok {
		// unexpected
		return fmt.Errorf("unexpected result type: %T", res.Val)
	}

	// now we can redirect to the external registry
	return f.handleExternalRegistry(ctx, w, req, img.Name(), schematicID, "manifests", manifestHash.String())
}

func (f *Frontend) resolveLatest(ctx context.Context, schematicID string) (string, error) {
	ver, err := f.artifactsManager.GetTalosVersions(ctx)
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

func (f *Frontend) buildInstallImage(ctx context.Context, img requestedImage, schematic *schematic.Schematic, version semver.Version, schematicID, versionTag string) (v1.Hash, error) {
	f.reqLogger(ctx).Info("building installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag))

	startedOn := time.Now().UTC()
	installerRepo := f.options.InstallerInternalRepository.Repo(
		f.options.InstallerInternalRepository.RepositoryStr(),
		img.Name(),
		schematicID,
	)

	var imageIndex v1.ImageIndex = empty.Index

	platforms := make([]installer.PlatformArtifact, 0, 2)
	dependencies := make([]installer.ResolvedDependency, 0)

	for _, arch := range []artifacts.Arch{artifacts.ArchAmd64, artifacts.ArchArm64} {
		prof := profile.InstallerProfile(img.secureboot, arch, img.platform)

		enhancement, err := profile.EnhanceFromSchematicWithDependencies(ctx, prof, schematic, f.artifactsManager, f.secureBootService, versionTag)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error enhancing profile from schematic: %w", err)
		}

		prof = enhancement.Profile

		var bootAsset asset.BootAsset

		bootAsset, err = f.assetBuilder.Build(ctx, prof, version.String(), img.Name(), "")
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

		platforms = append(platforms, installer.PlatformArtifact{
			Platform: platform,
			Ref:      installerRepo.Digest(digest.String()),
		})

		for _, dependency := range enhancement.Dependencies {
			dependencies = append(dependencies, resolvedImageDependency(dependency.Kind, dependency.Arch, dependency.Image))
		}
	}

	schematicDigest, err := v1.NewHash("sha256:" + schematicID)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("invalid schematic digest: %w", err)
	}

	dependencies = append(dependencies, installer.ResolvedDependency{
		Name:   "schematic",
		URI:    "urn:siderolabs:image-factory:schematic:" + schematicID,
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
		IndexRef:     indexRef,
		Platforms:    platforms,
		ImageName:    img.Name(),
		SchematicID:  schematicID,
		TalosVersion: versionTag,
		SecureBoot:   img.secureboot,
		Platform:     img.platform,
		InvocationID: invocationID,
		StartedOn:    startedOn,
		FinishedOn:   time.Now().UTC(),
		BuilderVersion: map[string]string{
			"tag": buildversion.Tag,
			"sha": buildversion.SHA,
		},
		ResolvedDependencies: dependencies,
	}

	f.reqLogger(ctx).Info("publishing installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag), zap.Stringer("digest", indexDigest))

	evidencePublisher := f.evidencePublisher
	if !InstallerEvidenceSupported(version) {
		evidencePublisher = nil
	}

	if err = PublishInstallerIndex(
		ctx,
		imageIndex,
		indexRef,
		installerRepo.Tag(versionTag),
		evidenceInput,
		f.pusher,
		f.puller,
		f.imageSigner,
		evidencePublisher,
	); err != nil {
		return v1.Hash{}, err
	}

	return indexDigest, nil
}

func resolvedImageDependency(kind string, arch artifacts.Arch, dependency artifacts.ImageDependency) installer.ResolvedDependency {
	return installer.ResolvedDependency{
		Name:      fmt.Sprintf("%s:%s:linux/%s", kind, dependency.Name, arch),
		URI:       dependency.Ref.String(),
		Digest:    map[string]string{dependency.Descriptor.Digest.Algorithm: dependency.Descriptor.Digest.Hex},
		MediaType: string(dependency.Descriptor.MediaType),
	}
}

// PublishInstallerIndex stages an immutable index and promotes its tag only after all evidence verifies.
func PublishInstallerIndex(
	ctx context.Context,
	imageIndex v1.ImageIndex,
	indexRef name.Digest,
	finalTag name.Tag,
	evidenceInput installer.EvidenceInput,
	pusher remotewrap.Pusher,
	puller remotewrap.Puller,
	imageSigner signer.Signer,
	evidencePublisher enterprise.InstallerEvidencePublisher,
) error {
	// Keep the user-facing tag absent while the evidence graph is assembled.
	if err := pusher.Push(ctx, indexRef, imageIndex); err != nil {
		return fmt.Errorf("failed to stage Installer image index: %w", err)
	}

	if evidencePublisher != nil {
		if err := evidencePublisher.Publish(ctx, evidenceInput); err != nil {
			return fmt.Errorf("failed to publish Installer evidence: %w", err)
		}

		if err := verifyPublished(ctx, "Installer evidence", func(ctx context.Context) error {
			return evidencePublisher.Verify(ctx, evidenceInput)
		}); err != nil {
			return fmt.Errorf("failed to verify Installer evidence: %w", err)
		}
	}

	if err := imageSigner.SignImage(ctx, indexRef, pusher); err != nil {
		return fmt.Errorf("failed to sign Installer image index: %w", err)
	}

	if err := verifyPublished(ctx, "Installer image index signature", func(ctx context.Context) error {
		return imageSigner.VerifyImage(ctx, indexRef, puller)
	}); err != nil {
		return fmt.Errorf("failed to verify Installer image index signature: %w", err)
	}

	if err := pusher.Push(ctx, finalTag, imageIndex); err != nil {
		return fmt.Errorf("failed to promote Installer image index: %w", err)
	}

	return nil
}

func verifyPublished(ctx context.Context, object string, verify retry.RetryableFuncWithContext) error {
	err := retry.Exponential(
		publishedVerificationTimeout,
		retry.WithUnits(250*time.Millisecond),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		verifyErr := verify(ctx)
		if verifyErr != nil {
			ctxlog.Logger(ctx, zap.L()).Warn(
				"read-after-write verification failed",
				zap.String("object", object),
				zap.Error(verifyErr),
			)

			return retry.ExpectedError(verifyErr)
		}

		return nil
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}

// handleCosignSigningKeyPub returns cosign public key in PEM format.
func (f *Frontend) handleCosignSigningKeyPub(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.WriteHeader(http.StatusOK)

	_, err := w.Write(f.imageSigner.GetPublicKeyPEM())

	return err
}
