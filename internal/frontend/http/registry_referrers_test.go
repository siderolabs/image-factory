// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	registryhttp "github.com/siderolabs/image-factory/internal/frontend/http"
)

func TestApplyReferrersFilterHeader(t *testing.T) {
	t.Parallel()

	t.Run("reports applied artifact type filter", func(t *testing.T) {
		t.Parallel()

		header := http.Header{}
		registryhttp.ApplyReferrersFilterHeader(header, "application/example")

		require.Equal(t, "artifactType", header.Get("Oci-Filters-Applied"))
	})

	t.Run("omits header without filter", func(t *testing.T) {
		t.Parallel()

		header := http.Header{}
		registryhttp.ApplyReferrersFilterHeader(header, "")

		require.Empty(t, header.Get("Oci-Filters-Applied"))
	})
}
