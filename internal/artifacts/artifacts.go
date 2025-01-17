// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package artifacts handles acquiring and caching source Talos artifacts.
package artifacts

import (
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
)

// Options are the options for the artifacts manager.
type Options struct { //nolint:govet
	// ImageRegistry is the registry which stores imager, extensions, etc..
	//
	// For official images, this is "ghcr.io".
	ImageRegistry string
	// Option to allow using an image registry without TLS.
	InsecureImageRegistry bool
	// MinVersion is the minimum version of Talos to use.
	MinVersion semver.Version
	// ImageVerifyOptions are the options for verifying the image signature.
	ImageVerifyOptions []cosign.CheckOpts
	// TalosVersionRecheckInterval is the interval for rechecking Talos versions.
	TalosVersionRecheckInterval time.Duration
	// RemoteOptions is the list of remote options for the puller.
	RemoteOptions []remote.Option
}

// Kind is the artifact kind.
type Kind string

// Supported artifact kinds.
const (
	KindKernel      Kind = "vmlinuz"
	KindInitramfs   Kind = "initramfs.xz"
	KindSystemdBoot Kind = "systemd-boot.efi"
	KindSystemdStub Kind = "systemd-stub.efi"
	KindDTB         Kind = "dtb"
	KindUBoot       Kind = "u-boot"
	KindRPiFirmware Kind = "raspberrypi-firmware"
)

// FetchTimeout controls overall timeout for fetching artifacts for a release.
const FetchTimeout = 20 * time.Minute

// Various images.
const (
	InstallerImage         = "samip5/siderolabs/installer"
	ImagerImage            = "samip5/siderolabs/imager"
	ExtensionManifestImage = "samip5/siderolabs/extensions"
	OverlayManifestImage   = "samip5/siderolabs/overlays"
)

const tmpSuffix = "-tmp"

// ErrNotFoundTag tags the errors when the artifact is not found.
type ErrNotFoundTag = struct{}
