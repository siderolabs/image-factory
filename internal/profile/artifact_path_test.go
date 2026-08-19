// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	imageprofile "github.com/siderolabs/image-factory/internal/profile"
)

func TestSplitArtifactPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path        string
		base        string
		wantSidecar imageprofile.ArtifactSidecar
	}{
		{path: "metal-amd64.raw.xz", base: "metal-amd64.raw.xz", wantSidecar: imageprofile.ArtifactSidecarNone},
		{path: "metal-amd64.raw.xz.sha256", base: "metal-amd64.raw.xz", wantSidecar: imageprofile.ArtifactSidecarSHA256},
		{path: "metal-amd64.iso.sha512", base: "metal-amd64.iso", wantSidecar: imageprofile.ArtifactSidecarSHA512},
		{path: "kernel-amd64.sigstore.json", base: "kernel-amd64", wantSidecar: imageprofile.ArtifactSidecarSignature},
		{path: "metal-amd64.raw.xz.sha256.sigstore.json", base: "metal-amd64.raw.xz.sha256", wantSidecar: imageprofile.ArtifactSidecarSignature},
	} {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			base, sidecar := imageprofile.SplitArtifactPath(test.path)
			assert.Equal(t, test.base, base)
			assert.Equal(t, test.wantSidecar, sidecar)
		})
	}
}

func TestParseArtifactPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"foo.bar-amd64.raw",
		"metal-amd64.raw.xz.sha256",
		"metal-amd64.iso.sha512",
		"kernel-amd64.sigstore.json",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			_, err := imageprofile.ParseArtifactPath(path, "1.12.0")
			require.NoError(t, err)
		})
	}

	for _, path := range []string{
		"kernel-amd64.iso",
		"cmdline-amd64.raw",
		"installer-installer-amd64.tar",
		"metal-amd64.raw.xz.sha256.sigstore.json",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			_, err := imageprofile.ParseArtifactPath(path, "1.12.0")
			require.Error(t, err)
		})
	}
}
