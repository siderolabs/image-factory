// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/siderolabs/image-factory/internal/remotewrap"
)

// Validate checks Options for inconsistencies that would otherwise produce
// invalid artifacts at runtime (e.g. malformed SPDX documentNamespace).
func (o *Options) Validate() error {
	if o.HTTP.ExternalURL == "" {
		return fmt.Errorf("http.externalURL is required")
	}

	u, err := url.Parse(o.HTTP.ExternalURL)
	if err != nil {
		return fmt.Errorf("http.externalURL is not a valid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("http.externalURL must have http or https scheme, got %q", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("http.externalURL must have a host, got %q", o.HTTP.ExternalURL)
	}

	if o.HTTP.ExternalPXEURL != "" {
		pu, err := url.Parse(o.HTTP.ExternalPXEURL)
		if err != nil {
			return fmt.Errorf("http.externalPXEURL is not a valid URL: %w", err)
		}

		if pu.Scheme != "http" && pu.Scheme != "https" {
			return fmt.Errorf("http.externalPXEURL must have http or https scheme, got %q", pu.Scheme)
		}

		if pu.Host == "" {
			return fmt.Errorf("http.externalPXEURL must have a host, got %q", o.HTTP.ExternalPXEURL)
		}
	}

	// 0 means "unset" (falls back to remotewrap.DefaultJobs); any explicit value must not be
	// below the default, as lower concurrency can deadlock the limiter under load.
	if o.Registry.Jobs != 0 && o.Registry.Jobs < remotewrap.DefaultJobs {
		return fmt.Errorf("registry.jobs must be >= %d if set, got %d", remotewrap.DefaultJobs, o.Registry.Jobs)
	}

	return nil
}

// Options configures the behavior of the image factory.
type Options struct { //nolint:govet // keeping order for semantic clarity
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

	// Authentication settings.
	//
	// Note: only available in the Enterprise edition.
	Authentication AuthenticationOptions `koanf:"authentication"`

	// Enterprise contains configuration for enterprise-specific features.
	Enterprise EnterpriseOptions `koanf:"enterprise"`

	// Registry contains low-level tuning for the registry client (pull/push concurrency, debugging).
	Registry RegistryOptions `koanf:"registry"`
}

// RegistryOptions tunes the shared registry client used for all pull/push operations.
type RegistryOptions struct {
	// Jobs is the maximum number of concurrent blob pull/push operations per registry client.
	//
	// go-containerregistry gates concurrent blob fetches on this value; too low a value can
	// deadlock under Image Factory's concurrent, multiplexed fetch pattern.
	// Defaults to remotewrap.DefaultJobs.
	Jobs int `koanf:"jobs"`

	// Debug tracks registry response bodies to help diagnose pull-limiter token leaks/stalls:
	// it periodically logs how many bodies are open and dumps any body that stays open too long
	// together with the stack that opened it.
	//
	// Set via config or the IF_REGISTRY_DEBUG environment variable.
	Debug bool `koanf:"debug"`
}

// AssetBuilderOptions contains settings for building assets.
type AssetBuilderOptions struct {
	// MinTalosVersion specifies the minimum supported Talos version for assets.
	MinTalosVersion string `koanf:"minTalosVersion"`

	// BrokenTalosVersions lists Talos versions that should be considered broken and avoided when building assets.
	// Those are versions that are known to have critical issues that prevent them from working correctly, such as bugs in Talos that cause build failures or runtime errors.
	BrokenTalosVersions []string `koanf:"brokenTalosVersions"`

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

	// AllowedOrigins configures the frontend API CORS with custom origins list.
	AllowedOrigins []string `koanf:"allowedOrigins"`
}

// InstallerOptions configures storage for installer images, including internal and external OCI repositories.
type InstallerOptions struct {
	// Internal is the internal OCI registry used by the image factory to push installer images.
	Internal OCIRepositoryOptions `koanf:"internal"`

	// External is the public OCI registry used for redirects to installer images.
	//
	// If this field is not set, Image Factory will proxy requests to the internal registry
	// through itself instead of issuing HTTP redirects to the external registry endpoint.
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

// Image returns the Namespace + Repository string (without the registry).
func (o *OCIRepositoryOptions) Image() string {
	parts := []string{}

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
	// Mutually exclusive with GSA signing.
	SigningKeyPath string `koanf:"signingKeyPath"`

	// GSA contains configuration for Google Service Account keyless signing via Sigstore.
	// When set, GSA-based keyless signing is used instead of a static key.
	// Mutually exclusive with SigningKeyPath.
	GSA GSASigningOptions `koanf:"gsa"`

	// CDN contains configuration for using a CDN to serve cached assets.
	CDN CDNCacheOptions `koanf:"cdn"`

	// S3 contains configuration for using S3 to store cached assets.
	S3 S3CacheOptions `koanf:"s3"`

	// Schematic contains configuration for caching schematic blobs.
	Schematic SchematicCacheOptions `koanf:"schematic"`
}

// GSASigningOptions configures Google Service Account keyless image signing via Sigstore Fulcio.
type GSASigningOptions struct {
	// ServiceAccountEmail is the GSA email embedded in the Fulcio certificate.
	// Used for signature verification — callers must trust signatures issued for this identity.
	ServiceAccountEmail string `koanf:"serviceAccountEmail"`

	// KeyFile is the path to a service account JSON key file.
	// If empty, Application Default Credentials are used (GOOGLE_APPLICATION_CREDENTIALS
	// environment variable or the metadata server on GCE).
	KeyFile string `koanf:"keyFile"`

	// FulcioURL is the Fulcio CA endpoint.
	// Defaults to the public Sigstore instance.
	FulcioURL string `koanf:"fulcioURL"`

	// RekorURL is the Rekor transparency log endpoint.
	// Defaults to the public Sigstore instance.
	RekorURL string `koanf:"rekorURL"`
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

	// PresignedURLTTL is the duration for which presigned URLs are valid.
	PresignedURLTTL time.Duration `koanf:"presignedURLTTL"`
}

// SchematicCacheOptions configures caching of schematic blobs.
type SchematicCacheOptions struct {
	// Capacity sets the maximum number of schematics to keep in the in-memory cache.
	Capacity uint64 `koanf:"capacity"`

	// NegativeTTL sets the time-to-live for negative cache entries (schematics not found in underlying storage).
	NegativeTTL time.Duration `koanf:"negativeTTL"`
}

// ContainerSignature contains configuration for verifying container image signatures.
type ContainerSignature struct {
	// SubjectRegExp is a regular expression used to validate the subject in container signatures.
	//
	// Set explicitly to empty string to disable subject validation, otherwise it defaults to a regex that allows trusted Sidero Labs account identities.
	// This keyless verification method will not work in air-gapped environments.
	SubjectRegExp string `koanf:"subjectRegExp"`

	// IssuerRegExp is a regular expression used to validate the issuer in container signatures.
	IssuerRegExp string `koanf:"issuerRegExp"`

	// Issuer is the expected issuer for container signatures (overrides RegExp if set).
	Issuer string `koanf:"issuer"`

	// PublicKeyFile is the path to the public key used for signature verification.
	//
	// Alternative to keyless verification using SubjectRegExp and Issuer/IssuerRegExp.
	// If set, the image factory will use this public key to verify signatures instead of relying on keyless identities.
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

	// Namespace is an optional repository path prefix prepended to every image pulled from
	// the registry: both the component images below and the extension/overlay images
	// discovered via the manifests.
	//
	// Useful for pull-through caches which prefix the upstream repository path, e.g. a Harbor
	// proxy-cache project: with registry "harbor.example.com" and namespace "ghcrio",
	// "siderolabs/imager" is pulled from "harbor.example.com/ghcrio/siderolabs/imager".
	Namespace string `koanf:"namespace"`

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

// AuthenticationOptions holds authentication settings.
type AuthenticationOptions struct { //nolint:govet // keeping order for semantic clarity
	// Enabled enables authentication.
	Enabled bool `koanf:"enabled"`
	// HTPasswdPath is the path to the htpasswd file containing user credentials.
	//
	// The file follows the standard htpasswd format (username:bcrypt_hash, one per line).
	// Multiple entries with the same username are supported, allowing multiple API keys per user.
	// Only bcrypt hashes ($2y$/$2a$/$2b$) are accepted.
	//
	// It is required if authentication is enabled.
	HTPasswdPath string `koanf:"htpasswdPath"`
}

// EnterpriseOptions contains configuration for enterprise-specific features.
type EnterpriseOptions struct {
	// ExtraExtensions contains configuration for extra (custom) extensions.
	ExtraExtensions ExtraExtensionsOptions `koanf:"extraExtensions"`

	// Scanner contains configuration for the vulnerability scanner.
	Scanner ScannerOptions `koanf:"scanner"`

	// SPDX contains configuration for SPDX document generation.
	SPDX SPDXOptions `koanf:"spdx"`

	// VEX contains configuration for VEX data fetching.
	VEX VEXOptions `koanf:"vex"`
}

// ExtraExtensionsOptions configures custom extensions offered alongside the official ones.
type ExtraExtensionsOptions struct {
	// Manifest specifies the OCI repository holding the extra extensions manifest image.
	//
	// It may live in a different registry than the official images.
	Manifest OCIRepositoryOptions `koanf:"manifest"`
}

// SPDXOptions configures SPDX document generation and caching.
type SPDXOptions struct {
	Cache OCIRepositoryOptions `koanf:"cache"`
}

// VEXOptions configures VEX data caching.
type VEXOptions struct {
	// Data specifies the OCI repository where VEX documents are stored.
	Data OCIRepositoryOptions `koanf:"data"`

	// Cache contains configuration for caching VEX documents.
	Cache LRUCacheOptions `koanf:"cache"`
}

// ScannerOptions configures the vulnerability scanner endpoint.
type ScannerOptions struct {
	// DatabaseURL overrides the Grype vulnerability database listing URL.
	// Set this to point at a mirror or air-gapped database service.
	DatabaseURL string `koanf:"databaseURL"`

	// Cache contains configuration for caching vulnerability scan results.
	Cache LRUCacheOptions `koanf:"cache"`
}

// LRUCacheOptions configures caching of vulnerability scan results.
type LRUCacheOptions struct {
	// TTL is the duration for caching objects.
	TTL time.Duration `koanf:"ttl"`

	// Capacity caps the number of cached objects before LRU eviction.
	Capacity uint64 `koanf:"capacity"`
}

// DefaultOptions are the default options.
var DefaultOptions = Options{
	HTTP: HTTPOptions{
		ListenAddr:     ":8080",
		ExternalURL:    "https://localhost/",
		AllowedOrigins: []string{"*"},
	},

	Build: AssetBuilderOptions{
		MinTalosVersion: "1.2.0",
		MaxConcurrency:  6,
	},

	ContainerSignature: ContainerSignature{
		SubjectRegExp:     `(@siderolabs\.com$|^releasemgr-svc@talos-production\.iam\.gserviceaccount\.com$)`,
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
			Bucket:          "image-factory",
			PresignedURLTTL: time.Hour,
		},
		Schematic: SchematicCacheOptions{
			Capacity:    100_000,
			NegativeTTL: 30 * time.Second,
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
				Registry:  "ghcr.io",
				Namespace: "siderolabs",
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

	Enterprise: EnterpriseOptions{
		VEX: VEXOptions{
			Data: OCIRepositoryOptions{
				Registry:   "ghcr.io",
				Namespace:  "siderolabs/talos-vex",
				Repository: "talos-vex-data",
			},
			Cache: LRUCacheOptions{
				TTL:      15 * time.Minute,
				Capacity: 65536,
			},
		},
		SPDX: SPDXOptions{
			Cache: OCIRepositoryOptions{
				Registry:   "ghcr.io",
				Namespace:  "siderolabs/image-factory",
				Repository: "spdx-cache",
			},
		},
		Scanner: ScannerOptions{
			DatabaseURL: "https://grype.anchore.io/databases",
			Cache: LRUCacheOptions{
				TTL:      15 * time.Minute,
				Capacity: 4096,
			},
		},
	},

	Registry: RegistryOptions{
		Jobs: remotewrap.DefaultJobs,
	},
}
