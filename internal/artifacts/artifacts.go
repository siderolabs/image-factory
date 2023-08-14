// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package artifacts handles acquiring and caching source Talos artifacts.
package artifacts

import (
	"time"

	"github.com/blang/semver/v4"
	"github.com/sigstore/cosign/v2/pkg/cosign"
)

// Options are the options for the artifacts manager.
type Options struct {
	// ImagePrefix is the prefix for the image name.
	//
	// For official images, this is "ghcr.io/siderolabs/"
	ImagePrefix string
	// MinVersion is the minimum version of Talos to use.
	MinVersion semver.Version
	// ImageVerifyOptions are the options for verifying the image signature.
	ImageVerifyOptions cosign.CheckOpts
}

// Kind is the artifact kind.
type Kind string

// Supported artifact kinds.
const (
	KindKernel      Kind = "vmlinuz"
	KindInitramfs   Kind = "initramfs.xz"
	KindSystemdBoot Kind = "systemd-boot.efi"
	KindSystemdStub Kind = "systemd-stub.efi"
)

// FetchTimeout controls overall timeout for fetching artifacts for a release.
const FetchTimeout = 20 * time.Minute
