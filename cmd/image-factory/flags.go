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
	opts := cmd.DefaultOptions

	flag.StringVar(&opts.HTTPListenAddr, "http-port", cmd.DefaultOptions.HTTPListenAddr, "HTTP listen address")

	flag.StringVar(&opts.MinTalosVersion, "min-talos-version", cmd.DefaultOptions.MinTalosVersion, "minimum Talos version")
	flag.StringVar(&opts.ImageRegistry, "image-registry", cmd.DefaultOptions.ImageRegistry, "image registry for imager, extensions, etc.")
	flag.BoolVar(&opts.InsecureImageRegistry, "insecure-image-registry", cmd.DefaultOptions.InsecureImageRegistry, "allow an insecure connection to the image registry")

	flag.DurationVar(&opts.RegistryRefreshInterval, "registry-refresh-interval", cmd.DefaultOptions.RegistryRefreshInterval, "image registry refresh interval")

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

	flag.BoolVar(&opts.CacheS3Enabled, "cache-s3-enabled", cmd.DefaultOptions.CacheS3Enabled, "enable S3 cache for boot assets")
	flag.StringVar(&opts.CacheS3Bucket, "cache-s3-bucket", cmd.DefaultOptions.CacheS3Bucket, "S3 bucket for the cache")
	flag.StringVar(&opts.CacheS3Region, "cache-s3-region", cmd.DefaultOptions.CacheS3Region, "S3 region for the cache")
	flag.StringVar(&opts.CacheS3Endpoint, "cache-s3-endpoint", cmd.DefaultOptions.CacheS3Endpoint, "S3 endpoint for the cache")
	flag.BoolVar(&opts.InsecureCacheS3, "insecure-cache-s3", cmd.DefaultOptions.InsecureCacheS3, "use insecure S3 connection for the cache")

	flag.BoolVar(&opts.CacheCDNEnabled, "cache-cdn-enabled", cmd.DefaultOptions.CacheCDNEnabled, "enable CDN for boot assets")
	flag.StringVar(&opts.CacheCDNHost, "cache-cdn-host", cmd.DefaultOptions.CacheCDNHost, "CDN host for the cache")
	flag.StringVar(&opts.CacheCDNTrimPrefix, "cache-cdn-trim-prefix", cmd.DefaultOptions.CacheCDNTrimPrefix, "CDN trim path for the cache")

	flag.StringVar(&opts.MetricsListenAddr, "metrics-listen-addr", cmd.DefaultOptions.MetricsListenAddr, "metrics listen address (set empty to disable)")

	flag.BoolVar(&opts.SecureBoot.Enabled, "secureboot", cmd.DefaultOptions.SecureBoot.Enabled, "enable Secure Boot asset generation")

	flag.StringVar(&opts.SecureBoot.SigningKeyPath, "secureboot-signing-key-path", cmd.DefaultOptions.SecureBoot.SigningKeyPath, "Secure Boot signing key path (use local PKI)")
	flag.StringVar(&opts.SecureBoot.SigningCertPath, "secureboot-signing-cert-path", cmd.DefaultOptions.SecureBoot.SigningCertPath, "Secure Boot signing certificate path (use local PKI)")
	flag.StringVar(&opts.SecureBoot.PCRKeyPath, "secureboot-pcr-key-path", cmd.DefaultOptions.SecureBoot.PCRKeyPath, "Secure Boot PCR key path (use local PKI)")

	flag.StringVar(&opts.SecureBoot.AwsKMSKeyID, "secureboot-aws-kms-id-key-id", cmd.DefaultOptions.SecureBoot.AwsKMSKeyID, "Secure Boot signing key AWS KMS ID")
	flag.StringVar(&opts.SecureBoot.AwsRegion, "secureboot-aws-region", cmd.DefaultOptions.SecureBoot.AwsRegion, "Secure Boot AWS region for KMS access")
	flag.StringVar(&opts.SecureBoot.AwsCertPath, "secureboot-aws-cert-path", cmd.DefaultOptions.SecureBoot.AwsCertPath, "Secure Boot signing certificate path")
	flag.StringVar(&opts.SecureBoot.AwsKMSPCRKeyID, "secureboot-aws-pcr-kms-key-id", cmd.DefaultOptions.SecureBoot.PCRKeyPath, "Secure Boot PCR key AWS KMS ID")

	flag.StringVar(&opts.SecureBoot.AzureKeyVaultURL, "secureboot-azure-key-vault-url", cmd.DefaultOptions.SecureBoot.AzureKeyVaultURL, "Secure Boot Azure Key Vault URL (use Azure PKI)")
	flag.StringVar(&opts.SecureBoot.AzureCertificateName, "secureboot-azure-certificate-name", cmd.DefaultOptions.SecureBoot.AzureCertificateName, "Secure Boot Azure Key Vault certificate name (use Azure PKI)") //nolint:lll
	flag.StringVar(&opts.SecureBoot.AzureKeyName, "secureboot-azure-key-name", cmd.DefaultOptions.SecureBoot.AzureKeyName, "Secure Boot Azure Key Vault PCR key name (use Azure PKI)")

	flag.Parse()

	return opts
}
