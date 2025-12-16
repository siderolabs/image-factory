// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"testing"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func TestIntegrationS3(t *testing.T) {
	pool := docker(t)

	// set up S3 access credentials for the tests, those are shared across all tests
	t.Setenv("AWS_ACCESS_KEY_ID", s3Access)
	t.Setenv("AWS_SECRET_ACCESS_KEY", s3Secret)

	t.Run("S3", func(t *testing.T) {
		options := cmd.DefaultOptions

		options.Cache.OCI = signingCacheRepository.OCIRepositoryOptions
		options.Metrics.Namespace = "test_s3"

		options.Cache.S3.Enabled = true
		options.Cache.S3.Bucket = "test-s3"
		options.Cache.S3.Insecure = true
		options.Cache.S3.Endpoint = setupS3(t, pool, options.Cache.S3.Bucket)

		commonTest(t, options)
	})
}
