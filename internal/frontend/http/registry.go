// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"

	"github.com/siderolabs/image-factory/internal/schematic"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// registrySchematics retains the legacy provider-context compatibility at composition.
type registrySchematics struct {
	factory  *schematic.Factory
	provider enterprise.AuthProvider
}

func (source registrySchematics) Get(ctx context.Context, id string) (*schematicpkg.Schematic, error) {
	return source.factory.Get(ctx, id, source.provider)
}
