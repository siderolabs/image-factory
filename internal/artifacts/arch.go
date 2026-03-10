// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

// Arch is the artifacts architecture.
type Arch string

// Supported architectures.
const (
	ArchAmd64 Arch = "amd64"
	ArchArm64 Arch = "arm64"
)

// ValidArch returns true if the given string is a supported architecture.
func ValidArch(s string) bool {
	switch Arch(s) {
	case ArchAmd64, ArchArm64:
		return true
	default:
		return false
	}
}
