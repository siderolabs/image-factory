// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	factoryapi "github.com/siderolabs/image-factory/api"
)

func (f *Frontend) handleOpenAPI(_ context.Context, w http.ResponseWriter, _ *http.Request, _ httprouter.Params) error {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "no-cache")

	if _, err := w.Write(factoryapi.Specification()); err != nil {
		return fmt.Errorf("write OpenAPI document: %w", err)
	}

	return nil
}
