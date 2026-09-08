// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package schematic_test

import (
	"context"
	"testing"

	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/authn"
	"github.com/siderolabs/image-factory/internal/schematic"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func TestServiceCreateAssignsAuthenticatedOwner(t *testing.T) {
	t.Parallel()

	store := &schematicStore{}
	service := schematic.NewService(store, true, true)
	configuration := &schematicpkg.Schematic{}
	principal, err := authn.NewPrincipal("alice", authn.CredentialProvider)
	require.NoError(t, err)
	ctx := authn.ContextWithPrincipal(t.Context(), principal)

	id, err := service.Create(ctx, configuration)
	require.NoError(t, err)
	require.Equal(t, "schematic-id", id)
	require.Equal(t, "alice", configuration.Owner)
	require.Same(t, configuration, store.putConfiguration)
}

func TestServiceCreateRejectsMismatchedOwner(t *testing.T) {
	t.Parallel()

	store := &schematicStore{}
	service := schematic.NewService(store, true, true)
	configuration := &schematicpkg.Schematic{Owner: "bob"}
	principal, err := authn.NewPrincipal("alice", authn.CredentialProvider)
	require.NoError(t, err)
	ctx := authn.ContextWithPrincipal(t.Context(), principal)

	_, err = service.Create(ctx, configuration)
	require.ErrorContains(t, err, "schematic owner does not match authenticated user")
	require.True(t, xerrors.TagIs[schematicpkg.ForbiddenTag](err))
	require.Nil(t, store.putConfiguration)
}

func TestServiceCreateRequiresPrincipal(t *testing.T) {
	t.Parallel()

	store := &schematicStore{}
	service := schematic.NewService(store, true, true)

	_, err := service.Create(t.Context(), &schematicpkg.Schematic{})
	require.ErrorContains(t, err, "authentication required")
	require.True(t, xerrors.TagIs[schematicpkg.RequiresAuthenticationTag](err))
	require.Nil(t, store.putConfiguration)
}

func TestServiceGetRequiresPrincipalForOwnedSchematic(t *testing.T) {
	t.Parallel()

	store := &schematicStore{
		getConfiguration: &schematicpkg.Schematic{Owner: "alice"},
	}
	service := schematic.NewService(store, true, true)

	_, err := service.Get(t.Context(), "schematic-id")
	require.ErrorContains(t, err, "authentication required")
	require.True(t, xerrors.TagIs[schematicpkg.RequiresAuthenticationTag](err))
	require.Zero(t, store.getCalls)
}

func TestServiceGetRejectsDifferentOwner(t *testing.T) {
	t.Parallel()

	service := schematic.NewService(&schematicStore{
		getConfiguration: &schematicpkg.Schematic{Owner: "alice"},
	}, true, true)
	principal, err := authn.NewPrincipal("bob", authn.CredentialProvider)
	require.NoError(t, err)

	_, err = service.Get(authn.ContextWithPrincipal(t.Context(), principal), "schematic-id")
	require.ErrorContains(t, err, "access denied")
	require.True(t, xerrors.TagIs[schematicpkg.ForbiddenTag](err))
}

type schematicStore struct {
	putConfiguration *schematicpkg.Schematic
	getConfiguration *schematicpkg.Schematic
	getCalls         int
}

func (store *schematicStore) Put(_ context.Context, configuration *schematicpkg.Schematic) (string, error) {
	store.putConfiguration = configuration

	return "schematic-id", nil
}

func (store *schematicStore) GetRaw(context.Context, string) (*schematicpkg.Schematic, error) {
	store.getCalls++

	return store.getConfiguration, nil
}
