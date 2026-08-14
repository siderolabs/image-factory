// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/api"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	document, err := api.Load(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "3.2.0", document.OpenAPI)
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", document.JSONSchemaDialect)
	require.NotNil(t, document.Info)
	assert.Equal(t, "Image Factory API", document.Info.Title)
	require.NotNil(t, document.Paths)

	versions := document.Paths.Find("/versions")
	require.NotNil(t, versions)
	require.NotNil(t, versions.Get)
	assert.Equal(t, "listVersions", versions.Get.OperationID)
}
