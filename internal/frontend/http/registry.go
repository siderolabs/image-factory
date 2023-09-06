// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/blang/semver/v4"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/validate"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"

	"github.com/siderolabs/image-service/internal/artifacts"
	"github.com/siderolabs/image-service/internal/asset"
	"github.com/siderolabs/image-service/internal/profile"
	"github.com/siderolabs/image-service/pkg/configuration"
)

// handleHealth handles registry health.
func (f *Frontend) handleHealth(_ context.Context, _ http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	// always healthy, yay!
	return nil
}

type requestedImage struct {
	secureboot bool
}

func getRequestedImage(p httprouter.Params) (requestedImage, error) {
	image := p.ByName("image")

	switch image {
	case "installer":
		return requestedImage{secureboot: false}, nil
	case "installer-secureboot":
		return requestedImage{secureboot: true}, nil
	default:
		return requestedImage{}, fmt.Errorf("invalid image: %s", image)
	}
}

func (img requestedImage) Name() string {
	if img.secureboot {
		return "installer-secureboot"
	}

	return "installer"
}

func (img requestedImage) SecureBoot() bool {
	return img.secureboot
}

// handleBlob handles image blob download.
//
// We always redirect to the external registry, as we assume the image has already been pushed.
func (f *Frontend) handleBlob(ctx context.Context, w http.ResponseWriter, _ *http.Request, p httprouter.Params) error {
	// verify that configuration exists
	configurationID := p.ByName("configuration")

	_, err := f.configService.Get(ctx, configurationID)
	if err != nil {
		return err
	}

	img, err := getRequestedImage(p)
	if err != nil {
		return err
	}

	digest := p.ByName("digest")

	var redirectURL url.URL

	redirectURL.Scheme = f.options.InstallerExternalRepository.Scheme()
	redirectURL.Host = f.options.InstallerExternalRepository.Registry.Name()

	location := redirectURL.JoinPath("v2", f.options.InstallerExternalRepository.RepositoryStr(), img.Name(), configurationID, "blobs", digest).String()

	f.logger.Info("redirecting blob", zap.String("location", location))

	w.Header().Add("Location", location)
	w.WriteHeader(http.StatusTemporaryRedirect)

	return nil
}

// handleManifest handles image manifest download.
//
// If the manifest is for the tag, we check if the image already exists, and either redirect, or build, push and redirect.
func (f *Frontend) handleManifest(ctx context.Context, w http.ResponseWriter, _ *http.Request, p httprouter.Params) error {
	configurationID := p.ByName("configuration")

	configuration, err := f.configService.Get(ctx, configurationID)
	if err != nil {
		return err
	}

	versionTag := p.ByName("tag")

	img, err := getRequestedImage(p)
	if err != nil {
		return err
	}

	redirect := func() error {
		var redirectURL url.URL

		redirectURL.Scheme = f.options.InstallerExternalRepository.Scheme()
		redirectURL.Host = f.options.InstallerExternalRepository.Registry.Name()

		location := redirectURL.JoinPath("v2", f.options.InstallerExternalRepository.RepositoryStr(), img.Name(), configurationID, "manifests", versionTag).String()

		f.logger.Info("redirecting manifest", zap.String("location", location))

		w.Header().Add("Location", location)
		w.WriteHeader(http.StatusTemporaryRedirect)

		return nil
	}

	// if the tag is the digest, we just redirect to the external registry
	if strings.HasPrefix(versionTag, "sha256:") {
		return redirect()
	}

	if !strings.HasPrefix(versionTag, "v") {
		versionTag = "v" + versionTag
	}

	// check if the asset has already been built
	f.logger.Info("heading installer image", zap.String("image", img.Name()), zap.String("configuration", configurationID), zap.String("version", versionTag))

	_, err = f.puller.Head(
		ctx,
		f.options.InstallerInternalRepository.Repo(
			f.options.InstallerInternalRepository.RepositoryStr(),
			img.Name(),
			configurationID,
		).Tag(versionTag),
	)
	if err == nil {
		// the asset has already been built, redirect to the external registry
		return redirect()
	}

	var transportError *transport.Error

	if !errors.As(err, &transportError) || transportError.StatusCode != http.StatusNotFound {
		// something is wrong
		return err
	}

	// installer image is not built yet, build it and push it
	version, err := semver.Parse(versionTag[1:])
	if err != nil {
		return fmt.Errorf("error parsing version: %w", err)
	}

	// build installer images for each architecture, combine them into a single index and push it
	key := fmt.Sprintf("%s-%s-%s", img.Name(), configurationID, versionTag)

	resultCh := f.sf.DoChan(key, func() (any, error) {
		// we use here detached context to make sure image is built no matter if the request is canceled
		return nil, f.buildInstallImage(context.Background(), img, configuration, version, configurationID, versionTag)
	})

	select {
	case res := <-resultCh:
		if res.Err != nil {
			return res.Err
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// now we can redirect to the external registry
	return redirect()
}

func (f *Frontend) buildInstallImage(ctx context.Context, img requestedImage, configuration *configuration.Configuration, version semver.Version, configurationID, versionTag string) error {
	f.logger.Info("building installer image", zap.String("image", img.Name()), zap.String("configuration", configurationID), zap.String("version", versionTag))

	var imageIndex v1.ImageIndex = empty.Index

	for _, arch := range []artifacts.Arch{artifacts.ArchAmd64, artifacts.ArchArm64} {
		prof := profile.InstallerProfile(img.SecureBoot(), arch)

		prof, err := profile.EnhanceFromConfiguration(prof, configuration, versionTag)
		if err != nil {
			return fmt.Errorf("error enhancing profile from configuration: %w", err)
		}

		if err = prof.Validate(); err != nil {
			return fmt.Errorf("error validating profile: %w", err)
		}

		var asset asset.BootAsset

		asset, err = f.assetBuilder.Build(ctx, prof, version.String())
		if err != nil {
			return err
		}

		defer asset.Release() //nolint:errcheck

		var archImage v1.Image

		archImage, err = tarball.Image(asset.Reader, nil)
		if err != nil {
			return fmt.Errorf("error creating image from asset: %w", err)
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

	if err := validate.Index(imageIndex); err != nil {
		return fmt.Errorf("error validating index: %w", err)
	}

	f.logger.Info("pushing installer image", zap.String("image", img.Name()), zap.String("configuration", configurationID), zap.String("version", versionTag))

	if err := f.pusher.Push(
		ctx,
		f.options.InstallerInternalRepository.Repo(
			f.options.InstallerInternalRepository.RepositoryStr(),
			img.Name(),
			configurationID,
		).Tag(versionTag),
		imageIndex,
	); err != nil {
		return fmt.Errorf("error pushing index: %w", err)
	}

	return nil
}
