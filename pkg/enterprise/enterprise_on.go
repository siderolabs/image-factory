// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package enterprise

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/enterprise/assetsignature"
	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/enterprise/auth/auth0"
	"github.com/siderolabs/image-factory/enterprise/checksum"
	"github.com/siderolabs/image-factory/enterprise/installerattestation"
	"github.com/siderolabs/image-factory/enterprise/scanner"
	scannerbuilder "github.com/siderolabs/image-factory/enterprise/scanner/builder"
	"github.com/siderolabs/image-factory/enterprise/spdx"
	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
	"github.com/siderolabs/image-factory/enterprise/spdx/storage/registry"
	"github.com/siderolabs/image-factory/enterprise/tokens"
	"github.com/siderolabs/image-factory/enterprise/vex"
	vexbuilder "github.com/siderolabs/image-factory/enterprise/vex/builder"
	"github.com/siderolabs/image-factory/internal/apitoken"
	"github.com/siderolabs/image-factory/internal/artifacts"
	assetcache "github.com/siderolabs/image-factory/internal/asset/cache"
	"github.com/siderolabs/image-factory/internal/image/signer"
	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Enabled indicates whether Enterprise features are enabled.
func Enabled() bool {
	return true
}

// NewVEXFrontend returns a new VEX FrontendPlugin and the underlying VEX builder.
//
// The builder is exposed so that downstream enterprise components (e.g., the scanner
// frontend) can reuse the same OCI-backed VEX document source without duplicating
// the OCI fetch and signature verification.
//
// The cache eviction goroutine is started under eg and stopped when ctx is canceled.
func NewVEXFrontend(ctx context.Context, eg *errgroup.Group, logger *zap.Logger, config VEXOptions) (FrontendPlugin, VEXSource, error) {
	b, err := vexbuilder.NewBuilder(logger, vexbuilder.Options{
		Registry:         config.Data,
		Insecure:         config.DataInsecure,
		MetricsNamespace: config.MetricsNamespace,
		RefreshInterval:  config.RefreshInterval,
		RemoteOptions:    config.RemoteOptions,
		VerifyOptions:    config.VerifyOptions,
		CacheTTL:         config.CacheTTL,
		Capacity:         config.CacheCapacity,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating VEX builder: %w", err)
	}

	eg.Go(b.Start)

	eg.Go(func() error {
		<-ctx.Done()

		b.Stop()

		return nil
	})

	prometheus.MustRegister(b)

	return vex.NewFrontend(b), b, nil
}

// NewScannerFrontend returns a new Scanner FrontendPlugin.
//
// The cache eviction goroutine is started under eg and stopped when ctx is canceled,
// mirroring the schematic cache lifecycle.
func NewScannerFrontend(ctx context.Context, eg *errgroup.Group, logger *zap.Logger, opts ScannerOptions) (FrontendPlugin, error) {
	b, err := scannerbuilder.NewBuilder(logger, scannerbuilder.Options{
		VEXSource:        opts.VEXSource,
		SPDXSource:       opts.SPDXSource,
		DatabaseURL:      opts.DatabaseURL,
		DatabaseUpdateAt: opts.DatabaseUpdateAt,
		DatabaseRootDir:  opts.DatabaseRootDir,
		MetricsNamespace: opts.MetricsNamespace,
		CacheTTL:         opts.CacheTTL,
		Capacity:         opts.CacheCapacity,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating scanner builder: %w", err)
	}

	eg.Go(b.Start)

	eg.Go(func() error {
		<-ctx.Done()

		return b.Stop()
	})

	prometheus.MustRegister(b)

	return scanner.NewFrontend(opts.SchematicFactory, b, opts.AuthProvider), nil
}

// BundleBuilder is a helper struct that encapsulates the SPDX builder used by both the SPDX and Scanner frontends.
type BundleBuilder struct {
	*builder.Builder
}

func (b *BundleBuilder) Build(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (io.ReadCloser, error) {
	bundle, err := b.Builder.Build(ctx, schematicID, versionTag, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to build SPDX bundle: %w", err)
	}

	return bundle.Reader()
}

// BuildBytes returns the canonical merged SPDX 2.3 JSON document for an Installer platform.
func (b *BundleBuilder) BuildBytes(ctx context.Context, schematicID, versionTag string, arch artifacts.Arch) (data []byte, err error) {
	reader, err := b.Build(ctx, schematicID, versionTag, arch)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errors.Join(err, reader.Close())
	}()

	data, err = io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read SPDX bundle: %w", err)
	}

	return data, nil
}

// NewSpdxFrontend returns a new SPDX FrontendPlugin and the underlying SPDX builder.
//
// The builder is exposed so that downstream enterprise components (e.g., the scanner
// frontend) can reuse the same SBOM extraction code path for the vanilla Talos image.
func NewSpdxFrontend(logger *zap.Logger, opts SPDXOptions) (FrontendPlugin, SPDXSource, error) {
	var repoOpts []name.Option

	if opts.CacheInsecure {
		repoOpts = append(repoOpts, name.Insecure)
	}

	cacheRepository, err := name.NewRepository(opts.CacheRepository, repoOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse cache repository: %w", err)
	}

	storage, err := registry.NewStorage(logger, registry.Options{
		CacheRepository:         cacheRepository,
		NameOptions:             repoOpts,
		CacheImageSigner:        opts.CacheImageSigner,
		RemoteOptions:           opts.RemoteOptions,
		RegistryRefreshInterval: opts.RegistryRefreshInterval,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize SPDX storage: %w", err)
	}

	spdxBuilder := builder.NewBuilder(logger, builder.Options{
		ExternalURL:      opts.ExternalURL,
		Storage:          storage,
		ArtifactsManager: opts.ArtifactsManager,
		SchematicFactory: opts.SchematicFactory,
		AssetBuilder:     opts.AssetBuilder,
		AuthProvider:     opts.AuthProvider,
	})

	return spdx.NewFrontend(opts.SchematicFactory, spdxBuilder, opts.AuthProvider), &BundleBuilder{spdxBuilder}, nil
}

// NewInstallerEvidencePublisher creates the mandatory Enterprise Installer evidence publisher.
func NewInstallerEvidencePublisher(
	logger *zap.Logger,
	imageSigner signer.Signer,
	spdxSource SPDXSource,
	pusher remotewrap.Pusher,
	puller remotewrap.Puller,
) (InstallerEvidencePublisher, error) {
	attestor, ok := imageSigner.(signer.ImageAttestor)
	if !ok {
		return nil, fmt.Errorf("image signer does not support typed attestations")
	}

	return installerattestation.NewPublisher(logger, installerattestation.Options{
		Attestor:   attestor,
		SBOMSource: spdxSource,
		Pusher:     pusher,
		Puller:     puller,
	})
}

// NewChecksummer returns an enterprise Checksummer implementation.
func NewChecksummer() Checksummer {
	return checksum.NewChecksummer()
}

// NewSignatureWriter returns a detached signature writer when imageSigner supports blob signing.
func NewSignatureWriter(logger *zap.Logger, imageSigner signer.Signer, cache assetcache.Cache) (SignatureWriter, error) {
	blobSigner, ok := imageSigner.(signer.BlobSigner)
	if !ok {
		return nil, fmt.Errorf("image signer does not support blob signing")
	}

	return assetsignature.NewWriter(logger, blobSigner, cache), nil
}

// NewHTPasswdProvider creates a new htpasswd-backed authentication provider.
func NewHTPasswdProvider(logger *zap.Logger, configPath string) (AuthProvider, error) {
	return auth.NewProvider(configPath, logger)
}

// NewAuth0Provider creates a new Auth0 JWT authentication provider.
func NewAuth0Provider(ctx context.Context, logger *zap.Logger, cfg Auth0Config) (AuthProvider, error) {
	return auth0.NewProvider(ctx, logger, auth0.Config{
		Domain:            cfg.Domain,
		Audience:          cfg.Audience,
		MachineScope:      cfg.MachineScope,
		ClientID:          cfg.ClientID,
		ClientSecret:      cfg.ClientSecret,
		ExternalURL:       cfg.ExternalURL,
		SessionKey:        cfg.SessionKey,
		IssuerURLOverride: cfg.IssuerURLOverride,
	})
}

// NewTokenFrontends returns the API-token FrontendPlugins together with a TokenVerifier.
//
// If opts.KeyPath is non-empty the signing key is loaded from the PEM file; otherwise a
// fresh ECDSA P-256 key pair is generated (suitable for single-replica deployments).
func NewTokenFrontends(authProvider AuthProvider, opts TokenOptions) ([]FrontendPlugin, TokenVerifier, error) {
	var (
		issuer *apitoken.Issuer
		err    error
	)

	if opts.KeyPath != "" {
		issuer, err = apitoken.LoadIssuer(opts.KeyPath, opts.TTL, opts.StorageTTL)
	} else {
		issuer, err = apitoken.GenerateIssuer(opts.TTL, opts.StorageTTL)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize token issuer: %w", err)
	}

	var repoOpts []name.Option

	if opts.StorageInsecure {
		repoOpts = append(repoOpts, name.Insecure)
	}

	repo, err := name.NewRepository(opts.StorageRepository, repoOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse token storage repository: %w", err)
	}

	storage, err := tokens.NewStorage(repo, opts.VerificationCacheRefreshInterval, opts.RemoteOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize token storage: %w", err)
	}

	manager := tokens.NewManager(issuer, storage)

	plugins := []FrontendPlugin{
		tokens.NewListCreateFrontend(manager, authProvider, opts.MaxPerOrg),
		tokens.NewRevokeFrontend(manager, authProvider),
		tokens.NewJWKSFrontend(issuer),
	}

	return plugins, manager, nil
}
