// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main is the entrypoint of the image service.
package main

import (
	"flag"

	"github.com/siderolabs/image-service/cmd/image-service/cmd"
)

func initFlags() cmd.Options {
	var opts cmd.Options

	flag.StringVar(&opts.HTTPListenAddr, "http-port", cmd.DefaultOptions.HTTPListenAddr, "HTTP listen address")

	flag.StringVar(&opts.MinTalosVersion, "min-talos-version", cmd.DefaultOptions.MinTalosVersion, "minimum Talos version")
	flag.StringVar(&opts.ImagePrefix, "image-prefix", cmd.DefaultOptions.ImagePrefix, "image prefix")

	flag.StringVar(&opts.ContainerSignatureSubjectRegExp, "container-signature-subject-regexp", cmd.DefaultOptions.ContainerSignatureSubjectRegExp, "container signature subject regexp")
	flag.StringVar(&opts.ContainerSignatureIssuer, "container-signature-issuer", cmd.DefaultOptions.ContainerSignatureIssuer, "container signature issuer")

	flag.IntVar(&opts.AssetBuildMaxConcurrency, "asset-builder-max-concurrency", cmd.DefaultOptions.AssetBuildMaxConcurrency, "maximum concurrency for asset builder")

	flag.StringVar(&opts.ExternalURL, "external-url", cmd.DefaultOptions.ExternalURL, "service external endpoint URL")

	flag.StringVar(&opts.FlavorServiceRepository, "flavor-service-repository", cmd.DefaultOptions.FlavorServiceRepository, "image repository for the flavor service")

	flag.StringVar(&opts.InstallerExternalRepository, "installer-external-repository", cmd.DefaultOptions.InstallerExternalRepository, "image repository for the installer (external)")
	flag.StringVar(&opts.InstallerInternalRepository, "installer-internal-repository", cmd.DefaultOptions.InstallerInternalRepository, "image repository for the installer (internal)")

	flag.Parse()

	return opts
}
