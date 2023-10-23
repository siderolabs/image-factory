// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import "time"

// Options configures image factory.
type Options struct { //nolint:govet
	// Listen address for the HTTP frontend.
	HTTPListenAddr string

	// Asset builder options: minimum supported Talos version.
	MinTalosVersion string
	// Image registry for source images: imager, extensions, etc..
	ImageRegistry string
	// Allow insecure connection to the image registry
	InsecureImageRegistry bool

	// Options to verify container signatures for imager, extensions, etc.
	ContainerSignatureSubjectRegExp string
	ContainerSignatureIssuer        string

	// Maximum number of concurrent asset builds.
	AssetBuildMaxConcurrency int

	// External URL of the image factory HTTP frontend.
	ExternalURL string

	// Schematic service OCI registry prefix.
	// It stores schematics for the image factory as blobs under that path.
	SchematicServiceRepository string

	// OCI registry to store installer images has two endpoints:
	// - one for the image factory to push images to
	// - external one for the redirects
	InstallerInternalRepository string
	InstallerExternalRepository string
	// Allow insecure connection to the internal installer repository
	InsecureInstallerInternalRepository bool

	// TalosVersionRecheckInterval is the interval for rechecking Talos versions.
	TalosVersionRecheckInterval time.Duration

	// CacheSigningKeyPath is the path to the signing key for the cache.
	//
	// Best choice is to use ECDSA key.
	CacheSigningKeyPath string
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

	SchematicServiceRepository: "ghcr.io/siderolabs/image-factory/schematics",

	InstallerInternalRepository: "ghcr.io/siderolabs",
	InstallerExternalRepository: "ghcr.io/siderolabs",

	TalosVersionRecheckInterval: 15 * time.Minute,
}
