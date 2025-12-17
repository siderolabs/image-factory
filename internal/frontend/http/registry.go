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
	"strings"

	"github.com/blang/semver/v4"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/regtransport"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// InvalidImageTag is an error tag for invalid image names.
type InvalidImageTag struct{}

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

func getRequestedImage(p httprouter.Params) (requestedImage, error) {
	image := p.ByName("image")

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
func (f *Frontend) handleBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, p httprouter.Params) error {
	// verify that schematic exists
	schematicID := p.ByName("schematic")

	_, err := f.schematicFactory.Get(ctx, schematicID)
	if err != nil {
		return err
	}

	img, err := getRequestedImage(p)
	if err != nil {
		return err
	}

	digest := p.ByName("digest")

	return f.handleExternalRegistry(w, req, img.Name(), schematicID, "blobs", digest)
}

func (f *Frontend) handleExternalRegistry(w http.ResponseWriter, req *http.Request, imageName, schematicID, manifestsOrBlobs, tagOrDigest string) error {
	var redirectURL url.URL

	repo := f.options.InstallerExternalRepository
	if f.options.ProxyInstallerInternalRepository {
		repo = f.options.InstallerInternalRepository
	}

	redirectURL.Scheme = repo.Scheme()
	redirectURL.Host = repo.Registry.Name()
	redirectURL.Path = "/"

	location := redirectURL.JoinPath("v2", repo.RepositoryStr(), imageName, schematicID, manifestsOrBlobs, tagOrDigest)

	if f.options.ProxyInstallerInternalRepository {
		f.logger.Info("proxying manifest/blob", zap.Stringer("location", location))

		proxy := &httputil.ReverseProxy{
			Director: func(out *http.Request) {
				out.URL.Scheme = location.Scheme
				out.URL.Host = location.Host
				out.URL.Path = location.Path
				out.URL.RawPath = ""
				out.URL.RawQuery = location.RawQuery
				// we don't forward the host header to avoid TLS issues with some registries
				out.Host = ""
			},
			Transport: remotewrap.GetTransport(),
		}

		proxy.ServeHTTP(w, req)

		return nil
	}

	f.logger.Info("redirecting manifest/blob", zap.Stringer("location", location))

	w.Header().Add("Location", location.String())
	w.WriteHeader(http.StatusTemporaryRedirect)

	return nil
}

// handleManifest handles image manifest download.
//
// If the manifest is for the tag, we check if the image already exists, and either redirect, or build, push and redirect.
func (f *Frontend) handleManifest(ctx context.Context, w http.ResponseWriter, req *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

	schematic, err := f.schematicFactory.Get(ctx, schematicID)
	if err != nil {
		return err
	}

	versionTag := p.ByName("tag")

	img, err := getRequestedImage(p)
	if err != nil {
		return err
	}

	// if the tag is the digest, or it doesn't look like the version, we just redirect to the external registry
	if strings.HasPrefix(versionTag, "sha256:") || !strings.HasPrefix(versionTag, "v") {
		return f.handleExternalRegistry(w, req, img.Name(), schematicID, "manifests", versionTag)
	}

	imageRepository := f.options.InstallerInternalRepository.Repo(
		f.options.InstallerInternalRepository.RepositoryStr(),
		img.Name(),
		schematicID,
	)

	// check if the asset has already been built
	f.logger.Info("heading installer image",
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
		// the asset has already been built, so check the signature
		f.logger.Info("verifying cached installer image signature",
			zap.String("image", img.Name()),
			zap.String("schematic", schematicID),
			zap.String("version", versionTag),
			zap.Stringer("ref", imageRepository.Digest(extDesc.Digest.String())),
		)

		_, _, signatureErr := cosign.VerifyImageSignatures(
			ctx,
			imageRepository.Digest(extDesc.Digest.String()),
			f.imageSigner.GetCheckOpts(),
		)
		if signatureErr == nil {
			// redirect to the external registry, but use the digest directly to avoid tag changes
			return f.handleExternalRegistry(w, req, img.Name(), schematicID, "manifests", extDesc.Digest.String())
		}

		// log the signature verification error, but continue to build the image
		f.logger.Error("error verifying cached image signature", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag), zap.Error(signatureErr))
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

	resultCh := f.sf.DoChan(key, func() (any, error) { //nolint:contextcheck
		// we use here detached context to make sure image is built no matter if the request is canceled
		return f.buildInstallImage(context.Background(), img, schematic, version, schematicID, versionTag)
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
	return f.handleExternalRegistry(w, req, img.Name(), schematicID, "manifests", manifestHash.String())
}

func (f *Frontend) buildInstallImage(ctx context.Context, img requestedImage, schematic *schematic.Schematic, version semver.Version, schematicID, versionTag string) (v1.Hash, error) {
	f.logger.Info("building installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag))

	var imageIndex v1.ImageIndex = empty.Index

	for _, arch := range []artifacts.Arch{artifacts.ArchAmd64, artifacts.ArchArm64} {
		prof := profile.InstallerProfile(img.secureboot, arch, img.platform)

		prof, err := profile.EnhanceFromSchematic(ctx, prof, schematic, f.artifactsManager, f.secureBootService, versionTag)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error enhancing profile from schematic: %w", err)
		}

		var asset asset.BootAsset

		asset, err = f.assetBuilder.Build(ctx, prof, version.String(), img.Name(), "")
		if err != nil {
			return v1.Hash{}, err
		}

		var archImage v1.Image

		archImage, err = tarball.Image(asset.Reader, nil)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("error creating image from asset: %w", err)
		}

		imageIndex = mutate.AppendManifests(imageIndex,
			mutate.IndexAddendum{
				Add: archImage,
				Descriptor: v1.Descriptor{
					Platform: &v1.Platform{
						Architecture: prof.Arch,
						OS:           "linux",
					},
				},
			})
	}

	f.logger.Info("pushing installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag))

	installerRepo := f.options.InstallerInternalRepository.Repo(
		f.options.InstallerInternalRepository.RepositoryStr(),
		img.Name(),
		schematicID,
	)

	if err := f.pusher.Push(
		ctx,
		installerRepo.Tag(versionTag),
		imageIndex,
	); err != nil {
		return v1.Hash{}, fmt.Errorf("error pushing index: %w", err)
	}

	digest, err := imageIndex.Digest()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("error getting index digest: %w", err)
	}

	f.logger.Info("signing installer image", zap.String("image", img.Name()), zap.String("schematic", schematicID), zap.String("version", versionTag), zap.Stringer("digest", digest))

	if err := f.imageSigner.SignImage(
		ctx,
		installerRepo.Digest(digest.String()),
		f.pusher,
	); err != nil {
		return v1.Hash{}, fmt.Errorf("error signing image: %w", err)
	}

	return digest, nil
}

// handleCosignSigningKeyPub returns cosign public key in PEM format.
func (f *Frontend) handleCosignSigningKeyPub(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.WriteHeader(http.StatusOK)

	_, err := w.Write(f.imageSigner.GetPublicKeyPEM())

	return err
}
