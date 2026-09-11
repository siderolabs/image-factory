// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
)

func TestTalosctlHandlerListsDownloadURLs(t *testing.T) {
	t.Parallel()

	handler := httpapi.NewTalosctlHandler(
		talosctlService{filenames: []string{"talosctl-darwin-arm64", "talosctl-linux-amd64"}},
		mustParseURL(t, "https://factory.example.com"),
	)
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/talosctl/1.9.0", nil)
	params := httprouter.Params{{Key: "version", Value: "1.9.0"}}

	err := handler.List(t.Context(), response, request, params)
	require.NoError(t, err)
	require.JSONEq(t, `[
		"https://factory.example.com/talosctl/v1.9.0/talosctl-darwin-arm64",
		"https://factory.example.com/talosctl/v1.9.0/talosctl-linux-amd64"
	]`, response.Body.String())
	require.Equal(t, "application/json", response.Header().Get("Content-Type"))
}

type talosctlService struct {
	filenames []string
}

func (service talosctlService) List(context.Context, string) (string, []string, error) {
	return "v1.9.0", service.filenames, nil
}

func (talosctlService) Open(context.Context, string, string, bool) (artifacts.TalosctlDownload, error) {
	return artifacts.TalosctlDownload{
		Reader:   io.NopCloser(strings.NewReader("binary")),
		Size:     6,
		Version:  "v1.9.0",
		Filename: "talosctl-linux-amd64",
	}, nil
}
