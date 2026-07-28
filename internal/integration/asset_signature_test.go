// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/image-factory/pkg/client"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func testAssetSignatureFrontend(ctx context.Context, t *testing.T, baseURL string) {
	assetPath := "kernel-amd64"
	version := "v1.12.4"

	if !enterprise.Enabled() {
		signatureResponse := downloadAsset(ctx, t, baseURL, emptySchematicID, version, assetPath+".sigstore.json")
		assert.Equal(t, http.StatusPaymentRequired, signatureResponse.StatusCode)

		return
	}

	c, err := client.New(baseURL, clientAuthCredentials()...)
	require.NoError(t, err)
	require.Equal(t, emptySchematicID, createSchematicGetID(ctx, t, c, *testSchematics[emptySchematicID]))

	signatureResponse := downloadAsset(ctx, t, baseURL, emptySchematicID, version, assetPath+".sigstore.json")
	require.Equal(t, http.StatusOK, signatureResponse.StatusCode)
	assert.Equal(t, "application/vnd.dev.sigstore.bundle.v0.3+json", signatureResponse.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="kernel-amd64.sigstore.json"`, signatureResponse.Header.Get("Content-Disposition"))

	bundleJSON, err := io.ReadAll(signatureResponse.Body)
	require.NoError(t, err)

	headRequest, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL+"/image/"+emptySchematicID+"/"+version+"/"+assetPath+".sigstore.json", nil)
	require.NoError(t, err)
	addTestAuth(headRequest)

	headResponse, err := http.DefaultClient.Do(headRequest)
	require.NoError(t, err)
	defer headResponse.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, headResponse.StatusCode)
	assert.Equal(t, int64(len(bundleJSON)), headResponse.ContentLength)

	headBody, err := io.ReadAll(headResponse.Body)
	require.NoError(t, err)
	assert.Empty(t, headBody)

	bundle := &protobundle.Bundle{}
	require.NoError(t, protojson.Unmarshal(bundleJSON, bundle))
	messageSignature := bundle.GetMessageSignature()
	require.NotNil(t, messageSignature)

	assetResponse := downloadAsset(ctx, t, baseURL, emptySchematicID, version, assetPath)
	require.Equal(t, http.StatusOK, assetResponse.StatusCode)

	assetBytes, err := io.ReadAll(assetResponse.Body)
	require.NoError(t, err)

	expectedDigest := sha256.Sum256(assetBytes)
	require.Equal(t, expectedDigest[:], messageSignature.GetMessageDigest().GetDigest())

	keyRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/oci/cosign/signing-key.pub", nil)
	require.NoError(t, err)
	addTestAuth(keyRequest)

	keyResponse, err := http.DefaultClient.Do(keyRequest)
	require.NoError(t, err)
	defer keyResponse.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, keyResponse.StatusCode)

	publicKeyPEM, err := io.ReadAll(keyResponse.Body)
	require.NoError(t, err)

	temporaryDirectory := t.TempDir()
	assetFile := filepath.Join(temporaryDirectory, assetPath)
	bundleFile := assetFile + ".sigstore.json"
	publicKeyFile := filepath.Join(temporaryDirectory, "signing-key.pub")

	require.NoError(t, os.WriteFile(assetFile, assetBytes, 0o600))
	require.NoError(t, os.WriteFile(bundleFile, bundleJSON, 0o600))
	require.NoError(t, os.WriteFile(publicKeyFile, publicKeyPEM, 0o600))

	command := exec.CommandContext(
		ctx,
		cosignPath,
		"verify-blob",
		"--key", publicKeyFile,
		"--bundle", bundleFile,
		"--insecure-ignore-tlog",
		assetFile,
	)

	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "cosign verify-blob failed:\n%s", output)
	assert.Contains(t, string(output), "Verified OK")
}
