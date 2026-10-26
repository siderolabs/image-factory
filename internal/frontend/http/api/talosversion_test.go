// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/siderolabs/gen/xerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/siderolabs/image-factory/internal/frontend/http/api"
	"github.com/siderolabs/image-factory/internal/frontend/http/transport"
	"github.com/siderolabs/image-factory/internal/talosversion"
)

func TestTalosVersionHandler(t *testing.T) {
	t.Parallel()

	handler := httpapi.NewTalosVersionHandler()
	version := httprouter.Params{{Key: "version", Value: "v1.13.0"}}

	response, err := serve(t, handler.Platforms, "/api/version/v1.13.0/platforms", version)
	require.NoError(t, err)

	platforms := decode[[]map[string]any](t, response)
	require.NotEmpty(t, platforms)
	assert.Contains(t, platforms, map[string]any{
		"name":                "aws",
		"label":               "Amazon Web Services (AWS)",
		"description":         platforms[0]["description"],
		"documentation":       "/platform-specific-installations/cloud-platforms/aws",
		"architectures":       []any{"amd64", "arm64"},
		"bootMethods":         []any{"disk-image"},
		"secureBootSupported": false,
		"notOnlyDiskImage":    false,
	})

	response, err = serve(t, handler.Boards, "/api/version/v1.13.0/boards", version)
	require.NoError(t, err)

	boards := decode[[]map[string]any](t, response)
	require.NotEmpty(t, boards)
	assert.Contains(t, boards[0], "overlay")

	response, err = serve(t, handler.Features, "/api/version/v1.13.0/features", version)
	require.NoError(t, err)

	features := decode[talosversion.Features](t, response)
	assert.True(t, features.Has(talosversion.FeatureSBOM))

	_, err = serve(t, handler.Features, "/api/version/latest/features", httprouter.Params{{Key: "version", Value: "latest"}})
	require.True(t, xerrors.TagIs[transport.InvalidRequestTag](err), err)
}
