// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cdn

package integration_test

import (
	"fmt"
	"testing"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func TestIntegrationCDN(t *testing.T) {
	pool := docker(t)

	// set up S3 access credentials for the tests, those are shared across all tests
	t.Setenv("AWS_ACCESS_KEY_ID", s3Access)
	t.Setenv("AWS_SECRET_ACCESS_KEY", s3Secret)

	t.Run("S3+CDN", func(t *testing.T) {
		options := cmd.DefaultOptions

		options.CacheRepository = signingCacheRepository
		options.MetricsNamespace = "test_s3_cdn"

		options.CacheS3Enabled = true
		options.CacheS3Bucket = "test-s3-cdn"
		options.InsecureCacheS3 = true
		options.CacheS3Endpoint = setupS3(t, pool, options.CacheS3Bucket)

		options.CacheCDNEnabled = true
		options.CacheCDNTrimPrefix = fmt.Sprintf("/%s", options.CacheS3Bucket)
		options.CacheCDNHost = setupMockCDN(t, pool, options.CacheS3Endpoint, options.CacheS3Bucket)

		commonTest(t, options)
	})
}
