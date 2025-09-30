// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cmd implements the entrypoint of the image factory.
package cmd

import (
	"context"
	"crypto"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	cryptotls "github.com/siderolabs/crypto/tls"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/signature"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	sigstoresignature "github.com/sigstore/sigstore/pkg/signature"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/asset"
	assetcache "github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/asset/cache/cdn"
	assetcachereg "github.com/siderolabs/image-factory/internal/asset/cache/registry"
	assetcaches3 "github.com/siderolabs/image-factory/internal/asset/cache/s3"
	frontendhttp "github.com/siderolabs/image-factory/internal/frontend/http"
	"github.com/siderolabs/image-factory/internal/remotewrap"
	"github.com/siderolabs/image-factory/internal/schematic"
	schematiccache "github.com/siderolabs/image-factory/internal/schematic/storage/cache"
	schematicreg "github.com/siderolabs/image-factory/internal/schematic/storage/registry"
	"github.com/siderolabs/image-factory/internal/secureboot"
	"github.com/siderolabs/image-factory/internal/version"
)

// RunFactory runs the image factory with specified options.
//
//nolint:gocyclo,cyclop
func RunFactory(ctx context.Context, logger *zap.Logger, opts Options) error {
	logger.Info("starting", zap.String("name", version.Name), zap.String("version", version.Tag), zap.String("sha", version.SHA))
	defer logger.Info("shutting down", zap.String("name", version.Name))

	// many image generation steps rely on SOURCE_DATE_EPOCH
	// to ensure reproducibility, set it to a fixed value
	if err := os.Setenv("SOURCE_DATE_EPOCH", "1559424892"); err != nil { // this value matches `pkgs` SOURCE_DATE_EPOCH
		return err
	}

	defer remotewrap.ShutdownTransport()

	artifactsManager, err := buildArtifactsManager(ctx, logger, opts)
	if err != nil {
		return err
	}

	defer artifactsManager.Close() //nolint:errcheck

	configFactory, err := buildSchematicFactory(logger, opts)
	if err != nil {
		return err
	}

	cacheSigningKey, err := loadPrivateKey(opts.CacheSigningKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load cache signing key: %w", err)
	}

	assetBuilder, err := buildAssetBuilder(logger, artifactsManager, cacheSigningKey, opts)
	if err != nil {
		return err
	}

	secureBootService, err := secureboot.NewService(secureboot.Options(opts.SecureBoot))
	if err != nil {
		return fmt.Errorf("failed to initialize SecureBoot service: %w", err)
	}

	var frontendOptions frontendhttp.Options

	frontendOptions.CacheSigningKey = cacheSigningKey

	frontendOptions.ExternalURL, err = url.Parse(opts.ExternalURL)
	if err != nil {
		return fmt.Errorf("failed to parse self URL: %w", err)
	}

	if opts.ExternalPXEURL != "" {
		frontendOptions.ExternalPXEURL, err = url.Parse(opts.ExternalPXEURL)
		if err != nil {
			return fmt.Errorf("failed to parse self PXE URL: %w", err)
		}
	} else {
		frontendOptions.ExternalPXEURL = frontendOptions.ExternalURL
	}

	var repoOpts []name.Option

	if opts.InsecureInstallerInternalRepository {
		repoOpts = append(repoOpts, name.Insecure)
	}

	frontendOptions.InstallerInternalRepository, err = name.NewRepository(opts.InstallerInternalRepository, repoOpts...)
	if err != nil {
		return fmt.Errorf("failed to parse internal installer repository: %w", err)
	}

	if opts.InstallerExternalRepository == "" {
		frontendOptions.ProxyInstallerInternalRepository = true
	} else {
		frontendOptions.InstallerExternalRepository, err = name.NewRepository(opts.InstallerExternalRepository)
		if err != nil {
			return fmt.Errorf("failed to parse external installer repository: %w", err)
		}
	}

	frontendOptions.RemoteOptions = append(frontendOptions.RemoteOptions, remoteOptions()...)
	frontendOptions.RegistryRefreshInterval = opts.RegistryRefreshInterval
	frontendOptions.MetricsNamespace = opts.MetricsNamespace

	frontendHTTP, err := frontendhttp.NewFrontend(logger, configFactory, assetBuilder, artifactsManager, secureBootService, frontendOptions)
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP frontend: %w", err)
	}

	httpServer := &http.Server{
		Addr:    opts.HTTPListenAddr,
		Handler: frontendHTTP.Handler(),
	}

	httpServer.Handler = frontendHTTP.Handler()

	insecure := opts.HTTPCertFile == "" && opts.HTTPKeyFile == ""

	eg, ctx := errgroup.WithContext(ctx)

	if !insecure {
		certLoader := cryptotls.NewDynamicCertificate(opts.HTTPCertFile, opts.HTTPKeyFile)
		if err = certLoader.Load(); err != nil {
			return fmt.Errorf("failed to load certificate: %w", err)
		}

		eg.Go(func() error {
			return certLoader.WatchWithRestarts(ctx, logger)
		})

		httpServer.TLSConfig = &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: certLoader.GetCertificate,
		}
	}

	eg.Go(func() error {
		var err error

		if insecure {
			err = httpServer.ListenAndServe()
		} else {
			err = httpServer.ListenAndServeTLS("", "")
		}

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

	if opts.MetricsListenAddr != "" {
		runMetricsServer(ctx, logger, eg, opts)
	}

	return eg.Wait()
}

func runMetricsServer(ctx context.Context, logger *zap.Logger, eg *errgroup.Group, opts Options) {
	var metricsMux http.ServeMux

	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    opts.MetricsListenAddr,
		Handler: &metricsMux,
	}

	eg.Go(func() error {
		logger.Info("serving metrics", zap.String("listen_addr", opts.MetricsListenAddr))

		err := metricsServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		return err
	})

	eg.Go(func() error {
		<-ctx.Done()

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		return metricsServer.Shutdown(shutdownCtx) //nolint:contextcheck
	})
}

func buildArtifactsManager(ctx context.Context, logger *zap.Logger, opts Options) (*artifacts.Manager, error) {
	var checkOpts []cosign.CheckOpts

	if opts.ContainerSignatureDisabled {
		logger.Warn("container signature verification is disabled, this is not recommended")
	} else {
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

		if len(strings.TrimSpace(opts.ContainerSignaturePublicKeyFile)) > 0 {
			var keyVerifier sigstoresignature.Verifier

			keyVerifier, err = getPublicKeyVerifier(opts)
			if err != nil {
				return nil, fmt.Errorf("failed to get signature verifier for key %s: %w", opts.ContainerSignaturePublicKeyFile, err)
			}

			checkOpts = append(checkOpts, cosign.CheckOpts{
				SigVerifier: keyVerifier,
				Offline:     true,
				IgnoreTlog:  true,
			})
		}

		// Prefer opts.ContainerSignatureIssuerRegExp if set as this is more flexible
		cosignIdentities := []cosign.Identity{
			{
				SubjectRegExp: opts.ContainerSignatureSubjectRegExp,
			},
		}

		if len(strings.TrimSpace(opts.ContainerSignatureIssuerRegExp)) > 0 {
			cosignIdentities[0].IssuerRegExp = opts.ContainerSignatureIssuerRegExp
		} else {
			cosignIdentities[0].Issuer = opts.ContainerSignatureIssuer
		}

		checkOpts = append(checkOpts, cosign.CheckOpts{
			RootCerts:         rootCerts,
			IntermediateCerts: intermediateCerts,
			RekorPubKeys:      rekorPubKeys,
			CTLogPubKeys:      ctLogPubKeys,
			Identities:        cosignIdentities,
		})
	}

	imageVerifyOptions := artifacts.ImageVerifyOptions{
		CheckOpts: checkOpts,
		Disabled:  opts.ContainerSignatureDisabled,
	}

	minVersion, err := semver.Parse(opts.MinTalosVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum Talos version: %w", err)
	}

	artifactsManager, err := artifacts.NewManager(logger, artifacts.Options{
		MinVersion:                  minVersion,
		ImageRegistry:               opts.ImageRegistry,
		InsecureImageRegistry:       opts.InsecureImageRegistry,
		ImageVerifyOptions:          imageVerifyOptions,
		TalosVersionRecheckInterval: opts.TalosVersionRecheckInterval,
		RemoteOptions:               remoteOptions(),
		RegistryRefreshInterval:     opts.RegistryRefreshInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize artifacts manager: %w", err)
	}

	return artifactsManager, nil
}

func buildAssetBuilder(logger *zap.Logger, artifactsManager *artifacts.Manager, cacheSigningKey crypto.PrivateKey, opts Options) (*asset.Builder, error) {
	var (
		cache assetcache.Cache
		err   error
	)

	regOptions := assetcachereg.Options{
		CacheSigningKey:         cacheSigningKey,
		RegistryRefreshInterval: opts.RegistryRefreshInterval,
	}
	regOptions.RemoteOptions = append(regOptions.RemoteOptions, remoteOptions()...)

	var repoOpts []name.Option

	if opts.InsecureCacheRepository {
		repoOpts = append(repoOpts, name.Insecure)
	}

	regOptions.CacheRepository, err = name.NewRepository(opts.CacheRepository, repoOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache repository: %w", err)
	}

	cache, err = assetcachereg.New(logger, regOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository cache: %w", err)
	}

	if opts.CacheS3Enabled {
		s3Options := assetcaches3.Options{
			Bucket:   opts.CacheS3Bucket,
			Endpoint: opts.CacheS3Endpoint,
			Region:   opts.CacheS3Region,
			Insecure: opts.InsecureCacheS3,
		}

		cache, err = assetcaches3.New(logger, cache, s3Options)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize s3 cache: %w", err)
		}
	}

	if opts.CacheCDNEnabled {
		cdnOptions := cdn.Options{
			Host:       opts.CacheCDNHost,
			TrimPrefix: opts.CacheCDNTrimPrefix,
		}

		cache, err = cdn.New(logger, cache, cdnOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize CDN cache: %w", err)
		}
	}

	builderOptions := asset.Options{
		MetricsNamespace:   opts.MetricsNamespace,
		AllowedConcurrency: opts.AssetBuildMaxConcurrency,
		GetAfterPut:        opts.CacheS3Enabled,
	}

	builder, err := asset.NewBuilder(logger, artifactsManager, cache, builderOptions)
	if err != nil {
		return nil, err
	}

	prometheus.MustRegister(builder)

	return builder, nil
}

func buildSchematicFactory(logger *zap.Logger, opts Options) (*schematic.Factory, error) {
	var repoOpts []name.Option

	if opts.InsecureSchematicRepository {
		repoOpts = append(repoOpts, name.Insecure)
	}

	repo, err := name.NewRepository(opts.SchematicServiceRepository, repoOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	storage, err := schematicreg.NewStorage(repo, opts.RegistryRefreshInterval, remoteOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize registry storage: %w", err)
	}

	c := schematiccache.NewCache(storage, schematiccache.Options{MetricsNamespace: opts.MetricsNamespace})

	factory := schematic.NewFactory(logger, c, schematic.Options{MetricsNamespace: opts.MetricsNamespace})

	prometheus.MustRegister(factory)

	return factory, nil
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
				google.Keychain,
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

// Basically taken from https://github.com/sigstore/cosign/blob/main/cmd/cosign/cli/options/signature_digest.go
func getHashAlgo(algo string) (crypto.Hash, error) {
	supportedSignatureAlgorithms := map[string]crypto.Hash{
		"sha224": crypto.SHA224,
		"sha256": crypto.SHA256,
		"sha384": crypto.SHA384,
		"sha512": crypto.SHA512,
	}
	normalizedAlgo := strings.ToLower(strings.TrimSpace(algo))

	if normalizedAlgo == "" {
		return crypto.SHA256, nil
	}

	ralgo, exists := supportedSignatureAlgorithms[normalizedAlgo]
	if !exists {
		return crypto.SHA256, fmt.Errorf("unknown digest algorithm: %s", algo)
	}

	if !ralgo.Available() {
		return crypto.SHA256, fmt.Errorf("hash %q is not available on this platform", algo)
	}

	return ralgo, nil
}

func getPublicKeyVerifier(opts Options) (sigstoresignature.Verifier, error) {
	hashAlgo, err := getHashAlgo(opts.ContainerSignaturePublicKeyHashAlgo)
	if err != nil {
		return nil, err
	}

	key, err := os.ReadFile(opts.ContainerSignaturePublicKeyFile)
	if err != nil {
		return nil, err
	}

	return signature.LoadPublicKeyRaw(key, hashAlgo)
}
