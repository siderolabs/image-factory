// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

func TestSchematicHandlerCreateDecodesAndEncodesHTTP(t *testing.T) {
	t.Parallel()

	service := &schematicService{id: "schematic-id"}
	handler := httpapi.NewSchematicHandler(service)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/schematics", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	err := handler.Create(t.Context(), response, request, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
	require.NotNil(t, service.created)

	var result struct {
		ID        string `json:"id"`
		Schematic string `json:"schematic"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, "schematic-id", result.ID)
	require.NotEmpty(t, result.Schematic)
}

type schematicService struct {
	created *schematicpkg.Schematic
	id      string
}

func (service *schematicService) Create(_ context.Context, configuration *schematicpkg.Schematic) (string, error) {
	service.created = configuration

	return service.id, nil
}

func (service *schematicService) Get(context.Context, string) (*schematicpkg.Schematic, error) {
	panic("unexpected Get call")
}
