// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cmd implements the entrypoint of the image service.
package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/blang/semver/v4"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-service/internal/artifacts"
	"github.com/siderolabs/image-service/internal/asset"
	"github.com/siderolabs/image-service/internal/configuration"
	"github.com/siderolabs/image-service/internal/configuration/storage/inmem"
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

	configService, err := buildConfigService(logger, opts)
	if err != nil {
		return err
	}

	assetBuilder := asset.NewBuilder(logger, artifactsManager, opts.AssetBuildMaxConcurrency)

	frontendHTTP := frontendhttp.NewFrontend(logger, configService, assetBuilder)

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

func buildConfigService(logger *zap.Logger, opts Options) (*configuration.Service, error) {
	// as an interim solution, use in-memory storage
	var storage inmem.Storage

	if opts.ConfigKeyBase64 == "" {
		return nil, fmt.Errorf("config key is required")
	}

	key, err := base64.StdEncoding.DecodeString(opts.ConfigKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config key: %w", err)
	}

	return configuration.NewService(logger, &storage, configuration.Options{
		Key: key,
	}), nil
}
