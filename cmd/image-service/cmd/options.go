// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

// Options configures image service.
type Options struct { //nolint:govet
	HTTPListenAddr string

	MinTalosVersion string
	ImagePrefix     string

	ConfigKeyBase64 string

	ContainerSignatureSubjectRegExp string
	ContainerSignatureIssuer        string

	AssetBuildMaxConcurrency int

	ExternalURL string
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
}
