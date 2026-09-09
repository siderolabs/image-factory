// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"

	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"

	"github.com/siderolabs/image-factory/internal/frontend/http/oci"
	"github.com/siderolabs/image-factory/internal/profile"
	"github.com/siderolabs/image-factory/internal/registry"
	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// InvalidImageTag is an error tag for invalid image names.
type InvalidImageTag = registry.InvalidImageTag

// ProxyUnavailableTag is an error tag for when the backing registry cannot be proxied to.
type ProxyUnavailableTag = registry.ProxyUnavailableTag

// registrySchematics retains the legacy provider-context compatibility at composition.
type registrySchematics struct {
	factory  *schematic.Factory
	provider enterprise.AuthProvider
}

func (source registrySchematics) Get(ctx context.Context, id string) (*schematicpkg.Schematic, error) {
	return source.factory.Get(ctx, id, source.provider)
}

func (f *Frontend) initializeRegistry() {
	service := registry.New(
		registrySchematics{factory: f.schematicFactory, provider: f.options.AuthProvider},
		f.artifactsManager,
		f.assetBuilder,
		func(ctx context.Context, prof talosprofile.Profile, configuration *schematicpkg.Schematic, versionTag string) (profile.EnhancementResult, error) {
			return profile.EnhanceFromSchematicWithDependencies(ctx, prof, configuration, f.artifactsManager, f.secureBootService, versionTag)
		},
		registry.Options{
			InternalRepository: f.options.InstallerInternalRepository,
			ExternalRepository: f.options.InstallerExternalRepository,
			ProxyInternal:      f.options.ProxyInstallerInternalRepository,
			AuthEnabled:        f.options.AuthProvider != nil,
			ProxyImages:        f.options.ImageProxy.Images,
			ProxyRegistry:      f.options.ImageProxy.BackingRegistry,
			ProxyNamespace:     f.options.ImageProxy.Namespace,
			Puller:             f.puller,
			Pusher:             f.pusher,
			Signer:             f.imageSigner,
			EvidencePublisher:  f.evidencePublisher,
			Logger:             f.logger,
		},
	)
	f.oci = oci.NewRegistryHandler(service, f.logger)
}
