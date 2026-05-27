// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
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
}
