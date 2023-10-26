// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main is the entrypoint of the image factory.
package main

import (
	"flag"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

func initFlags() cmd.Options {
	var opts cmd.Options

	flag.StringVar(&opts.HTTPListenAddr, "http-port", cmd.DefaultOptions.HTTPListenAddr, "HTTP listen address")

	flag.StringVar(&opts.MinTalosVersion, "min-talos-version", cmd.DefaultOptions.MinTalosVersion, "minimum Talos version")
	flag.StringVar(&opts.ImageRegistry, "image-registry", cmd.DefaultOptions.ImageRegistry, "image registry for imager, extensions, etc.")
	flag.BoolVar(&opts.InsecureImageRegistry, "insecure-image-registry", cmd.DefaultOptions.InsecureImageRegistry, "allow an insecure connection to the image registry")

	flag.StringVar(&opts.ContainerSignatureSubjectRegExp, "container-signature-subject-regexp", cmd.DefaultOptions.ContainerSignatureSubjectRegExp, "container signature subject regexp")
	flag.StringVar(&opts.ContainerSignatureIssuer, "container-signature-issuer", cmd.DefaultOptions.ContainerSignatureIssuer, "container signature issuer")

	flag.IntVar(&opts.AssetBuildMaxConcurrency, "asset-builder-max-concurrency", cmd.DefaultOptions.AssetBuildMaxConcurrency, "maximum concurrency for asset builder")

	flag.StringVar(&opts.ExternalURL, "external-url", cmd.DefaultOptions.ExternalURL, "factory external endpoint URL")

	flag.StringVar(&opts.SchematicServiceRepository, "schematic-service-repository", cmd.DefaultOptions.SchematicServiceRepository, "image repository for the schematic service")

	flag.StringVar(&opts.InstallerExternalRepository, "installer-external-repository", cmd.DefaultOptions.InstallerExternalRepository, "image repository for the installer (external)")
	flag.StringVar(&opts.InstallerInternalRepository, "installer-internal-repository", cmd.DefaultOptions.InstallerInternalRepository, "image repository for the installer (internal)")
	flag.BoolVar(
		&opts.InsecureInstallerInternalRepository,
		"insecure-installer-internal-repository",
		cmd.DefaultOptions.InsecureInstallerInternalRepository,
		"allow an insecure connection to the image repository for the installer (internal)",
	)

	flag.DurationVar(&opts.TalosVersionRecheckInterval, "talos-versions-recheck-interval", cmd.DefaultOptions.TalosVersionRecheckInterval, "interval to recheck Talos versions")

	flag.StringVar(&opts.CacheSigningKeyPath, "cache-signing-key-path", cmd.DefaultOptions.CacheSigningKeyPath, "path to the default cache signing key (PEM-encoded, ECDSA private key)")

	flag.StringVar(&opts.CacheRepository, "cache-repository", cmd.DefaultOptions.CacheRepository, "cache repository for boot assets")

	flag.Parse()

	return opts
}
