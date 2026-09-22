// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func TestIntegrationCDN(t *testing.T) {
	pool := docker(t)

	// set up S3 access credentials for the tests, those are shared across all tests
	t.Setenv("AWS_ACCESS_KEY_ID", s3Access)
	t.Setenv("AWS_SECRET_ACCESS_KEY", s3Secret)

	t.Run("S3+CDN", func(t *testing.T) {
		options := cmd.DefaultOptions

		options.Cache.OCI = signingCacheRepository.OCIRepositoryOptions
		options.Metrics.Namespace = "test_s3_cdn"

		options.Cache.S3.Enabled = true
		options.Cache.S3.Bucket = "test-s3-cdn"
		options.Cache.S3.Insecure = true
		options.Cache.S3.Endpoint = setupS3(t, pool, options.Cache.S3.Bucket)

		options.Cache.CDN.Enabled = true
		options.Cache.CDN.TrimPrefix = fmt.Sprintf("/%s", options.Cache.S3.Bucket)
		options.Cache.CDN.Host = setupMockCDN(t, pool, options.Cache.S3.Endpoint, options.Cache.S3.Bucket)

		commonTest(t, options)
	})

	// same suite, with CDN URL signing on: the mock CDN rejects any redirect that arrives
	// without a token, so every asset download here proves the factory signed it.
	t.Run("S3+CDN+HMAC", func(t *testing.T) {
		secretPath := filepath.Join(t.TempDir(), "cdn-hmac-secret")
		require.NoError(t, os.WriteFile(secretPath, []byte("integration-test-secret"), 0o600))

		options := cmd.DefaultOptions

		options.Cache.OCI = signingCacheRepository.OCIRepositoryOptions
		options.Metrics.Namespace = "test_s3_cdn_hmac"

		options.Cache.S3.Enabled = true
		options.Cache.S3.Bucket = "test-s3-cdn-hmac"
		options.Cache.S3.Insecure = true
		options.Cache.S3.Endpoint = setupS3(t, pool, options.Cache.S3.Bucket)

		options.Cache.CDN.Enabled = true
		options.Cache.CDN.TrimPrefix = fmt.Sprintf("/%s", options.Cache.S3.Bucket)
		options.Cache.CDN.Host = setupMockCDNWithHMAC(t, pool, options.Cache.S3.Endpoint, options.Cache.S3.Bucket)
		options.Cache.CDN.HMACSecretPath = secretPath

		commonTest(t, options)
	})

	if enterprise.Enabled() {
		t.Run("AuthCDNNoRedirect", func(t *testing.T) {
			testAuthCDNNoRedirect(t, pool)
		})
	}
}
