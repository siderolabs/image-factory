// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"strings"
	"time"
)

// Options configures the behavior of the image factory.
type Options struct {
	// HTTP configuration for the image factory frontend.
	HTTP HTTPOptions `koanf:"http"`

	// Options for building assets used in images, including concurrency and Talos version constraints.
	Build AssetBuilderOptions `koanf:"build"`

	// ContainerSignature holds configuration for verifying container image signatures.
	ContainerSignature ContainerSignature `koanf:"containerSignature"`

	// Cache contains configuration for storing and retrieving boot assets.
	Cache CacheOptions `koanf:"cache"`

	// Metrics holds configuration for the Prometheus metrics endpoint.
	Metrics MetricsOptions `koanf:"metrics"`

	// SecureBoot contains configuration for generating SecureBoot-enabled assets.
	SecureBoot SecureBootOptions `koanf:"secureBoot"`

	// Artifacts defines names and references for various images used by the factory.
	Artifacts ArtifactsOptions `koanf:"artifacts"`
}

// AssetBuilderOptions contains settings for building assets.
type AssetBuilderOptions struct {
	// MinTalosVersion specifies the minimum supported Talos version for assets.
	MinTalosVersion string `koanf:"minTalosVersion"`

	// MaxConcurrency sets the maximum number of simultaneous asset build operations.
	MaxConcurrency int `koanf:"maxConcurrency"`
}

// HTTPOptions configures the HTTP frontend of the image factory.
type HTTPOptions struct {
	// ListenAddr is the local address to bind the HTTP frontend to.
	ListenAddr string `koanf:"httpListenAddr"`

	// CertFile is the path to the TLS certificate for the HTTP frontend (optional).
	CertFile string `koanf:"certFile"`

	// KeyFile is the path to the TLS key for the HTTP frontend (optional).
	KeyFile string `koanf:"keyFile"`

	// ExternalURL is the public URL for the image factory HTTP frontend, used in links and redirects.
	ExternalURL string `koanf:"externalURL"`

	// ExternalPXEURL is the public URL for the PXE frontend, used for booting nodes via PXE.
	ExternalPXEURL string `koanf:"externalPXEURL"`
}

// InstallerOptions configures storage for installer images, including internal and external OCI repositories.
type InstallerOptions struct {
	// Internal is the internal OCI registry used by the image factory to push installer images.
	Internal OCIRepositoryOptions `koanf:"internal"`

	// External is the public OCI registry used for redirects to installer images.
	External OCIRepositoryOptions `koanf:"external"`
}

// OCIRepositoryOptions contains configuration for connecting to a container image repository.
//
// It separates the registry host, the namespace/organization, and the repository name.
//
//	<Registry>/<Namespace>/<Repository>
type OCIRepositoryOptions struct {
	// Registry is the hostname of the container registry, e.g., `ghcr.io`.
	// This is where images are stored.
	Registry string `koanf:"registry"`

	// Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
	// Some registries allow repositories without a namespace.
	Namespace string `koanf:"namespace"`

	// Repository is the name of the repository inside the namespace, e.g., `talos`.
	// Combined with Registry and Namespace, it forms the fully qualified repository path.
	Repository string `koanf:"repository"`

	// Insecure allows connections to registries over HTTP or with invalid TLS certificates.
	Insecure bool `koanf:"insecure"`
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (o *OCIRepositoryOptions) UnmarshalText(text []byte) error {
	input := string(text)

	if input == "" {
		return nil
	}

	parts := strings.Split(input, "/")

	switch len(parts) {
	case 1:
		// e.g. "nginx"
		o.Registry = ""
		o.Namespace = ""
		o.Repository = parts[0]
	case 2:
		// e.g. "library/golang" or "127.0.0.1:5000/nginx"
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			// first part is registry
			o.Registry = parts[0]
			o.Namespace = ""
			o.Repository = parts[1]
		} else {
			o.Registry = ""
			o.Namespace = parts[0]
			o.Repository = parts[1]
		}
	default:
		// more than 2 parts
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			// first part is registry
			o.Registry = parts[0]
			o.Namespace = strings.Join(parts[1:len(parts)-1], "/")
			o.Repository = parts[len(parts)-1]
		} else {
			// no registry
			o.Registry = ""
			o.Namespace = strings.Join(parts[:len(parts)-1], "/")
			o.Repository = parts[len(parts)-1]
		}
	}

	return nil
}

// String returns the repository path by joining non-empty parts with "/".
// It gracefully handles missing Registry, Namespace, or Repository.
func (o *OCIRepositoryOptions) String() string {
	parts := []string{}

	if o.Registry != "" {
		parts = append(parts, o.Registry)
	}

	if o.Namespace != "" {
		parts = append(parts, o.Namespace)
	}

	if o.Repository != "" {
		parts = append(parts, o.Repository)
	}

	return strings.Join(parts, "/")
}

// MetricsOptions holds configuration for exposing Prometheus metrics.
type MetricsOptions struct {
	// Addr is the bind address for the metrics HTTP server.
	// Leave empty to disable metrics.
	Addr string `koanf:"addr"`

	// Namespace sets the namespace for Prometheus metrics (not user-configurable, set by tests).
	Namespace string `koanf:"-"`
}

// CacheOptions configures caching of boot assets in the image factory.
type CacheOptions struct {
	// OCI contains configuration for using OCI Registry to store cached assets.
	// This configuration is required.
	OCI OCIRepositoryOptions `koanf:"oci"`

	// SigningKeyPath is the path to the ECDSA key used to sign cached assets.
	SigningKeyPath string `koanf:"signingKeyPath"`

	// CDN contains configuration for using a CDN to serve cached assets.
	CDN CDNCacheOptions `koanf:"cdn"`

	// S3 contains configuration for using S3 to store cached assets.
	S3 S3CacheOptions `koanf:"s3"`
}

// CDNCacheOptions configures CDN-based cache for the image factory.
type CDNCacheOptions struct {
	// Host is the CDN URL used to serve cached assets.
	Host string `koanf:"host"`

	// TrimPrefix removes a prefix from asset paths before redirecting to the CDN.
	TrimPrefix string `koanf:"trimPrefix"`

	// Enabled enables the CDN cache.
	Enabled bool `koanf:"enabled"`
}

// S3CacheOptions configures S3-based cache for the image factory.
type S3CacheOptions struct {
	// Bucket is the S3 bucket name where cached assets are stored.
	Bucket string `koanf:"bucket"`

	// Endpoint is the S3 endpoint URL (without scheme or trailing slash).
	Endpoint string `koanf:"endpoint"`

	// Region is the S3 region for the bucket.
	Region string `koanf:"region"`

	// Insecure allows connecting to S3 without TLS or with invalid certificates.
	Insecure bool `koanf:"insecure"`

	// Enabled enables S3 cache.
	Enabled bool `koanf:"enabled"`
}

// ContainerSignature contains configuration for verifying container image signatures.
type ContainerSignature struct {
	// SubjectRegExp is a regular expression used to validate the subject in container signatures.
	SubjectRegExp string `koanf:"subjectRegExp"`

	// IssuerRegExp is a regular expression used to validate the issuer in container signatures.
	IssuerRegExp string `koanf:"issuerRegExp"`

	// Issuer is the expected issuer for container signatures (overrides RegExp if set).
	Issuer string `koanf:"issuer"`

	// PublicKeyFile is the path to the public key used for signature verification.
	PublicKeyFile string `koanf:"publicKeyFile"`

	// PublicKeyHashAlgo specifies the hash algorithm used for verifying the public key.
	PublicKeyHashAlgo string `koanf:"publicKeyHashAlgo"`

	// Disabled disables signature verification.
	Disabled bool `koanf:"disabled"`
}

// SecureBootOptions configures generation of SecureBoot-enabled assets.
type SecureBootOptions struct {
	// File specifies file-based SecureBoot keys and certificates.
	File FileProviderOptions `koanf:"file"`

	// AzureKeyVault configures SecureBoot using Azure Key Vault.
	AzureKeyVault AzureKeyVaultProviderOptions `koanf:"azureKeyVault"`

	// AWSKMS configures SecureBoot using AWS KMS.
	AWSKMS AWSKMSProviderOptions `koanf:"awsKMS"`

	// Enabled enables SecureBoot asset generation.
	Enabled bool `koanf:"enabled"`
}

// FileProviderOptions configures file-based SecureBoot keys and certificates.
type FileProviderOptions struct {
	// SigningKeyPath is the path to the private key used for signing boot assets.
	SigningKeyPath string `koanf:"signingKeyPath"`

	// SigningCertPath is the path to the certificate used for signing boot assets.
	SigningCertPath string `koanf:"signingCertPath"`

	// PCRKeyPath is the path to the key used for PCR measurement.
	PCRKeyPath string `koanf:"pcrKeyPath"`
}

// AzureKeyVaultProviderOptions configures SecureBoot keys and certificates in Azure Key Vault.
type AzureKeyVaultProviderOptions struct {
	// URL is the Key Vault endpoint.
	URL string `koanf:"url"`

	// CertificateName is the name of the certificate in Key Vault.
	CertificateName string `koanf:"certificateName"`

	// KeyName is the name of the key in Key Vault.
	KeyName string `koanf:"keyName"`
}

// AWSKMSProviderOptions configures SecureBoot using AWS KMS keys and certificates.
type AWSKMSProviderOptions struct {
	// KeyID is the AWS KMS Key ID used for signing boot assets.
	KeyID string `koanf:"keyID"`

	// PCRKeyID is the AWS KMS Key ID used for PCR measurement.
	PCRKeyID string `koanf:"pcrKeyID"`

	// CertPath is the path to the certificate used with AWS KMS.
	CertPath string `koanf:"certPath"`

	// CertARN is the ARN of the ACM certificate used with AWS KMS.
	CertARN string `koanf:"certARN"`

	// Region is the AWS region containing the KMS keys.
	Region string `koanf:"region"`
}

// ArtifactsOptions defines the names and references of images used by the image factory.
type ArtifactsOptions struct {
	// Core contains configuration for core images used by the image factory.
	Core CoreImagesOptions `koanf:"core"`

	// Schematic is the OCI repository used to store schematic blobs required by the image factory for building images.
	Schematic OCIRepositoryOptions `koanf:"schematic"`

	// Installer contains configuration for storing and accessing installer images.
	Installer InstallerOptions `koanf:"installer"`

	// TalosVersionRecheckInterval sets the interval at which the image factory rechecks available Talos versions.
	TalosVersionRecheckInterval time.Duration `koanf:"talosVersionRecheckInterval"`

	// RefreshInterval specifies how often the image factory should refresh its connection to registries.
	RefreshInterval time.Duration `koanf:"refreshInterval"`
}

// CoreImagesOptions defines the configuration for core images used by the image factory.
type CoreImagesOptions struct {
	// Registry specifies the OCI registry host for base images, extensions, and related artifacts.
	// E.g., "ghcr.io".
	Registry string `koanf:"registry"`

	// Components defines the names of images used by the image factory.
	// This typically maps to repositories and tags for core components.
	Components ComponentsOptions `koanf:"components"`

	// Insecure allows connections to the registry over HTTP or with invalid TLS certificates.
	// Use with caution, as this may expose security risks.
	Insecure bool `koanf:"insecure"`
}

// ComponentsOptions defines the names of images used by the image factory.
type ComponentsOptions struct {
	// InstallerBase is the base image for creating installer images.
	InstallerBase string `koanf:"installerBase"`

	// Installer is the main installer image.
	Installer string `koanf:"installer"`

	// Imager is the image builder used by the factory.
	Imager string `koanf:"imager"`

	// ExtensionManifest is the image manifest for extensions.
	ExtensionManifest string `koanf:"extensionManifest"`

	// OverlayManifest is the image manifest for overlays.
	OverlayManifest string `koanf:"overlayManifest"`

	// Talosctl is the image containing the Talos CLI tool.
	Talosctl string `koanf:"talosctl"`
}

// DefaultOptions are the default options.
var DefaultOptions = Options{
	HTTP: HTTPOptions{
		ListenAddr:  ":8080",
		ExternalURL: "https://localhost/",
	},

	Build: AssetBuilderOptions{
		MinTalosVersion: "1.2.0",
		MaxConcurrency:  6,
	},

	ContainerSignature: ContainerSignature{
		SubjectRegExp:     `@siderolabs\.com$`,
		IssuerRegExp:      "",
		Issuer:            "https://accounts.google.com",
		PublicKeyHashAlgo: "sha256",
	},

	Cache: CacheOptions{
		OCI: OCIRepositoryOptions{
			Registry:   "ghcr.io",
			Namespace:  "siderolabs/image-factory",
			Repository: "cache",
		},
		S3: S3CacheOptions{
			Bucket: "image-factory",
		},
	},

	Metrics: MetricsOptions{
		Addr: ":2122",
	},

	Artifacts: ArtifactsOptions{
		TalosVersionRecheckInterval: 15 * time.Minute,
		RefreshInterval:             5 * time.Minute,

		Schematic: OCIRepositoryOptions{
			Registry:   "ghcr.io",
			Namespace:  "siderolabs/image-factory",
			Repository: "schematics",
		},

		Installer: InstallerOptions{
			Internal: OCIRepositoryOptions{
				Registry:   "ghcr.io",
				Repository: "siderolabs",
			},
		},

		Core: CoreImagesOptions{
			Registry: "ghcr.io",
			Components: ComponentsOptions{
				InstallerBase:     "siderolabs/installer-base",
				Installer:         "siderolabs/installer",
				Imager:            "siderolabs/imager",
				ExtensionManifest: "siderolabs/extensions",
				OverlayManifest:   "siderolabs/overlays",
				Talosctl:          "siderolabs/talosctl-all",
			},
		},
	},
}
