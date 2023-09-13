// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import "time"

// Options configures image service.
type Options struct { //nolint:govet
	// Listen address for the HTTP frontend.
	HTTPListenAddr string

	// Asset builder options: minimum supported Talos version.
	MinTalosVersion string
	// Image registry for source images: imager, extensions, etc..
	ImageRegistry string

	// Options to verify container signatures for imager, extensions, etc.
	ContainerSignatureSubjectRegExp string
	ContainerSignatureIssuer        string

	// Maximum number of concurrent asset builds.
	AssetBuildMaxConcurrency int

	// External URL of the image service HTTP frontend.
	ExternalURL string

	// Flavor service OCI registry prefix.
	// It stores flavors for the image service as blobs under that path.
	FlavorServiceRepository string

	// OCI registry to store installer images has two endpoints:
	// - one for the image service to push images to
	// - external one for the redirects
	InstallerInternalRepository string
	InstallerExternalRepository string

	// TalosVersionRecheckInterval is the interval for rechecking Talos versions.
	TalosVersionRecheckInterval time.Duration
}

// DefaultOptions are the default options.
var DefaultOptions = Options{
	HTTPListenAddr: ":8080",

	MinTalosVersion: "1.4.0-alpha.0",
	ImageRegistry:   "ghcr.io",

	ContainerSignatureSubjectRegExp: `@siderolabs\.com$`,
	ContainerSignatureIssuer:        "https://accounts.google.com",

	AssetBuildMaxConcurrency: 6,

	ExternalURL: "https://localhost/",

	FlavorServiceRepository: "ghcr.io/siderolabs/image-service/flavors",

	InstallerInternalRepository: "ghcr.io/siderolabs",
	InstallerExternalRepository: "ghcr.io/siderolabs",

	TalosVersionRecheckInterval: 15 * time.Minute,
}
