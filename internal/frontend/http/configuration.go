// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/image-factory/internal/schematic/storage"
	"github.com/siderolabs/image-factory/pkg/enterprise"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// handleSchematicCreate handles creation of the schematic.
func (f *Frontend) handleSchematicCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err = r.Body.Close(); err != nil {
		return err
	}

	schematic, err := schematicpkg.Unmarshal(data)
	if err != nil {
		return err
	}

	if f.options.AuthProvider != nil {
		if username, ok := f.options.AuthProvider.UsernameFromContext(ctx); ok {
			if schematic.Owner != "" && schematic.Owner != username {
				return xerrors.NewTagged[schematicpkg.ForbiddenTag](errors.New("schematic owner does not match authenticated user"))
			}

			schematic.Owner = username
		}
	}

	err = schematic.Validate(enterprise.Enabled())
	if err != nil {
		return err
	}

	id, err := f.schematicFactory.Put(ctx, schematic)
	if err != nil {
		return err
	}

	normalized, err := schematic.Marshal()
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := struct {
		ID        string `json:"id"`
		Schematic string `json:"schematic"`
	}{
		ID:        id,
		Schematic: string(normalized),
	}

	return json.NewEncoder(w).Encode(resp)
}

// handleSchematicGet handles retrieval of the schematic.
func (f *Frontend) handleSchematicGet(ctx context.Context, w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	schematicID := p.ByName("schematic")

	schematic, err := f.schematicFactory.Get(ctx, schematicID, f.options.AuthProvider)
	if err != nil {
		if xerrors.TagIs[storage.ErrNotFoundTag](err) {
			http.Error(w, "schematic not found", http.StatusNotFound)

			return nil
		}

		return err
	}

	marshaled, err := schematic.Marshal()
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/yaml")

	_, err = w.Write(marshaled)

	return err
}
