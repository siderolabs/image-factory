// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getSecureBootCert(ctx context.Context, t *testing.T, baseURL string) []byte {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/secureboot/signing-cert.pem", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	t.Cleanup(func() {
		resp.Body.Close()
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/x-pem-file", resp.Header.Get("Content-Type"))

	pem, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return pem
}

func testSecureBootFrontend(ctx context.Context, t *testing.T, baseURL string) {
	t.Run("secureboot certificate", func(t *testing.T) {
		t.Parallel()

		pem := getSecureBootCert(ctx, t, baseURL)

		assert.Equal(t, secureBootSigningCert, pem)
	})
}
