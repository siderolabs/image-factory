// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package artifacts handles acquiring and caching source Talos artifacts.
package artifacts

import (
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/siderolabs/image-factory/internal/image/verify"
)

// Options are the options for the artifacts manager.
type Options struct { //nolint:govet
	// ImageRegistry is the registry which stores imager, extensions, etc..
	//
	// For official images, this is "ghcr.io".
	ImageRegistry string
	// ImageRegistryNamespace is an optional repository path prefix (e.g. a Harbor
	// proxy-cache project) prepended to every image pulled from ImageRegistry,
	// including extension and overlay images discovered via the manifests.
	ImageRegistryNamespace string
	// Repository to source extra extensions from.
	ExtraExtensionsImageRegistry string
	// Allow using an image registry without TLS.
	InsecureImageRegistry bool
	// Allow using an image registry without TLS for extra extensions.
	InsecureExtraExtensionsRegistry bool
	// MinVersion is the minimum version of Talos to use.
	MinVersion semver.Version
	// BrokenVersions are Talos versions that should be rejected when listing available versions.
	BrokenVersions []semver.Version
	// ImageVerifyOptions are the options for verifying the image signature.
	ImageVerifyOptions ImageVerifyOptions
	// TalosVersionRecheckInterval is the interval for rechecking Talos versions.
	TalosVersionRecheckInterval time.Duration
	// RemoteOptions is the list of remote options for the puller.
	RemoteOptions []remote.Option
	// RegistryRefreshInterval is the interval for refreshing the image registry connections.
	RegistryRefreshInterval time.Duration

	// Images used by the artifacts manager.
	InstallerBaseImage          string
	InstallerImage              string
	ImagerImage                 string
	ExtensionManifestImage      string
	ExtraExtensionManifestImage string
	OverlayManifestImage        string
	TalosctlImage               string

	// External identification.
	ExternalURL string
}

// ImageVerifyOptions are the options for verifying the image signature.
type ImageVerifyOptions = verify.VerifyOptions

// Kind is the artifact kind.
type Kind string

// Supported artifact kinds.
const (
	KindKernel      Kind = "vmlinuz"
	KindInitramfs   Kind = "initramfs.xz"
	KindSystemdBoot Kind = "systemd-boot.efi"
	KindSystemdStub Kind = "systemd-stub.efi"
)

// OverlayKind if the kind of overlay artifacts.
type OverlayKind string

// Supported overlay kinds.
const (
	OverlayKindProfiles OverlayKind = "profiles"
)

// FetchTimeout controls overall timeout for fetching artifacts for a release.
const FetchTimeout = 20 * time.Minute

const tmpSuffix = "-tmp"

// ErrNotFoundTag tags the errors when the artifact is not found.
type ErrNotFoundTag struct{}
