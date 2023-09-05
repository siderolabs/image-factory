// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

// Options configures image service.
type Options struct { //nolint:govet
	// Listen address for the HTTP frontend.
	HTTPListenAddr string

	// Asset builder options: minimum supported Talos version.
	MinTalosVersion string
	// Image prefix for the `imager`.
	ImagePrefix string

	// Options to verify container signatures for imager, extensions, etc.
	ContainerSignatureSubjectRegExp string
	ContainerSignatureIssuer        string

	// Maximum number of concurrent asset builds.
	AssetBuildMaxConcurrency int

	// External URL of the image service HTTP frontend.
	ExternalURL string

	// Configuration service OCI registry prefix.
	// It stores configurations for the image service as blobs under that path.
	ConfigurationServiceRepository string
}

// DefaultOptions are the default options.
var DefaultOptions = Options{
	HTTPListenAddr: ":8080",

	MinTalosVersion: "1.4.0",
	ImagePrefix:     "ghcr.io/siderolabs/",

	ContainerSignatureSubjectRegExp: `@siderolabs\.com$`,
	ContainerSignatureIssuer:        "https://accounts.google.com",

	AssetBuildMaxConcurrency: 6,

	ExternalURL: "https://localhost/",

	ConfigurationServiceRepository: "ghcr.io/siderolabs/image-service/configuration",
}
