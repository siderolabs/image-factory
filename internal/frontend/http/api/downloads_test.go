// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts"
	"github.com/siderolabs/image-factory/internal/downloads"
	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	schematicpkg "github.com/siderolabs/image-factory/pkg/schematic"
)

const knownSchematic = "known"

type storedSchematics struct{}

func (storedSchematics) Create(context.Context, *schematicpkg.Schematic) (string, error) {
	panic("unexpected Create call")
}

func (storedSchematics) Get(_ context.Context, id string) (*schematicpkg.Schematic, error) {
	if id != knownSchematic {
		return nil, xerrors.NewTaggedf[schematicpkg.NotFoundTag]("schematic %q not found", id)
	}

	return &schematicpkg.Schematic{
		Customization: schematicpkg.Customization{
			ExtraKernelArgs: []string{"console=ttyS0"},
			SystemExtensions: schematicpkg.SystemExtensions{
				OfficialExtensions: []string{"siderolabs/amd-ucode"},
			},
		},
	}, nil
}

type talosctlTuples struct{}

func (talosctlTuples) GetTalosctlTuples(context.Context, string) ([]artifacts.TalosctlTuple, error) {
	return []artifacts.TalosctlTuple{{OS: "linux", Arch: "amd64"}}, nil
}

type username string

func (u username) UsernameFromContext(context.Context) (string, bool) {
	return string(u), u != ""
}

func newDownloadsHandler(usernames httpapi.UsernameSource) *httpapi.DownloadsHandler {
	return httpapi.NewDownloadsHandler(storedSchematics{}, talosctlTuples{}, usernames, downloads.Links{
		ExternalURL:    &url.URL{Scheme: "https", Host: "factory.example.com"},
		ExternalPXEURL: &url.URL{Scheme: "https", Host: "factory.example.com"},
	})
}

func serve(t *testing.T, handle func(context.Context, http.ResponseWriter, *http.Request, httprouter.Params) error, target string, params httprouter.Params) (*httptest.ResponseRecorder, error) {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	response := httptest.NewRecorder()

	return response, handle(t.Context(), response, request, params)
}

func decode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()

	require.Equal(t, "application/json", response.Header().Get("Content-Type"))

	var result T

	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))

	return result
}

func TestDownloadsHandlerAssets(t *testing.T) {
	t.Parallel()

	handler := newDownloadsHandler(username("alice"))
	params := httprouter.Params{{Key: "schematic", Value: knownSchematic}, {Key: "version", Value: "v1.13.0"}}

	response, err := serve(t, handler.Assets, "/api/schematics/known/v1.13.0/assets?target=metal&arch=amd64", params)
	require.NoError(t, err)

	assets := decode[downloads.Assets](t, response)
	require.NotEmpty(t, assets.BootAssets)
	assert.Equal(t, "factory.example.com/metal-installer/known:v1.13.0", assets.InstallerImage)
	assert.Equal(t, []downloads.Talosctl{{Name: "talosctl-linux-amd64", URL: "https://factory.example.com/talosctl/v1.13.0/talosctl-linux-amd64"}}, assets.Talosctl)

	for _, asset := range assets.BootAssets {
		if asset.Kind == downloads.AssetPXE {
			assert.Equal(t, "https://alice:<password>@factory.example.com/pxe/known/v1.13.0/metal-amd64", asset.URL)
		}
	}

	for _, target := range []string{
		"?target=metal&arch=riscv64",
		"?target=cloud&platform=nope&arch=amd64",
		"?target=metal&arch=amd64&secureboot=maybe",
	} {
		_, err = serve(t, handler.Assets, "/api/schematics/known/v1.13.0/assets"+target, params)
		require.True(t, xerrors.TagIs[transport.InvalidRequestTag](err), "%s: %v", target, err)
	}

	_, err = serve(t, handler.Assets, "/api/schematics/unknown/v1.13.0/assets?target=metal&arch=amd64",
		httprouter.Params{{Key: "schematic", Value: "unknown"}, {Key: "version", Value: "v1.13.0"}})
	require.True(t, xerrors.TagIs[schematicpkg.NotFoundTag](err), err)
}
