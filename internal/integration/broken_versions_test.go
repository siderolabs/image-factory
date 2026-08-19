// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/pkg/client"
)

// TestBrokenVersions verifies that versions listed in BrokenTalosVersions are
// excluded from the /versions response and reported by /versions?broken=true.
func TestBrokenVersions(t *testing.T) {
	t.Parallel()

	const brokenVersion = "v1.13.1"

	options := cmd.DefaultOptions
	options.Cache.OCI = cacheRepository.OCIRepositoryOptions
	options.Metrics.Namespace = "test_broken_versions"
	options.Build.BrokenTalosVersions = []string{brokenVersion[1:]}

	ctx, listenAddr, _ := setupFactory(t, options)
	baseURL := "http://" + listenAddr

	c, err := client.New(baseURL, clientAuthCredentials()...)
	require.NoError(t, err)

	broken, err := c.BrokenVersions(ctx)
	require.NoError(t, err)

	assert.Contains(t, broken, brokenVersion)

	versions, err := c.Versions(ctx)
	require.NoError(t, err)

	assert.NotContains(t, versions, brokenVersion)
	require.NotEmpty(t, versions)

	for _, path := range []string{
		"/versions",
		"/versions?broken=true",
		"/version/" + versions[0] + "/extensions/official",
		"/version/" + versions[0] + "/overlays/official",
	} {
		req, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		require.NoError(t, requestErr)

		resp, requestErr := http.DefaultClient.Do(req)
		require.NoError(t, requestErr)
		t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck

		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), path)
	}
}
