// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func testFrontend(ctx context.Context, t *testing.T, baseURL string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	server := resp.Header.Get("Server")

	if enterprise.Enabled() {
		assert.Contains(t, server, "Image Factory Enterprise")
	} else {
		assert.Contains(t, server, "Image Factory")
		assert.NotContains(t, server, "Enterprise")
	}
}
