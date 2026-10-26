// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package downloads builds the download links of a schematic: its boot assets and their parts, its installer
// image and talosctl binaries, and on Enterprise its checksums, signatures, SBOM, VEX and vulnerability scans.
package downloads

// Target is the kind of machine an image is built for.
type Target string

// Targets.
const (
	TargetMetal Target = "metal"
	TargetCloud Target = "cloud"
	TargetSBC   Target = "sbc"
)
