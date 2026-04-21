// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build enterprise

package enterprise

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/enterprise/auth"
	"github.com/siderolabs/image-factory/enterprise/checksum"
	"github.com/siderolabs/image-factory/enterprise/spdx"
	"github.com/siderolabs/image-factory/enterprise/spdx/builder"
	"github.com/siderolabs/image-factory/enterprise/spdx/storage/registry"
)

// Enabled indicates whether Enterprise features are enabled.
func Enabled() bool {
	return true
}

// NewSpdxFrontend returns a new Spdx FrontendPlugin.
func NewSpdxFrontend(logger *zap.Logger, opts SPDXOptions) (FrontendPlugin, error) {
	var repoOpts []name.Option

	if opts.CacheInsecure {
		repoOpts = append(repoOpts, name.Insecure)
	}

	cacheRepository, err := name.NewRepository(opts.CacheRepository, repoOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache repository: %w", err)
	}

	storage, err := registry.NewStorage(logger, registry.Options{
		CacheRepository:         cacheRepository,
		CacheImageSigner:        opts.CacheImageSigner,
		RemoteOptions:           opts.RemoteOptions,
		RegistryRefreshInterval: opts.RegistryRefreshInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SPDX storage: %w", err)
	}

	builder := builder.NewBuilder(logger, builder.Options{
		Storage:          storage,
		ArtifactsManager: opts.ArtifactsManager,
		SchematicFactory: opts.SchematicFactory,
		AssetBuilder:     opts.AssetBuilder,
		AuthProvider:     opts.AuthProvider,
	})

	return spdx.NewFrontend(opts.SchematicFactory, builder, opts.AuthProvider), nil
}

// NewChecksummer returns an enterprise Checksummer implementation.
func NewChecksummer() Checksummer {
	return checksum.NewChecksummer()
}

// NewAuthProvider creates a new authentication provider.
func NewAuthProvider(logger *zap.Logger, configPath string) (AuthProvider, error) {
	return auth.NewProvider(configPath, logger)
}
