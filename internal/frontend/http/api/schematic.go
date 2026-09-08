// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package api implements HTTP adapters for Image Factory application services.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"

	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

// SchematicService owns schematic validation, ownership, and persistence.
type SchematicService interface {
	Create(ctx context.Context, configuration *schematicpkg.Schematic) (string, error)
	Get(ctx context.Context, id string) (*schematicpkg.Schematic, error)
}

// SchematicHandler adapts schematic application operations to HTTP.
type SchematicHandler struct {
	service SchematicService
}

// NewSchematicHandler creates a schematic HTTP adapter.
func NewSchematicHandler(service SchematicService) *SchematicHandler {
	return &SchematicHandler{service: service}
}

// Create decodes, creates, and encodes a schematic.
func (handler *SchematicHandler) Create(ctx context.Context, writer http.ResponseWriter, request *http.Request, _ httprouter.Params) error {
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}

	if err = request.Body.Close(); err != nil {
		return err
	}

	configuration, err := schematicpkg.Unmarshal(data)
	if err != nil {
		return err
	}

	id, err := handler.service.Create(ctx, configuration)
	if err != nil {
		return err
	}

	normalized, err := configuration.Marshal()
	if err != nil {
		return err
	}

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)

	response := struct {
		ID        string `json:"id"`
		Schematic string `json:"schematic"`
	}{
		ID:        id,
		Schematic: string(normalized),
	}

	return json.NewEncoder(writer).Encode(response)
}

// Get retrieves and encodes a schematic.
func (handler *SchematicHandler) Get(ctx context.Context, writer http.ResponseWriter, _ *http.Request, params httprouter.Params) error {
	configuration, err := handler.service.Get(ctx, params.ByName("schematic"))
	if err != nil {
		if xerrors.TagIs[schematicpkg.NotFoundTag](err) {
			http.Error(writer, "schematic not found", http.StatusNotFound)

			return nil
		}

		return err
	}

	marshaled, err := configuration.Marshal()
	if err != nil {
		return err
	}

	writer.Header().Add("Content-Type", "application/yaml")

	_, err = writer.Write(marshaled)

	return err
}
