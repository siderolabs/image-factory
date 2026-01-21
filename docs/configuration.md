# Configuration

## CLI Usage

```console
Usage of image-factory:
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

### `build.minTalosVersion`

- **Type:** `string`
- **Env:** `BUILD_MINTALOSVERSION`

MinTalosVersion specifies the minimum supported Talos version for assets.

---

### `build.maxConcurrency`

- **Type:** `int`
- **Env:** `BUILD_MAXCONCURRENCY`

MaxConcurrency sets the maximum number of simultaneous asset build operations.

---

### `containerSignature.subjectRegExp`

- **Type:** `string`
- **Env:** `CONTAINERSIGNATURE_SUBJECTREGEXP`

SubjectRegExp is a regular expression used to validate the subject in container signatures.

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

### `metrics.addr`

- **Type:** `string`
- **Env:** `METRICS_ADDR`

Addr is the bind address for the metrics HTTP server.
Leave empty to disable metrics.

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

### `artifacts.core.registry`

- **Type:** `string`
- **Env:** `ARTIFACTS_CORE_REGISTRY`

Registry specifies the OCI registry host for base images, extensions, and related artifacts.
E.g., "ghcr.io".

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

Talosctl is the image containing the Talos CLI tool.

---

### `artifacts.core.insecure`

- **Type:** `bool`
- **Env:** `ARTIFACTS_CORE_INSECURE`

Insecure allows connections to the registry over HTTP or with invalid TLS certificates.
Use with caution, as this may expose security risks.

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

## Default Configuration

### YAML

```yaml
artifacts:
    core:
        components:
            extensionManifest: siderolabs/extensions
            imager: siderolabs/imager
            installer: siderolabs/installer
            installerBase: siderolabs/installer-base
            overlayManifest: siderolabs/overlays
            talosctl: siderolabs/talosctl-all
        insecure: false
        registry: ghcr.io
    installer:
        external:
            insecure: false
            namespace: ""
            registry: ""
            repository: ""
        internal:
            insecure: false
            namespace: ""
            registry: ghcr.io
            repository: siderolabs
    refreshInterval: 5m0s
    schematic:
        insecure: false
        namespace: siderolabs/image-factory
        registry: ghcr.io
        repository: schematics
    talosVersionRecheckInterval: 15m0s
build:
    maxConcurrency: 6
    minTalosVersion: 1.2.0
cache:
    cdn:
        enabled: false
        host: ""
        trimPrefix: ""
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
        region: ""
    signingKeyPath: ""
containerSignature:
    disabled: false
    issuer: https://accounts.google.com
    issuerRegExp: ""
    publicKeyFile: ""
    publicKeyHashAlgo: sha256
    subjectRegExp: '@siderolabs\.com$'
http:
    certFile: ""
    externalPXEURL: ""
    externalURL: https://localhost/
    httpListenAddr: :8080
    keyFile: ""
metrics:
    addr: :2122
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
IF_ARTIFACTS_CORE_COMPONENTS_IMAGER=siderolabs/imager
IF_ARTIFACTS_CORE_COMPONENTS_INSTALLER=siderolabs/installer
IF_ARTIFACTS_CORE_COMPONENTS_INSTALLERBASE=siderolabs/installer-base
IF_ARTIFACTS_CORE_COMPONENTS_OVERLAYMANIFEST=siderolabs/overlays
IF_ARTIFACTS_CORE_COMPONENTS_TALOSCTL=siderolabs/talosctl-all
IF_ARTIFACTS_CORE_INSECURE=false
IF_ARTIFACTS_CORE_REGISTRY=ghcr.io
IF_ARTIFACTS_INSTALLER_EXTERNAL_INSECURE=false
IF_ARTIFACTS_INSTALLER_EXTERNAL_NAMESPACE=
IF_ARTIFACTS_INSTALLER_EXTERNAL_REGISTRY=
IF_ARTIFACTS_INSTALLER_EXTERNAL_REPOSITORY=
IF_ARTIFACTS_INSTALLER_INTERNAL_INSECURE=false
IF_ARTIFACTS_INSTALLER_INTERNAL_NAMESPACE=
IF_ARTIFACTS_INSTALLER_INTERNAL_REGISTRY=ghcr.io
IF_ARTIFACTS_INSTALLER_INTERNAL_REPOSITORY=siderolabs
IF_ARTIFACTS_REFRESHINTERVAL=5m0s
IF_ARTIFACTS_SCHEMATIC_INSECURE=false
IF_ARTIFACTS_SCHEMATIC_NAMESPACE=siderolabs/image-factory
IF_ARTIFACTS_SCHEMATIC_REGISTRY=ghcr.io
IF_ARTIFACTS_SCHEMATIC_REPOSITORY=schematics
IF_ARTIFACTS_TALOSVERSIONRECHECKINTERVAL=15m0s
IF_BUILD_MAXCONCURRENCY=6
IF_BUILD_MINTALOSVERSION=1.2.0
IF_CACHE_CDN_ENABLED=false
IF_CACHE_CDN_HOST=
IF_CACHE_CDN_TRIMPREFIX=
IF_CACHE_OCI_INSECURE=false
IF_CACHE_OCI_NAMESPACE=siderolabs/image-factory
IF_CACHE_OCI_REGISTRY=ghcr.io
IF_CACHE_OCI_REPOSITORY=cache
IF_CACHE_S3_BUCKET=image-factory
IF_CACHE_S3_ENABLED=false
IF_CACHE_S3_ENDPOINT=
IF_CACHE_S3_INSECURE=false
IF_CACHE_S3_REGION=
IF_CACHE_SIGNINGKEYPATH=
IF_CONTAINERSIGNATURE_DISABLED=false
IF_CONTAINERSIGNATURE_ISSUER=https://accounts.google.com
IF_CONTAINERSIGNATURE_ISSUERREGEXP=
IF_CONTAINERSIGNATURE_PUBLICKEYFILE=
IF_CONTAINERSIGNATURE_PUBLICKEYHASHALGO=sha256
IF_CONTAINERSIGNATURE_SUBJECTREGEXP=@siderolabs\.com$
IF_HTTP_CERTFILE=
IF_HTTP_EXTERNALPXEURL=
IF_HTTP_EXTERNALURL=https://localhost/
IF_HTTP_HTTPLISTENADDR=:8080
IF_HTTP_KEYFILE=
IF_METRICS_ADDR=:2122
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
