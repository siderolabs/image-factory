// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"strings"

	imagerprofile "github.com/siderolabs/talos/pkg/imager/profile"
)

// ArtifactSidecar identifies an optional checksum or signature suffix.
type ArtifactSidecar string

// Supported artifact sidecars.
const (
	ArtifactSidecarNone      ArtifactSidecar = ""
	ArtifactSidecarSHA256    ArtifactSidecar = ".sha256"
	ArtifactSidecarSHA512    ArtifactSidecar = ".sha512"
	ArtifactSidecarSignature ArtifactSidecar = ".sigstore.json"
)

// IsChecksum reports whether the sidecar requests a checksum.
func (sidecar ArtifactSidecar) IsChecksum() bool {
	return sidecar == ArtifactSidecarSHA256 || sidecar == ArtifactSidecarSHA512
}

// SplitArtifactPath separates one supported sidecar suffix from an artifact filename.
func SplitArtifactPath(path string) (string, ArtifactSidecar) {
	for _, sidecar := range []ArtifactSidecar{
		ArtifactSidecarSignature,
		ArtifactSidecarSHA512,
		ArtifactSidecarSHA256,
	} {
		if base, ok := strings.CutSuffix(path, string(sidecar)); ok {
			return base, sidecar
		}
	}

	return path, ArtifactSidecarNone
}

// ParseArtifactPath validates an artifact filename using the same parser as image handling.
func ParseArtifactPath(path, version string) (imagerprofile.Profile, error) {
	path, _ = SplitArtifactPath(path)

	return ParseFromPath(path, version)
}
