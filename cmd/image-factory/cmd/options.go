// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"time"

	"go.uber.org/zap/zapcore"
)

// Options configures image factory.
type Options struct { //nolint:govet
	// LogLevel sets the logging level for the image factory.
	LogLevel *zapcore.Level

	// Listen address for the HTTP frontend.
	HTTPListenAddr string

	// Asset builder options: minimum supported Talos version.
	MinTalosVersion string
	// Image registry for source images: imager, extensions, etc..
	ImageRegistry string
	// Allow insecure connection to the image registry
	InsecureImageRegistry bool

	// RegistryRefreshInterval is the interval for refreshing the image registry connections.
	RegistryRefreshInterval time.Duration

	// Options to verify container signatures for imager, extensions, etc.
	ContainerSignatureSubjectRegExp     string
	ContainerSignatureIssuerRegExp      string
	ContainerSignatureIssuer            string
	ContainerSignaturePublicKeyFile     string
	ContainerSignaturePublicKeyHashAlgo string

	// Maximum number of concurrent asset builds.
	AssetBuildMaxConcurrency int

	// External URL of the image factory HTTP frontend.
	ExternalURL string
	// External URL of the image factory PXE frontend.
	ExternalPXEURL string

	// Schematic service OCI registry prefix.
	// It stores schematics for the image factory as blobs under that path.
	SchematicServiceRepository string
	// Allow insecure connection to the schematic service repository.
	InsecureSchematicRepository bool

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

	// OCI registry to use to store cached boot assets.
	// Only used internally by the image factory.
	CacheRepository string
	// Allow insecure connection to the cache repository.
	InsecureCacheRepository bool

	// CacheS3Enabled enables S3 cache for the image factory.
	CacheS3Enabled bool
	// CacheS3Bucket is the bucket name for the cache.
	CacheS3Bucket string
	// CacheS3Endpoint is the S3 endpoint for the cache.
	// It should not include the scheme or trailing slash.
	CacheS3Endpoint string
	// CacheS3Region is the S3 region for the cache.
	CacheS3Region string
	// InsecureCacheS3 allows insecure connection to the S3 storage.
	InsecureCacheS3 bool

	// CacheCDNEnabled enables CDN cache for the image factory.
	CacheCDNEnabled bool
	// CacheCDNHost is the URL of the CDN.
	CacheCDNHost string
	// CacheCDNTrimPrefix is the path to trim from the underlying redirect URL, including the leading slash.
	// For example, if asset URL is https://example.com/image-factory/cache/asset.tar.gz,
	// and CacheCDNTrimPrefix is /image-factory, then the redirect URL will be
	// https://cdn.example.com/cache/asset.tar.gz.
	// If empty, the path will not be trimmed.
	CacheCDNTrimPrefix string

	// Bind address for Prometheus metrics.
	//
	// Leave empty to disable.
	MetricsListenAddr string

	// MetricsNamespace is the namespace for Prometheus metrics.
	// It's not user-configurable, but set by the image factory tests.
	MetricsNamespace string

	// SecureBoot settings.
	SecureBoot SecureBootOptions
}

// SecureBootOptions configures SecureBoot.
type SecureBootOptions struct { //nolint:govet
	// Enable SecureBoot asset generation.
	Enabled bool

	// File-based approach.
	SigningKeyPath, SigningCertPath string
	PCRKeyPath                      string

	// Azure Key Vault approach.
	AzureKeyVaultURL     string
	AzureCertificateName string
	AzureKeyName         string

	// AWS KMS approach.
	//
	// AWS KMS Key ID and region.
	// AWS doesn't have a good way to store a certificate, so it's expected to be a file.
	AwsKMSKeyID    string
	AwsKMSPCRKeyID string
	AwsCertPath    string
	AwsRegion      string
}

// DefaultOptions are the default options.
var DefaultOptions = Options{
	HTTPListenAddr: ":8080",

	MinTalosVersion: "1.2.0",
	ImageRegistry:   "ghcr.io",

	RegistryRefreshInterval: 5 * time.Minute,

	ContainerSignatureSubjectRegExp:     `@siderolabs\.com$`,
	ContainerSignatureIssuerRegExp:      "",
	ContainerSignatureIssuer:            "https://accounts.google.com",
	ContainerSignaturePublicKeyHashAlgo: "sha256",

	AssetBuildMaxConcurrency: 6,

	ExternalURL: "https://localhost/",

	SchematicServiceRepository: "ghcr.io/siderolabs/image-factory/schematics",

	InstallerInternalRepository: "ghcr.io/siderolabs",
	InstallerExternalRepository: "ghcr.io/siderolabs",

	TalosVersionRecheckInterval: 15 * time.Minute,

	CacheRepository: "ghcr.io/siderolabs/image-factory/cache",
	CacheS3Bucket:   "image-factory",

	MetricsListenAddr: ":2122",
}
