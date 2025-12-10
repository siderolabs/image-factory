// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/artifacts/internal/image"
)

func siderolabsVerifyOptions(t *testing.T) image.VerifyOptions {
	t.Helper()

	trustedRoot, err := cosign.TrustedRoot()
	require.NoError(t, err)

	return image.VerifyOptions{
		CheckOpts: []cosign.CheckOpts{
			{
				TrustedMaterial: trustedRoot,

				Identities: []cosign.Identity{
					{
						Issuer:        "https://accounts.google.com",
						SubjectRegExp: `@siderolabs\.com$`,
					},
				},
			},
		},
	}
}

func TestVerifyLegacy(t *testing.T) {
	t.Parallel()

	result, err := image.VerifySignatures(
		t.Context(),
		name.MustParseReference("ghcr.io/siderolabs/talos:v1.11.0"),
		siderolabsVerifyOptions(t),
	)
	require.NoError(t, err)

	assert.True(t, result.Verified)
	assert.Equal(t, "legacy: certificate subject", result.Method)
}

func TestVerifyBundledSuccess(t *testing.T) {
	t.Parallel()

	result, err := image.VerifySignatures(
		t.Context(),
		name.MustParseReference("ghcr.io/siderolabs/talos:v1.11.5"),
		siderolabsVerifyOptions(t),
	)
	require.NoError(t, err)

	assert.True(t, result.Verified)
	assert.Equal(t, "bundled: certificate subject", result.Method)
}

func TestVerifyFailure(t *testing.T) {
	t.Parallel()

	_, err := image.VerifySignatures(
		t.Context(),
		name.MustParseReference("ghcr.io/siderolabs/talos:v0.8.0"),
		siderolabsVerifyOptions(t),
	)
	require.Error(t, err)
	require.EqualError(t, err, "no valid bundles exist in registry\nno signatures found")
}
