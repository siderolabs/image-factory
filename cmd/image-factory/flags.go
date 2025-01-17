// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main is the entrypoint of the image factory.
package main

import (
	"flag"

	"github.com/skyssolutions/siderolabs-image-factory/cmd/image-factory/cmd"
)

func initFlags() cmd.Options {
	var opts cmd.Options

	flag.StringVar(&opts.HTTPListenAddr, "http-port", cmd.DefaultOptions.HTTPListenAddr, "HTTP listen address")

	flag.StringVar(&opts.MinTalosVersion, "min-talos-version", cmd.DefaultOptions.MinTalosVersion, "minimum Talos version")
	flag.StringVar(&opts.ImageRegistry, "image-registry", cmd.DefaultOptions.ImageRegistry, "image registry for imager, extensions, etc.")
	flag.BoolVar(&opts.InsecureImageRegistry, "insecure-image-registry", cmd.DefaultOptions.InsecureImageRegistry, "allow an insecure connection to the image registry")

	flag.StringVar(&opts.ContainerSignatureSubjectRegExp, "container-signature-subject-regexp", cmd.DefaultOptions.ContainerSignatureSubjectRegExp, "container signature subject regexp")
	flag.StringVar(&opts.ContainerSignatureIssuerRegExp, "container-signature-issuer-regexp", cmd.DefaultOptions.ContainerSignatureIssuerRegExp, "container signature issuer regexp")
	flag.StringVar(&opts.ContainerSignatureIssuer, "container-signature-issuer", cmd.DefaultOptions.ContainerSignatureIssuer, "container signature issuer")
	flag.StringVar(&opts.ContainerSignaturePublicKeyFile, "container-signature-pubkey", cmd.DefaultOptions.ContainerSignaturePublicKeyFile, "container signature public key (optional)")
	flag.StringVar(&opts.ContainerSignaturePublicKeyHashAlgo, "container-signature-pubkey-hashalgo", cmd.DefaultOptions.ContainerSignaturePublicKeyHashAlgo, "hash algo of the container signature public key (optional)") //nolint:lll

	flag.IntVar(&opts.AssetBuildMaxConcurrency, "asset-builder-max-concurrency", cmd.DefaultOptions.AssetBuildMaxConcurrency, "maximum concurrency for asset builder")

	flag.StringVar(&opts.ExternalURL, "external-url", cmd.DefaultOptions.ExternalURL, "factory external endpoint URL")
	flag.StringVar(&opts.ExternalPXEURL, "external-pxe-url", cmd.DefaultOptions.ExternalPXEURL, "factory external PXE endpoint URL, if not set defaults to --external-url")

	flag.StringVar(&opts.SchematicServiceRepository, "schematic-service-repository", cmd.DefaultOptions.SchematicServiceRepository, "image repository for the schematic service")
	flag.BoolVar(
		&opts.InsecureSchematicRepository,
		"insecure-schematic-service-repository",
		cmd.DefaultOptions.InsecureSchematicRepository,
		"allow an insecure connection to the schematics repository",
	)

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
	flag.BoolVar(
		&opts.InsecureCacheRepository,
		"insecure-cache-repository",
		cmd.DefaultOptions.InsecureCacheRepository,
		"allow an insecure connection to the cache repository",
	)

	flag.StringVar(&opts.MetricsListenAddr, "metrics-listen-addr", cmd.DefaultOptions.MetricsListenAddr, "metrics listen address (set empty to disable)")

	flag.BoolVar(&opts.SecureBoot.Enabled, "secureboot", cmd.DefaultOptions.SecureBoot.Enabled, "enable Secure Boot asset generation")

	flag.StringVar(&opts.SecureBoot.SigningKeyPath, "secureboot-signing-key-path", cmd.DefaultOptions.SecureBoot.SigningKeyPath, "Secure Boot signing key path (use local PKI)")
	flag.StringVar(&opts.SecureBoot.SigningCertPath, "secureboot-signing-cert-path", cmd.DefaultOptions.SecureBoot.SigningCertPath, "Secure Boot signing certificate path (use local PKI)")
	flag.StringVar(&opts.SecureBoot.PCRKeyPath, "secureboot-pcr-key-path", cmd.DefaultOptions.SecureBoot.PCRKeyPath, "Secure Boot PCR key path (use local PKI)")

	flag.StringVar(&opts.SecureBoot.AzureKeyVaultURL, "secureboot-azure-key-vault-url", cmd.DefaultOptions.SecureBoot.AzureKeyVaultURL, "Secure Boot Azure Key Vault URL (use Azure PKI)")
	flag.StringVar(&opts.SecureBoot.AzureCertificateName, "secureboot-azure-certificate-name", cmd.DefaultOptions.SecureBoot.AzureCertificateName, "Secure Boot Azure Key Vault certificate name (use Azure PKI)") //nolint:lll
	flag.StringVar(&opts.SecureBoot.AzureKeyName, "secureboot-azure-key-name", cmd.DefaultOptions.SecureBoot.AzureKeyName, "Secure Boot Azure Key Vault PCR key name (use Azure PKI)")

	flag.Parse()

	return opts
}
