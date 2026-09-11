// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package schematic

import (
	"context"
	"errors"

	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-factory/internal/authn"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// Repository stores and retrieves raw schematics.
type Repository interface {
	Put(ctx context.Context, configuration *schematicpkg.Schematic) (string, error)
	GetRaw(ctx context.Context, id string) (*schematicpkg.Schematic, error)
}

// Service owns schematic validation, ownership, and persistence orchestration.
type Service struct {
	repository        Repository
	authEnabled       bool
	enterpriseEnabled bool
}

// NewService creates a schematic application service.
func NewService(repository Repository, authEnabled, enterpriseEnabled bool) *Service {
	return &Service{
		repository:        repository,
		authEnabled:       authEnabled,
		enterpriseEnabled: enterpriseEnabled,
	}
}

// Create validates and stores a schematic under the authenticated owner.
func (service *Service) Create(ctx context.Context, configuration *schematicpkg.Schematic) (string, error) {
	if service.authEnabled {
		principal, ok := authn.PrincipalFromContext(ctx)
		if !ok {
			return "", xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
		}

		if configuration.Owner != "" && configuration.Owner != principal.Username() {
			return "", xerrors.NewTagged[schematicpkg.ForbiddenTag](errors.New("schematic owner does not match authenticated user"))
		}

		configuration.Owner = principal.Username()
	}

	if err := configuration.Validate(service.enterpriseEnabled); err != nil {
		return "", err
	}

	return service.repository.Put(ctx, configuration)
}

// Get retrieves a schematic and enforces the configured ownership policy.
func (service *Service) Get(ctx context.Context, id string) (*schematicpkg.Schematic, error) {
	if service.authEnabled {
		if _, ok := authn.PrincipalFromContext(ctx); !ok {
			return nil, xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
		}
	}

	configuration, err := service.repository.GetRaw(ctx, id)
	if err != nil {
		return nil, err
	}

	if configuration.Owner == "" && !service.authEnabled {
		return configuration, nil
	}

	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok {
		return nil, xerrors.NewTagged[schematicpkg.RequiresAuthenticationTag](errors.New("authentication required"))
	}

	if principal.Username() != configuration.Owner {
		return nil, xerrors.NewTagged[schematicpkg.ForbiddenTag](errors.New("access denied"))
	}

	return configuration, nil
}
