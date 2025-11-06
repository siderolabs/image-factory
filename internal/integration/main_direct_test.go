// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"testing"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func TestIntegrationDirect(t *testing.T) {
	options := cmd.DefaultOptions

	options.CacheRepository = cacheRepository
	options.MetricsNamespace = ""

	commonTest(t, options)
}
