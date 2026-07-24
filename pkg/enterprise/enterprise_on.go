// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package enterprise

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/enterprise/checksum"
	enterprisedt "github.com/siderolabs/image-factory/enterprise/downloadtoken"
	"github.com/siderolabs/image-factory/enterprise/scanner"
	scannerbuilder "github.com/siderolabs/image-factory/enterprise/scanner/builder"
	"github.com/siderolabs/image-factory/enterprise/spdx"
	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
	"github.com/siderolabs/image-factory/enterprise/spdx/storage/registry"
	"github.com/siderolabs/image-factory/enterprise/vex"
	vexbuilder "github.com/siderolabs/image-factory/enterprise/vex/builder"
	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/downloadtoken"
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
	b := scannerbuilder.NewBuilder(logger, scannerbuilder.Options{
		VEXSource:        opts.VEXSource,
		SPDXSource:       opts.SPDXSource,
		DatabaseURL:      opts.DatabaseURL,
		MetricsNamespace: opts.MetricsNamespace,
		CacheTTL:         opts.CacheTTL,
		Capacity:         opts.CacheCapacity,
	})

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

// NewChecksummer returns an enterprise Checksummer implementation.
func NewChecksummer() Checksummer {
	return checksum.NewChecksummer()
}

// NewAuthProvider creates a new authentication provider.
func NewAuthProvider(logger *zap.Logger, configPath string) (AuthProvider, error) {
	return auth.NewProvider(configPath, logger)
}

// NewDownloadTokenIssuer creates a new download token issuer.
// If keyPath is non-empty the key is loaded from the PEM file; otherwise a
// fresh ECDSA P-256 key pair is generated (suitable for single-replica deployments).
func NewDownloadTokenIssuer(keyPath string, ttl time.Duration) (DownloadTokenIssuer, error) {
	if keyPath != "" {
		return downloadtoken.LoadIssuer(keyPath, ttl)
	}

	return downloadtoken.GenerateIssuer(ttl)
}

// NewDownloadTokenFrontend returns the FrontendPlugin for download token issuance.
func NewDownloadTokenFrontend(issuer DownloadTokenIssuer, authProvider AuthProvider) FrontendPlugin {
	return enterprisedt.NewFrontend(issuer, authProvider)
}

// NewJWKSFrontend returns the FrontendPlugin for the JWKS public key endpoint.
func NewJWKSFrontend(issuer DownloadTokenIssuer) FrontendPlugin {
	return enterprisedt.NewJWKSFrontend(issuer)
}
