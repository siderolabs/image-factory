// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
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

	cfg, err := schematic.Unmarshal(data)
	if err != nil {
		return err
	}

	id, err := f.schematicFactory.Put(ctx, cfg)
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := struct {
		ID string `json:"id"`
	}{
		ID: id,
	}

	return json.NewEncoder(w).Encode(resp)
}
