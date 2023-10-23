// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cmd implements the entrypoint of the image factory.
package cmd

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	frontendhttp "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/internal/schematic/storage/cache"
	"github.com/siderolabs/image-factory/internal/schematic/storage/registry"
	"github.com/siderolabs/image-factory/internal/version"
)

// RunFactory runs the image factory with specified options.
func RunFactory(ctx context.Context, logger *zap.Logger, opts Options) error {
	logger.Info("starting", zap.String("name", version.Name), zap.String("version", version.Tag), zap.String("sha", version.SHA))
	defer logger.Info("shutting down", zap.String("name", version.Name))

	artifactsManager, err := buildArtifactsManager(ctx, logger, opts)
	if err != nil {
		return err
	}

	defer artifactsManager.Close() //nolint:errcheck

	configFactory, err := buildSchematicFactory(logger, opts)
	if err != nil {
		return err
	}

	assetBuilder := asset.NewBuilder(logger, artifactsManager, opts.AssetBuildMaxConcurrency)

	var frontendOptions frontendhttp.Options

	cacheSigningKey, err := loadPrivateKey(opts.CacheSigningKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load cache signing key: %w", err)
	}

	frontendOptions.CacheSigningKey = cacheSigningKey

	frontendOptions.ExternalURL, err = url.Parse(opts.ExternalURL)
	if err != nil {
		return fmt.Errorf("failed to parse self URL: %w", err)
	}

	repoOpts := []name.Option{}
	if opts.InsecureInstallerInternalRepository {
		repoOpts = append(repoOpts, name.Insecure)
	}

	frontendOptions.InstallerInternalRepository, err = name.NewRepository(opts.InstallerInternalRepository, repoOpts...)
	if err != nil {
		return fmt.Errorf("failed to parse internal installer repository: %w", err)
	}

	frontendOptions.InstallerExternalRepository, err = name.NewRepository(opts.InstallerExternalRepository)
	if err != nil {
		return fmt.Errorf("failed to parse external installer repository: %w", err)
	}

	frontendOptions.RemoteOptions = append(frontendOptions.RemoteOptions, remoteOptions()...)

	frontendHTTP, err := frontendhttp.NewFrontend(logger, configFactory, assetBuilder, artifactsManager, frontendOptions)
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
		MinVersion:            minVersion,
		ImageRegistry:         opts.ImageRegistry,
		InsecureImageRegistry: opts.InsecureImageRegistry,
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
		TalosVersionRecheckInterval: opts.TalosVersionRecheckInterval,
		RemoteOptions:               remoteOptions(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize artifacts manager: %w", err)
	}

	return artifactsManager, nil
}

func buildSchematicFactory(logger *zap.Logger, opts Options) (*schematic.Factory, error) {
	repo, err := name.NewRepository(opts.SchematicServiceRepository)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	storage, err := registry.NewStorage(repo, remoteOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return schematic.NewFactory(logger, cache.NewCache(storage), schematic.Options{}), nil
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

func loadPrivateKey(keyPath string) (crypto.PrivateKey, error) {
	fileBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	return cryptoutils.UnmarshalPEMToPrivateKey(fileBytes, cryptoutils.SkipPassword)
}
