// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cmd implements the entrypoint of the image service.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-service/internal/artifacts"
	"github.com/siderolabs/image-service/internal/asset"
	"github.com/siderolabs/image-service/internal/flavor"
	"github.com/siderolabs/image-service/internal/flavor/storage/cache"
	"github.com/siderolabs/image-service/internal/flavor/storage/registry"
	frontendhttp "github.com/siderolabs/image-service/internal/frontend/http"
	"github.com/siderolabs/image-service/internal/version"
)

// RunService runs the image service with specified options.
func RunService(ctx context.Context, logger *zap.Logger, opts Options) error {
	logger.Info("starting", zap.String("name", version.Name), zap.String("version", version.Tag), zap.String("sha", version.SHA))
	defer logger.Info("shutting down", zap.String("name", version.Name))

	artifactsManager, err := buildArtifactsManager(ctx, logger, opts)
	if err != nil {
		return err
	}

	defer artifactsManager.Close() //nolint:errcheck

	configService, err := buildFlavorService(logger, opts)
	if err != nil {
		return err
	}

	assetBuilder := asset.NewBuilder(logger, artifactsManager, opts.AssetBuildMaxConcurrency)

	var frontendOptions frontendhttp.Options

	frontendOptions.ExternalURL, err = url.Parse(opts.ExternalURL)
	if err != nil {
		return fmt.Errorf("failed to parse self URL: %w", err)
	}

	frontendOptions.InstallerInternalRepository, err = name.NewRepository(opts.InstallerInternalRepository)
	if err != nil {
		return fmt.Errorf("failed to parse internal installer repository: %w", err)
	}

	frontendOptions.InstallerExternalRepository, err = name.NewRepository(opts.InstallerExternalRepository)
	if err != nil {
		return fmt.Errorf("failed to parse external installer repository: %w", err)
	}

	frontendHTTP, err := frontendhttp.NewFrontend(logger, configService, assetBuilder, frontendOptions)
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP frontend: %w", err)
	}

	httpServer := &http.Server{
		Addr:    opts.HTTPListenAddr,
		Handler: frontendHTTP.Handler(),
	}

	httpServer.Handler = frontendHTTP.Handler()

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		err := httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		return err
	})

	eg.Go(func() error {
		<-ctx.Done()

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		return httpServer.Shutdown(shutdownCtx) //nolint:contextcheck
	})

	return eg.Wait()
}

func buildArtifactsManager(ctx context.Context, logger *zap.Logger, opts Options) (*artifacts.Manager, error) {
	rootCerts, err := fulcio.GetRoots()
	if err != nil {
		return nil, fmt.Errorf("getting Fulcio roots: %w", err)
	}

	intermediateCerts, err := fulcio.GetIntermediates()
	if err != nil {
		return nil, fmt.Errorf("getting Fulcio intermediates: %w", err)
	}

	rekorPubKeys, err := cosign.GetRekorPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting rekor public keys: %w", err)
	}

	ctLogPubKeys, err := cosign.GetCTLogPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("error ctlog public keys: %w", err)
	}

	minVersion, err := semver.Parse(opts.MinTalosVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum Talos version: %w", err)
	}

	artifactsManager, err := artifacts.NewManager(logger, artifacts.Options{
		MinVersion:  minVersion,
		ImagePrefix: opts.ImagePrefix,
		ImageVerifyOptions: cosign.CheckOpts{
			Identities: []cosign.Identity{
				{
					SubjectRegExp: opts.ContainerSignatureSubjectRegExp,
					Issuer:        opts.ContainerSignatureIssuer,
				},
			},
			RootCerts:         rootCerts,
			IntermediateCerts: intermediateCerts,
			RekorPubKeys:      rekorPubKeys,
			CTLogPubKeys:      ctLogPubKeys,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize artifacts manager: %w", err)
	}

	return artifactsManager, nil
}

func buildFlavorService(logger *zap.Logger, opts Options) (*flavor.Service, error) {
	repo, err := name.NewRepository(opts.FlavorServiceRepository)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	storage, err := registry.NewStorage(repo, remoteOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return flavor.NewService(logger, cache.NewCache(storage), flavor.Options{}), nil
}

// remoteOptions returns options for remote registry access.
//
// Enable registry auth from the standard Docker config, and from GitHub via the token.
func remoteOptions() []remote.Option {
	return []remote.Option{
		remote.WithAuthFromKeychain(
			authn.NewMultiKeychain(
				authn.DefaultKeychain,
				github.Keychain,
			),
		),
	}
}
