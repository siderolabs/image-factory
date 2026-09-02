# Configuration

## CLI Usage

```console
Usage:
  image-factory [flags]              Run the factory.

Flags:
      --config configs    Configuration source(s). Can be specified multiple times or as a comma-separated list.
                          Supported forms:
                            env=[PREFIX]        Load configuration from environment variables (optional prefix).
                            FILE                Load configuration from a file; format is inferred from extension.
                            file=FILE           Explicit file source (same as FILE).
                          
                          Supported file extensions:
                            .json               JSON
                            .yaml, .yml         YAML
                            .env                dotenv
                          
                          Sources are applied in the order provided; later values override earlier ones.
                          A default is always applied, regardless of whether --config is specified. (default env=IF_)
      --log-level level   Log level [debug info warn error dpanic panic fatal] (default info)
```

## Configuration Reference

Documentation for basic configuration parameters.

### `http`

HTTP configuration for the image factory frontend.

---

### `http.httpListenAddr`

- **Type:** `string`
- **Env:** `HTTP_HTTPLISTENADDR`

ListenAddr is the local address to bind the HTTP frontend to.

---

### `http.certFile`

- **Type:** `string`
- **Env:** `HTTP_CERTFILE`

CertFile is the path to the TLS certificate for the HTTP frontend (optional).

---

### `http.keyFile`

- **Type:** `string`
- **Env:** `HTTP_KEYFILE`

KeyFile is the path to the TLS key for the HTTP frontend (optional).

---

### `http.externalURL`

- **Type:** `string`
- **Env:** `HTTP_EXTERNALURL`

ExternalURL is the public URL for the image factory HTTP frontend, used in links and redirects.

---

### `http.externalPXEURL`

- **Type:** `string`
- **Env:** `HTTP_EXTERNALPXEURL`

ExternalPXEURL is the public URL for the PXE frontend, used for booting nodes via PXE.

---

### `http.allowedOrigins`

- **Type:** `[]string`
- **Env:** `HTTP_ALLOWEDORIGINS`

AllowedOrigins configures the frontend API CORS with custom origins list.

---

### `build`

Options for building assets used in images, including concurrency and Talos version constraints.

---

### `build.minTalosVersion`

- **Type:** `string`
- **Env:** `BUILD_MINTALOSVERSION`

MinTalosVersion specifies the minimum supported Talos version for assets.

---

### `build.brokenTalosVersions`

- **Type:** `[]string`
- **Env:** `BUILD_BROKENTALOSVERSIONS`

BrokenTalosVersions lists Talos versions that should be considered broken and avoided when building assets.
Those are versions that are known to have critical issues that prevent them from working correctly, such as bugs in Talos that cause build failures or runtime errors.

---

### `build.maxConcurrency`

- **Type:** `int`
- **Env:** `BUILD_MAXCONCURRENCY`

MaxConcurrency sets the maximum number of simultaneous asset build operations.

---

### `containerSignature`

ContainerSignature holds configuration for verifying container image signatures.

---

### `containerSignature.subjectRegExp`

- **Type:** `string`
- **Env:** `CONTAINERSIGNATURE_SUBJECTREGEXP`

SubjectRegExp is a regular expression used to validate the subject in container signatures.

Set explicitly to empty string to disable subject validation, otherwise it defaults to a regex that allows trusted Sidero Labs account identities.
This keyless verification method will not work in air-gapped environments.

---

### `containerSignature.issuerRegExp`

- **Type:** `string`
- **Env:** `CONTAINERSIGNATURE_ISSUERREGEXP`

IssuerRegExp is a regular expression used to validate the issuer in container signatures.

---

### `containerSignature.issuer`

- **Type:** `string`
- **Env:** `CONTAINERSIGNATURE_ISSUER`

Issuer is the expected issuer for container signatures (overrides RegExp if set).

---

### `containerSignature.publicKeyFile`

- **Type:** `string`
- **Env:** `CONTAINERSIGNATURE_PUBLICKEYFILE`

PublicKeyFile is the path to the public key used for signature verification.

Alternative to keyless verification using SubjectRegExp and Issuer/IssuerRegExp.
If set, the image factory will use this public key to verify signatures instead of relying on keyless identities.

---

### `containerSignature.publicKeyHashAlgo`

- **Type:** `string`
- **Env:** `CONTAINERSIGNATURE_PUBLICKEYHASHALGO`

PublicKeyHashAlgo specifies the hash algorithm used for verifying the public key.

---

### `containerSignature.disabled`

- **Type:** `bool`
- **Env:** `CONTAINERSIGNATURE_DISABLED`

Disabled disables signature verification.

---

### `cache`

Cache contains configuration for storing and retrieving boot assets.

---

### `cache.oci`

OCI contains configuration for using OCI Registry to store cached assets.
This configuration is required.

---

### `cache.oci.registry`

- **Type:** `string`
- **Env:** `CACHE_OCI_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `cache.oci.namespace`

- **Type:** `string`
- **Env:** `CACHE_OCI_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `cache.oci.repository`

- **Type:** `string`
- **Env:** `CACHE_OCI_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `cache.oci.insecure`

- **Type:** `bool`
- **Env:** `CACHE_OCI_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `cache.signingKeyPath`

- **Type:** `string`
- **Env:** `CACHE_SIGNINGKEYPATH`

SigningKeyPath is the path to the ECDSA key used to sign cached assets.
Mutually exclusive with GSA signing.

---

### `cache.gsa`

GSA contains configuration for Google Service Account keyless signing via Sigstore.
When set, GSA-based keyless signing is used instead of a static key.
Mutually exclusive with SigningKeyPath.

---

### `cache.gsa.serviceAccountEmail`

- **Type:** `string`
- **Env:** `CACHE_GSA_SERVICEACCOUNTEMAIL`

ServiceAccountEmail is the GSA email embedded in the Fulcio certificate.
Used for signature verification — callers must trust signatures issued for this identity.

---

### `cache.gsa.keyFile`

- **Type:** `string`
- **Env:** `CACHE_GSA_KEYFILE`

KeyFile is the path to a service account JSON key file.
If empty, Application Default Credentials are used (GOOGLE_APPLICATION_CREDENTIALS
environment variable or the metadata server on GCE).

---

### `cache.gsa.fulcioURL`

- **Type:** `string`
- **Env:** `CACHE_GSA_FULCIOURL`

FulcioURL is the Fulcio CA endpoint.
Defaults to the public Sigstore instance.

---

### `cache.gsa.rekorURL`

- **Type:** `string`
- **Env:** `CACHE_GSA_REKORURL`

RekorURL is the Rekor transparency log endpoint.
It must point at a Rekor v2 (tile-backed) log; Rekor v1 is no longer supported for signing.
Setting it requires TSAURL as well, since Rekor v2 does not timestamp entries.
If FulcioURL, RekorURL and TSAURL are all empty, the Sigstore public-good signing config is fetched from TUF.

---

### `cache.gsa.tsaURL`

- **Type:** `string`
- **Env:** `CACHE_GSA_TSAURL`

TSAURL is the RFC3161 timestamp authority endpoint.
Required whenever RekorURL is set.

---

### `cache.cdn`

CDN contains configuration for using a CDN to serve cached assets.

---

### `cache.cdn.host`

- **Type:** `string`
- **Env:** `CACHE_CDN_HOST`

Host is the CDN URL used to serve cached assets.

---

### `cache.cdn.trimPrefix`

- **Type:** `string`
- **Env:** `CACHE_CDN_TRIMPREFIX`

TrimPrefix removes a prefix from asset paths before redirecting to the CDN.

---

### `cache.cdn.enabled`

- **Type:** `bool`
- **Env:** `CACHE_CDN_ENABLED`

Enabled enables the CDN cache.

---

### `cache.s3`

S3 contains configuration for using S3 to store cached assets.

---

### `cache.s3.bucket`

- **Type:** `string`
- **Env:** `CACHE_S3_BUCKET`

Bucket is the S3 bucket name where cached assets are stored.

---

### `cache.s3.endpoint`

- **Type:** `string`
- **Env:** `CACHE_S3_ENDPOINT`

Endpoint is the S3 endpoint URL (without scheme or trailing slash).

---

### `cache.s3.region`

- **Type:** `string`
- **Env:** `CACHE_S3_REGION`

Region is the S3 region for the bucket.

---

### `cache.s3.insecure`

- **Type:** `bool`
- **Env:** `CACHE_S3_INSECURE`

Insecure allows connecting to S3 without TLS or with invalid certificates.

---

### `cache.s3.enabled`

- **Type:** `bool`
- **Env:** `CACHE_S3_ENABLED`

Enabled enables S3 cache.

---

### `cache.s3.presignedURLTTL`

- **Type:** `time.Duration`
- **Env:** `CACHE_S3_PRESIGNEDURLTTL`

PresignedURLTTL is the duration for which presigned URLs are valid.

---

### `cache.schematic`

Schematic contains configuration for caching schematic blobs.

---

### `cache.schematic.capacity`

- **Type:** `uint64`
- **Env:** `CACHE_SCHEMATIC_CAPACITY`

Capacity sets the maximum number of schematics to keep in the in-memory cache.

---

### `cache.schematic.negativeTTL`

- **Type:** `time.Duration`
- **Env:** `CACHE_SCHEMATIC_NEGATIVETTL`

NegativeTTL sets the time-to-live for negative cache entries (schematics not found in underlying storage).

---

### `metrics`

Metrics holds configuration for the Prometheus metrics endpoint.

---

### `metrics.addr`

- **Type:** `string`
- **Env:** `METRICS_ADDR`

Addr is the bind address for the metrics HTTP server.
Leave empty to disable metrics.

---

### `secureBoot`

SecureBoot contains configuration for generating SecureBoot-enabled assets.

---

### `secureBoot.file`

File specifies file-based SecureBoot keys and certificates.

---

### `secureBoot.file.signingKeyPath`

- **Type:** `string`
- **Env:** `SECUREBOOT_FILE_SIGNINGKEYPATH`

SigningKeyPath is the path to the private key used for signing boot assets.

---

### `secureBoot.file.signingCertPath`

- **Type:** `string`
- **Env:** `SECUREBOOT_FILE_SIGNINGCERTPATH`

SigningCertPath is the path to the certificate used for signing boot assets.

---

### `secureBoot.file.pcrKeyPath`

- **Type:** `string`
- **Env:** `SECUREBOOT_FILE_PCRKEYPATH`

PCRKeyPath is the path to the key used for PCR measurement.

---

### `secureBoot.azureKeyVault`

AzureKeyVault configures SecureBoot using Azure Key Vault.

---

### `secureBoot.azureKeyVault.url`

- **Type:** `string`
- **Env:** `SECUREBOOT_AZUREKEYVAULT_URL`

URL is the Key Vault endpoint.

---

### `secureBoot.azureKeyVault.certificateName`

- **Type:** `string`
- **Env:** `SECUREBOOT_AZUREKEYVAULT_CERTIFICATENAME`

CertificateName is the name of the certificate in Key Vault.

---

### `secureBoot.azureKeyVault.keyName`

- **Type:** `string`
- **Env:** `SECUREBOOT_AZUREKEYVAULT_KEYNAME`

KeyName is the name of the key in Key Vault.

---

### `secureBoot.awsKMS`

AWSKMS configures SecureBoot using AWS KMS.

---

### `secureBoot.awsKMS.keyID`

- **Type:** `string`
- **Env:** `SECUREBOOT_AWSKMS_KEYID`

KeyID is the AWS KMS Key ID used for signing boot assets.

---

### `secureBoot.awsKMS.pcrKeyID`

- **Type:** `string`
- **Env:** `SECUREBOOT_AWSKMS_PCRKEYID`

PCRKeyID is the AWS KMS Key ID used for PCR measurement.

---

### `secureBoot.awsKMS.certPath`

- **Type:** `string`
- **Env:** `SECUREBOOT_AWSKMS_CERTPATH`

CertPath is the path to the certificate used with AWS KMS.

---

### `secureBoot.awsKMS.certARN`

- **Type:** `string`
- **Env:** `SECUREBOOT_AWSKMS_CERTARN`

CertARN is the ARN of the ACM certificate used with AWS KMS.

---

### `secureBoot.awsKMS.region`

- **Type:** `string`
- **Env:** `SECUREBOOT_AWSKMS_REGION`

Region is the AWS region containing the KMS keys.

---

### `secureBoot.enabled`

- **Type:** `bool`
- **Env:** `SECUREBOOT_ENABLED`

Enabled enables SecureBoot asset generation.

---

### `artifacts`

Artifacts defines names and references for various images used by the factory.

---

### `artifacts.core`

Core contains configuration for core images used by the image factory.

---

### `artifacts.core.registry`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_REGISTRY`

Registry specifies the OCI registry host for base images, extensions, and related artifacts.
E.g., "ghcr.io".

---

### `artifacts.core.namespace`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_NAMESPACE`

Namespace is an optional repository path prefix prepended to every image pulled from
the registry: both the component images below and the extension/overlay images
discovered via the manifests.

Useful for pull-through caches which prefix the upstream repository path, e.g. a Harbor
proxy-cache project: with registry "harbor.example.com" and namespace "ghcrio",
"siderolabs/imager" is pulled from "harbor.example.com/ghcrio/siderolabs/imager".

---

### `artifacts.core.components`

Components defines the names of images used by the image factory.
This typically maps to repositories and tags for core components.

---

### `artifacts.core.components.installerBase`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_INSTALLERBASE`

InstallerBase is the base image for creating installer images.

---

### `artifacts.core.components.installer`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_INSTALLER`

Installer is the main installer image.

---

### `artifacts.core.components.imager`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_IMAGER`

Imager is the image builder used by the factory.

---

### `artifacts.core.components.extensionManifest`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_EXTENSIONMANIFEST`

ExtensionManifest is the image manifest for extensions.

---

### `artifacts.core.components.overlayManifest`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_OVERLAYMANIFEST`

OverlayManifest is the image manifest for overlays.

---

### `artifacts.core.components.talosctl`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_TALOSCTL`

Talosctl is the image containing the Talos CLI tool (talosctl-all).

---

### `artifacts.core.components.imageFactory`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_COMPONENTS_IMAGEFACTORY`

ImageFactory is the image containing the Image Factory itself.

---

### `artifacts.core.insecure`

- **Type:** `bool`
- **Env:** `ARTIFACTS_CORE_INSECURE`

Insecure allows connections to the registry over HTTP or with invalid TLS certificates.
Use with caution, as this may expose security risks.

---

### `artifacts.schematic`

Schematic is the OCI repository used to store schematic blobs required by the image factory for building images.

---

### `artifacts.schematic.registry`

- **Type:** `string`
- **Env:** `ARTIFACTS_SCHEMATIC_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `artifacts.schematic.namespace`

- **Type:** `string`
- **Env:** `ARTIFACTS_SCHEMATIC_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `artifacts.schematic.repository`

- **Type:** `string`
- **Env:** `ARTIFACTS_SCHEMATIC_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `artifacts.schematic.insecure`

- **Type:** `bool`
- **Env:** `ARTIFACTS_SCHEMATIC_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `artifacts.installer`

Installer contains configuration for storing and accessing installer images.

---

### `artifacts.installer.internal`

Internal is the internal OCI registry used by the image factory to push installer images.

---

### `artifacts.installer.internal.registry`

- **Type:** `string`
- **Env:** `ARTIFACTS_INSTALLER_INTERNAL_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `artifacts.installer.internal.namespace`

- **Type:** `string`
- **Env:** `ARTIFACTS_INSTALLER_INTERNAL_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `artifacts.installer.internal.repository`

- **Type:** `string`
- **Env:** `ARTIFACTS_INSTALLER_INTERNAL_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `artifacts.installer.internal.insecure`

- **Type:** `bool`
- **Env:** `ARTIFACTS_INSTALLER_INTERNAL_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `artifacts.installer.external`

External is the public OCI registry used for redirects to installer images.

If this field is not set, Image Factory will proxy requests to the internal registry
through itself instead of issuing HTTP redirects to the external registry endpoint.

---

### `artifacts.installer.external.registry`

- **Type:** `string`
- **Env:** `ARTIFACTS_INSTALLER_EXTERNAL_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `artifacts.installer.external.namespace`

- **Type:** `string`
- **Env:** `ARTIFACTS_INSTALLER_EXTERNAL_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `artifacts.installer.external.repository`

- **Type:** `string`
- **Env:** `ARTIFACTS_INSTALLER_EXTERNAL_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `artifacts.installer.external.insecure`

- **Type:** `bool`
- **Env:** `ARTIFACTS_INSTALLER_EXTERNAL_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `artifacts.talosVersionRecheckInterval`

- **Type:** `time.Duration`
- **Env:** `ARTIFACTS_TALOSVERSIONRECHECKINTERVAL`

TalosVersionRecheckInterval sets the interval at which the image factory rechecks available Talos versions.

---

### `artifacts.refreshInterval`

- **Type:** `time.Duration`
- **Env:** `ARTIFACTS_REFRESHINTERVAL`

RefreshInterval specifies how often the image factory should refresh its connection to registries.

---

### `authentication`

Authentication settings.

Note: only available in the Enterprise edition.

---

### `authentication.enabled`

- **Type:** `bool`
- **Env:** `AUTHENTICATION_ENABLED`

Enabled enables authentication.

---

### `authentication.provider`

- **Type:** `string`
- **Env:** `AUTHENTICATION_PROVIDER`

Provider selects the authentication backend.
Valid values are "htpasswd" (default) and "auth0".

---

### `authentication.htpasswdPath`

- **Type:** `string`
- **Env:** `AUTHENTICATION_HTPASSWDPATH`

HTPasswdPath is the path to the htpasswd file containing user credentials.

The file follows the standard htpasswd format (username:bcrypt_hash, one per line).
Multiple entries with the same username are supported, allowing multiple API keys per user.
Only bcrypt hashes ($2y$/$2a$/$2b$) are accepted.

It is required when provider is "htpasswd" (the default).

---

### `authentication.auth0`

Auth0 holds configuration for the Auth0 JWT authentication provider.

Tokens must carry an `org_id` claim, which becomes the caller identity in the same way a username does for htpasswd.
In Auth0 this means issuing tokens to organization-scoped clients; tokens without the claim are rejected.

It is required when provider is "auth0", and ignored otherwise.

Domain and audience alone validate bearer tokens.
The browser-login fields are optional, and add the sign-in routes on top when set.

---

### `authentication.auth0.domain`

- **Type:** `string`
- **Env:** `AUTHENTICATION_AUTH0_DOMAIN`

Domain is the Auth0 tenant domain, e.g. `mycompany.auth0.com`.

Required.

---

### `authentication.auth0.audience`

- **Type:** `string`
- **Env:** `AUTHENTICATION_AUTH0_AUDIENCE`

Audience is the Auth0 API identifier (audience claim), e.g. `https://image-factory.example.com`.

Required.

---

### `authentication.auth0.machineScope`

- **Type:** `string`
- **Env:** `AUTHENTICATION_AUTH0_MACHINESCOPE`

MachineScope names a scope that marks a token as a machine credential, e.g. `factory:machine`.

Tokens carrying it receive exactly `image:read`: generated downloads, PXE assets, and generated installer OCI pulls.
Everything else is rejected with 403, including schematic definitions and proxied source images.
Intended for the long-lived tokens provisioned onto Talos nodes, which need to pull installers but should not be able to inspect or create schematics.

Optional; when empty every valid token has full access.

---

### `authentication.auth0.clientID`

- **Type:** `string`
- **Env:** `AUTHENTICATION_AUTH0_CLIENTID`

ClientID is the Auth0 application Client ID used for the browser login flow.

Optional; part of the browser-login group.

---

### `authentication.auth0.clientSecret`

- **Type:** `string`
- **Env:** `AUTHENTICATION_AUTH0_CLIENTSECRET`

ClientSecret is the Auth0 application Client Secret.
Inject via IF_AUTHENTICATION_AUTH0_CLIENTSECRET environment variable.

Optional; part of the browser-login group.

---

### `authentication.auth0.sessionKey`

- **Type:** `string`
- **Env:** `AUTHENTICATION_AUTH0_SESSIONKEY`

SessionKey is the base64-encoded 32-byte AES-256 key used to encrypt session cookies.
Inject via IF_AUTHENTICATION_AUTH0_SESSIONKEY environment variable.
Generate one with `openssl rand -base64 32`.
Surrounding whitespace is trimmed, so a file or mounted secret with a trailing newline works.
All replicas must share the same key, since a session or in-progress login started
on one replica has to be decrypted by whichever replica handles the next request.

Optional; part of the browser-login group.

---

### `authentication.tokens`

Tokens holds configuration for self-issued API token management.

---

### `authentication.tokens.keyPaths`

- **Type:** `[]string`
- **Env:** `AUTHENTICATION_TOKENS_KEYPATHS`

KeyPaths is an ordered list of PEM-encoded ECDSA P-256 keys or certificates.
The first entry must be a private key and is the only key used to mint tokens.
Later entries are verification-only and may contain private keys, public keys, or X.509 certificates.
If empty, a fresh key is generated at startup, which only works for single-replica deployments.

---

### `authentication.tokens.storage`

Storage is the OCI repository used to persist the per-org token index (the list of active stored tokens; presence in the index is what makes such a token valid).
A token minted with "stored": false is not recorded, so it cannot be listed or revoked and does not count against MaxPerOrg.

---

### `authentication.tokens.storage.registry`

- **Type:** `string`
- **Env:** `AUTHENTICATION_TOKENS_STORAGE_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `authentication.tokens.storage.namespace`

- **Type:** `string`
- **Env:** `AUTHENTICATION_TOKENS_STORAGE_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `authentication.tokens.storage.repository`

- **Type:** `string`
- **Env:** `AUTHENTICATION_TOKENS_STORAGE_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `authentication.tokens.storage.insecure`

- **Type:** `bool`
- **Env:** `AUTHENTICATION_TOKENS_STORAGE_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `authentication.tokens.ttl`

TTL bounds token lifetimes by whether they are stored, plus the CLI-only bootstrap policy.

---

### `authentication.tokens.ttl.stored`

Stored bounds revocable tokens persisted in the configured OCI repository.

---

### `authentication.tokens.ttl.stored.max`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_STORED_MAX`

Max is the longest validity duration a caller may request.

---

### `authentication.tokens.ttl.stored.min`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_STORED_MIN`

Min is the shortest validity duration a caller may request.

---

### `authentication.tokens.ttl.stored.default`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_STORED_DEFAULT`

Default is the validity duration granted when the caller requests no explicit TTL.

---

### `authentication.tokens.ttl.ephemeral`

Ephemeral bounds tokens whose expiry is the only way to take them out of circulation.

---

### `authentication.tokens.ttl.ephemeral.max`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_EPHEMERAL_MAX`

Max is the longest validity duration a caller may request.

---

### `authentication.tokens.ttl.ephemeral.min`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_EPHEMERAL_MIN`

Min is the shortest validity duration a caller may request.

---

### `authentication.tokens.ttl.ephemeral.default`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_EPHEMERAL_DEFAULT`

Default is the validity duration granted when the caller requests no explicit TTL.

---

### `authentication.tokens.ttl.bootstrap`

Bootstrap bounds the CLI-only cross-subject credential.
It is never stored and may live longer than ordinary ephemeral tokens because it is kept
offline and retired by removing its signing key from KeyPaths.

---

### `authentication.tokens.ttl.bootstrap.max`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_BOOTSTRAP_MAX`

Max is the longest validity duration a caller may request.

---

### `authentication.tokens.ttl.bootstrap.min`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_BOOTSTRAP_MIN`

Min is the shortest validity duration a caller may request.

---

### `authentication.tokens.ttl.bootstrap.default`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_TTL_BOOTSTRAP_DEFAULT`

Default is the validity duration granted when the caller requests no explicit TTL.

---

### `authentication.tokens.verificationCacheRefreshInterval`

- **Type:** `time.Duration`
- **Env:** `AUTHENTICATION_TOKENS_VERIFICATIONCACHEREFRESHINTERVAL`

VerificationCacheRefreshInterval controls how often the backing registry clients are rebuilt
so refreshed credentials are picked up.

The legacy name is retained for configuration compatibility.

---

### `authentication.tokens.maxPerOrg`

- **Type:** `int`
- **Env:** `AUTHENTICATION_TOKENS_MAXPERORG`

MaxPerOrg caps how many stored tokens an org may have active at once.

---

### `enterprise`

Enterprise contains configuration for enterprise-specific features.

---

### `enterprise.extraExtensions`

ExtraExtensions contains configuration for extra (custom) extensions.

---

### `enterprise.extraExtensions.manifest`

Manifest specifies the OCI repository holding the extra extensions manifest image.

It may live in a different registry than the official images.

---

### `enterprise.extraExtensions.manifest.registry`

- **Type:** `string`
- **Env:** `ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `enterprise.extraExtensions.manifest.namespace`

- **Type:** `string`
- **Env:** `ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `enterprise.extraExtensions.manifest.repository`

- **Type:** `string`
- **Env:** `ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `enterprise.extraExtensions.manifest.insecure`

- **Type:** `bool`
- **Env:** `ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `enterprise.scanner`

Scanner contains configuration for the vulnerability scanner.

---

### `enterprise.scanner.databaseURL`

- **Type:** `string`
- **Env:** `ENTERPRISE_SCANNER_DATABASEURL`

DatabaseURL overrides the Grype vulnerability database listing URL.
Set this to point at a mirror or air-gapped database service.

---

### `enterprise.scanner.databaseUpdateAt`

- **Type:** `string`
- **Env:** `ENTERPRISE_SCANNER_DATABASEUPDATEAT`

DatabaseUpdateAt is the local time of day ("HH:MM") at which the Grype vulnerability database is updated.
Empty disables the schedule, leaving the database at the build downloaded when the process started.

Scan results depend on the database build, so replicas that started at different times otherwise disagree about the same report.
Pick a time after the upstream database is published (Anchore publishes daily around 06:40 UTC) so every replica converges on the same build for the rest of the day.
Times are interpreted in the process timezone (TZ, UTC in the container).

---

### `enterprise.scanner.databaseRootDir`

- **Type:** `string`
- **Env:** `ENTERPRISE_SCANNER_DATABASEROOTDIR`

DatabaseRootDir is where the Grype vulnerability database is installed.
The Helm chart mounts a volume at the default path, so a persistent volume
keeps the database (and the scan results it produces) across restarts.

---

### `enterprise.scanner.cache`

Cache contains configuration for caching vulnerability scan results.

---

### `enterprise.scanner.cache.ttl`

- **Type:** `time.Duration`
- **Env:** `ENTERPRISE_SCANNER_CACHE_TTL`

TTL is the duration for caching objects.

---

### `enterprise.scanner.cache.capacity`

- **Type:** `uint64`
- **Env:** `ENTERPRISE_SCANNER_CACHE_CAPACITY`

Capacity caps the number of cached objects before LRU eviction.

---

### `enterprise.spdx`

SPDX contains configuration for SPDX document generation.

---

### `enterprise.spdx.cache`

---

### `enterprise.spdx.cache.registry`

- **Type:** `string`
- **Env:** `ENTERPRISE_SPDX_CACHE_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `enterprise.spdx.cache.namespace`

- **Type:** `string`
- **Env:** `ENTERPRISE_SPDX_CACHE_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `enterprise.spdx.cache.repository`

- **Type:** `string`
- **Env:** `ENTERPRISE_SPDX_CACHE_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `enterprise.spdx.cache.insecure`

- **Type:** `bool`
- **Env:** `ENTERPRISE_SPDX_CACHE_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `enterprise.vex`

VEX contains configuration for VEX data fetching.

---

### `enterprise.vex.data`

Data specifies the OCI repository where VEX documents are stored.

---

### `enterprise.vex.data.registry`

- **Type:** `string`
- **Env:** `ENTERPRISE_VEX_DATA_REGISTRY`

Registry is the hostname of the container registry, e.g., `ghcr.io`.
This is where images are stored.

---

### `enterprise.vex.data.namespace`

- **Type:** `string`
- **Env:** `ENTERPRISE_VEX_DATA_NAMESPACE`

Namespace is the repository namespace or organization within the registry, e.g., `sidero-labs`.
Some registries allow repositories without a namespace.

---

### `enterprise.vex.data.repository`

- **Type:** `string`
- **Env:** `ENTERPRISE_VEX_DATA_REPOSITORY`

Repository is the name of the repository inside the namespace, e.g., `talos`.
Combined with Registry and Namespace, it forms the fully qualified repository path.

---

### `enterprise.vex.data.insecure`

- **Type:** `bool`
- **Env:** `ENTERPRISE_VEX_DATA_INSECURE`

Insecure allows connections to registries over HTTP or with invalid TLS certificates.

---

### `enterprise.vex.cache`

Cache contains configuration for caching VEX documents.

---

### `enterprise.vex.cache.ttl`

- **Type:** `time.Duration`
- **Env:** `ENTERPRISE_VEX_CACHE_TTL`

TTL is the duration for caching objects.

---

### `enterprise.vex.cache.capacity`

- **Type:** `uint64`
- **Env:** `ENTERPRISE_VEX_CACHE_CAPACITY`

Capacity caps the number of cached objects before LRU eviction.

---

### `registry`

Registry contains low-level tuning for the registry client (pull/push concurrency, debugging).

---

### `registry.jobs`

- **Type:** `int`
- **Env:** `REGISTRY_JOBS`

Jobs is the maximum number of concurrent blob pull/push operations per registry client.

go-containerregistry gates concurrent blob fetches on this value; too low a value can
deadlock under Image Factory's concurrent, multiplexed fetch pattern.
Defaults to remotewrap.DefaultJobs.

---

### `registry.debug`

- **Type:** `bool`
- **Env:** `REGISTRY_DEBUG`

Debug tracks registry response bodies to help diagnose pull-limiter token leaks/stalls:
it periodically logs how many bodies are open and dumps any body that stays open too long
together with the stack that opened it.

Set via config or the IF_REGISTRY_DEBUG environment variable.

---

### `audit`

Audit configures the audit log of authenticated requests.

---

### `audit.mode`

- **Type:** `string`
- **Env:** `AUDIT_MODE`

Mode selects the audit sink: "" (none, default), "file", or "log".

---

### `audit.file`

File configures the "file" audit sink.

---

### `audit.file.path`

- **Type:** `string`
- **Env:** `AUDIT_FILE_PATH`

Path is the file to append audit records to, as newline-delimited JSON.
Leave empty to write records to /tmp/audit.log.

---

## Default Configuration

### YAML

```yaml
artifacts:
    core:
        components:
            extensionManifest: siderolabs/extensions
            imageFactory: siderolabs/image-factory
            imager: siderolabs/imager
            installer: siderolabs/installer
            installerBase: siderolabs/installer-base
            overlayManifest: siderolabs/overlays
            talosctl: siderolabs/talosctl-all
        insecure: false
        namespace: ""
        registry: ghcr.io
    installer:
        external:
            insecure: false
            namespace: ""
            registry: ""
            repository: ""
        internal:
            insecure: false
            namespace: siderolabs
            registry: ghcr.io
            repository: ""
    refreshInterval: 5m0s
    schematic:
        insecure: false
        namespace: siderolabs/image-factory
        registry: ghcr.io
        repository: schematics
    talosVersionRecheckInterval: 15m0s
audit:
    file:
        maxAge: 30
        maxBackups: 16
        maxSizeMB: 256
        path: ""
    mode: ""
authentication:
    auth0:
        audience: ""
        clientID: ""
        clientSecret: ""
        domain: ""
        machineScope: ""
        sessionKey: ""
    enabled: false
    htpasswdPath: ""
    provider: htpasswd
    tokens:
        keyPaths: []
        maxPerOrg: 10
        storage:
            insecure: false
            namespace: siderolabs/image-factory
            registry: ghcr.io
            repository: tokens
        ttl:
            bootstrap:
                default: 2160h0m0s
                max: 87600h0m0s
                min: 1h0m0s
            ephemeral:
                default: 5m0s
                max: 8h0m0s
                min: 30s
            stored:
                default: 8760h0m0s
                max: 8760h0m0s
                min: 1h0m0s
        verificationCacheRefreshInterval: 5m0s
build:
    brokenTalosVersions: []
    maxConcurrency: 6
    minTalosVersion: 1.2.0
cache:
    cdn:
        enabled: false
        host: ""
        trimPrefix: ""
    gsa:
        fulcioURL: ""
        keyFile: ""
        rekorURL: ""
        serviceAccountEmail: ""
        tsaURL: ""
    oci:
        insecure: false
        namespace: siderolabs/image-factory
        registry: ghcr.io
        repository: cache
    s3:
        bucket: image-factory
        enabled: false
        endpoint: ""
        insecure: false
        presignedURLTTL: 1h0m0s
        region: ""
    schematic:
        capacity: 100000
        negativeTTL: 30s
    signingKeyPath: ""
containerSignature:
    disabled: false
    issuer: https://accounts.google.com
    issuerRegExp: ""
    publicKeyFile: ""
    publicKeyHashAlgo: sha256
    subjectRegExp: (@siderolabs\.com$|^releasemgr-svc@talos-production\.iam\.gserviceaccount\.com$)
enterprise:
    extraExtensions:
        manifest:
            insecure: false
            namespace: ""
            registry: ""
            repository: ""
    scanner:
        cache:
            capacity: 4096
            ttl: 15m0s
        databaseRootDir: /var/lib/grype
        databaseURL: https://grype.anchore.io/databases
        databaseUpdateAt: "07:00"
    spdx:
        cache:
            insecure: false
            namespace: siderolabs/image-factory
            registry: ghcr.io
            repository: spdx-cache
    vex:
        cache:
            capacity: 65536
            ttl: 15m0s
        data:
            insecure: false
            namespace: siderolabs/talos-vex
            registry: ghcr.io
            repository: talos-vex-data
http:
    allowedOrigins:
        - '*'
    certFile: ""
    externalPXEURL: ""
    externalURL: https://localhost/
    httpListenAddr: :8080
    keyFile: ""
metrics:
    addr: :2122
registry:
    debug: false
    jobs: 64
secureBoot:
    awsKMS:
        certARN: ""
        certPath: ""
        keyID: ""
        pcrKeyID: ""
        region: ""
    azureKeyVault:
        certificateName: ""
        keyName: ""
        url: ""
    enabled: false
    file:
        pcrKeyPath: ""
        signingCertPath: ""
        signingKeyPath: ""
```

### Environment Variables

```env
IF_ARTIFACTS_CORE_COMPONENTS_EXTENSIONMANIFEST=siderolabs/extensions
IF_ARTIFACTS_CORE_COMPONENTS_IMAGEFACTORY=siderolabs/image-factory
IF_ARTIFACTS_CORE_COMPONENTS_IMAGER=siderolabs/imager
IF_ARTIFACTS_CORE_COMPONENTS_INSTALLER=siderolabs/installer
IF_ARTIFACTS_CORE_COMPONENTS_INSTALLERBASE=siderolabs/installer-base
IF_ARTIFACTS_CORE_COMPONENTS_OVERLAYMANIFEST=siderolabs/overlays
IF_ARTIFACTS_CORE_COMPONENTS_TALOSCTL=siderolabs/talosctl-all
IF_ARTIFACTS_CORE_INSECURE=false
IF_ARTIFACTS_CORE_NAMESPACE=
IF_ARTIFACTS_CORE_REGISTRY=ghcr.io
IF_ARTIFACTS_INSTALLER_EXTERNAL_INSECURE=false
IF_ARTIFACTS_INSTALLER_EXTERNAL_NAMESPACE=
IF_ARTIFACTS_INSTALLER_EXTERNAL_REGISTRY=
IF_ARTIFACTS_INSTALLER_EXTERNAL_REPOSITORY=
IF_ARTIFACTS_INSTALLER_INTERNAL_INSECURE=false
IF_ARTIFACTS_INSTALLER_INTERNAL_NAMESPACE=siderolabs
IF_ARTIFACTS_INSTALLER_INTERNAL_REGISTRY=ghcr.io
IF_ARTIFACTS_INSTALLER_INTERNAL_REPOSITORY=
IF_ARTIFACTS_REFRESHINTERVAL=5m0s
IF_ARTIFACTS_SCHEMATIC_INSECURE=false
IF_ARTIFACTS_SCHEMATIC_NAMESPACE=siderolabs/image-factory
IF_ARTIFACTS_SCHEMATIC_REGISTRY=ghcr.io
IF_ARTIFACTS_SCHEMATIC_REPOSITORY=schematics
IF_ARTIFACTS_TALOSVERSIONRECHECKINTERVAL=15m0s
IF_AUDIT_FILE_MAXAGE=30
IF_AUDIT_FILE_MAXBACKUPS=16
IF_AUDIT_FILE_MAXSIZEMB=256
IF_AUDIT_FILE_PATH=
IF_AUDIT_MODE=
IF_AUTHENTICATION_AUTH0_AUDIENCE=
IF_AUTHENTICATION_AUTH0_CLIENTID=
IF_AUTHENTICATION_AUTH0_CLIENTSECRET=
IF_AUTHENTICATION_AUTH0_DOMAIN=
IF_AUTHENTICATION_AUTH0_MACHINESCOPE=
IF_AUTHENTICATION_AUTH0_SESSIONKEY=
IF_AUTHENTICATION_ENABLED=false
IF_AUTHENTICATION_HTPASSWDPATH=
IF_AUTHENTICATION_PROVIDER=htpasswd
IF_AUTHENTICATION_TOKENS_KEYPATHS=[]
IF_AUTHENTICATION_TOKENS_MAXPERORG=10
IF_AUTHENTICATION_TOKENS_STORAGE_INSECURE=false
IF_AUTHENTICATION_TOKENS_STORAGE_NAMESPACE=siderolabs/image-factory
IF_AUTHENTICATION_TOKENS_STORAGE_REGISTRY=ghcr.io
IF_AUTHENTICATION_TOKENS_STORAGE_REPOSITORY=tokens
IF_AUTHENTICATION_TOKENS_TTL_BOOTSTRAP_DEFAULT=2160h0m0s
IF_AUTHENTICATION_TOKENS_TTL_BOOTSTRAP_MAX=87600h0m0s
IF_AUTHENTICATION_TOKENS_TTL_BOOTSTRAP_MIN=1h0m0s
IF_AUTHENTICATION_TOKENS_TTL_EPHEMERAL_DEFAULT=5m0s
IF_AUTHENTICATION_TOKENS_TTL_EPHEMERAL_MAX=8h0m0s
IF_AUTHENTICATION_TOKENS_TTL_EPHEMERAL_MIN=30s
IF_AUTHENTICATION_TOKENS_TTL_STORED_DEFAULT=8760h0m0s
IF_AUTHENTICATION_TOKENS_TTL_STORED_MAX=8760h0m0s
IF_AUTHENTICATION_TOKENS_TTL_STORED_MIN=1h0m0s
IF_AUTHENTICATION_TOKENS_VERIFICATIONCACHEREFRESHINTERVAL=5m0s
IF_BUILD_BROKENTALOSVERSIONS=[]
IF_BUILD_MAXCONCURRENCY=6
IF_BUILD_MINTALOSVERSION=1.2.0
IF_CACHE_CDN_ENABLED=false
IF_CACHE_CDN_HOST=
IF_CACHE_CDN_TRIMPREFIX=
IF_CACHE_GSA_FULCIOURL=
IF_CACHE_GSA_KEYFILE=
IF_CACHE_GSA_REKORURL=
IF_CACHE_GSA_SERVICEACCOUNTEMAIL=
IF_CACHE_GSA_TSAURL=
IF_CACHE_OCI_INSECURE=false
IF_CACHE_OCI_NAMESPACE=siderolabs/image-factory
IF_CACHE_OCI_REGISTRY=ghcr.io
IF_CACHE_OCI_REPOSITORY=cache
IF_CACHE_S3_BUCKET=image-factory
IF_CACHE_S3_ENABLED=false
IF_CACHE_S3_ENDPOINT=
IF_CACHE_S3_INSECURE=false
IF_CACHE_S3_PRESIGNEDURLTTL=1h0m0s
IF_CACHE_S3_REGION=
IF_CACHE_SCHEMATIC_CAPACITY=100000
IF_CACHE_SCHEMATIC_NEGATIVETTL=30s
IF_CACHE_SIGNINGKEYPATH=
IF_CONTAINERSIGNATURE_DISABLED=false
IF_CONTAINERSIGNATURE_ISSUER=https://accounts.google.com
IF_CONTAINERSIGNATURE_ISSUERREGEXP=
IF_CONTAINERSIGNATURE_PUBLICKEYFILE=
IF_CONTAINERSIGNATURE_PUBLICKEYHASHALGO=sha256
IF_CONTAINERSIGNATURE_SUBJECTREGEXP=(@siderolabs\.com$|^releasemgr-svc@talos-production\.iam\.gserviceaccount\.com$)
IF_ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_INSECURE=false
IF_ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_NAMESPACE=
IF_ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_REGISTRY=
IF_ENTERPRISE_EXTRAEXTENSIONS_MANIFEST_REPOSITORY=
IF_ENTERPRISE_SCANNER_CACHE_CAPACITY=4096
IF_ENTERPRISE_SCANNER_CACHE_TTL=15m0s
IF_ENTERPRISE_SCANNER_DATABASEROOTDIR=/var/lib/grype
IF_ENTERPRISE_SCANNER_DATABASEUPDATEAT=07:00
IF_ENTERPRISE_SCANNER_DATABASEURL=https://grype.anchore.io/databases
IF_ENTERPRISE_SPDX_CACHE_INSECURE=false
IF_ENTERPRISE_SPDX_CACHE_NAMESPACE=siderolabs/image-factory
IF_ENTERPRISE_SPDX_CACHE_REGISTRY=ghcr.io
IF_ENTERPRISE_SPDX_CACHE_REPOSITORY=spdx-cache
IF_ENTERPRISE_VEX_CACHE_CAPACITY=65536
IF_ENTERPRISE_VEX_CACHE_TTL=15m0s
IF_ENTERPRISE_VEX_DATA_INSECURE=false
IF_ENTERPRISE_VEX_DATA_NAMESPACE=siderolabs/talos-vex
IF_ENTERPRISE_VEX_DATA_REGISTRY=ghcr.io
IF_ENTERPRISE_VEX_DATA_REPOSITORY=talos-vex-data
IF_HTTP_ALLOWEDORIGINS=["*"]
IF_HTTP_CERTFILE=
IF_HTTP_EXTERNALPXEURL=
IF_HTTP_EXTERNALURL=https://localhost/
IF_HTTP_HTTPLISTENADDR=:8080
IF_HTTP_KEYFILE=
IF_METRICS_ADDR=:2122
IF_REGISTRY_DEBUG=false
IF_REGISTRY_JOBS=64
IF_SECUREBOOT_AWSKMS_CERTARN=
IF_SECUREBOOT_AWSKMS_CERTPATH=
IF_SECUREBOOT_AWSKMS_KEYID=
IF_SECUREBOOT_AWSKMS_PCRKEYID=
IF_SECUREBOOT_AWSKMS_REGION=
IF_SECUREBOOT_AZUREKEYVAULT_CERTIFICATENAME=
IF_SECUREBOOT_AZUREKEYVAULT_KEYNAME=
IF_SECUREBOOT_AZUREKEYVAULT_URL=
IF_SECUREBOOT_ENABLED=false
IF_SECUREBOOT_FILE_PCRKEYPATH=
IF_SECUREBOOT_FILE_SIGNINGCERTPATH=
IF_SECUREBOOT_FILE_SIGNINGKEYPATH=
```
