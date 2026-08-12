# API

## Authentication

Authentication is an Enterprise feature, enabled with `authentication.enabled`; when it is off, every endpoint is public.
When it is on, each endpoint below carries an **Access** table with these four fields; endpoints that never take a credential are marked `Access: public` instead.
See [Authentication](authentication.md) for the providers, the token formats and the scopes.

| Field             | Values                                                                                                                                                                                                                      |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Auth**          | `public` takes no credential, `required` takes one for the configured provider (Basic for htpasswd, `Authorization: Bearer` or the same JWT in the Basic password for auth0), and a missing or invalid credential is `401`. |
| **`?token=`**     | Whether a [download token](authentication.md#download-tokens) in the query string is accepted in place of a credential.                                                                                                     |
| **Machine scope** | Whether a token carrying `authentication.auth0.machineScope` may call the endpoint. `denied` is `403`, even though the token itself is valid.                                                                               |
| **Ownership**     | Whether the schematic's `owner` must match the caller identity, where a mismatch is `403`; an unauthenticated caller gets `401` whether or not the schematic exists, so existence is not leaked.                            |

`machineScope` is the only scope the factory interprets.
There is no per-endpoint scope model: a token either carries the machine scope, and is then limited to the endpoints that allow it, or it does not and has full access.
The htpasswd provider has no scopes at all.

## Enterprise Frontend API

### `GET /spdx/:schematic/:version/:arch`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth     | `?token=`    | Machine scope | Ownership |
| -------- | ------------ | ------------- | --------- |
| required | not accepted | denied        | enforced  |

Returns an SPDX 2.3 JSON document containing all packages from the Talos and extensions for the given schematic and version.
The response is a JSON-encoded SPDX document which can be consumed directly by vulnerability scanners such as grype:

```shell
grype sbom:response.spdx.json
```

SPDX bundles are available for Talos versions **v1.11.0** and later.

### `GET /vex/:version/vex.json`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth     | `?token=`    | Machine scope | Ownership      |
| -------- | ------------ | ------------- | -------------- |
| required | not accepted | denied        | not applicable |

The document describes a Talos Linux release, not a schematic: it is identical for every caller and contains nothing derived from a schematic, so there is no `owner` to match.
Any authenticated caller can therefore read it for any version.

Returns a VEX JSON document containing vulnerability information for all packages in the Talos Linux release.
The response is a JSON-encoded VEX document which can be consumed directly by vulnerability scanners such as grype:

```shell
grype sbom:talos.spdx.json --vex response.vex.json
```

### `GET /scans/:schematic/:version/:arch/:report`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth     | `?token=`    | Machine scope | Ownership |
| -------- | ------------ | ------------- | --------- |
| required | not accepted | denied        | enforced  |

Returns a vulnerability scan report for the specified schematic, Talos Linux version and architecture.

Supported report formats:

* `.json` - JSON-encoded report in the format provided by the underlying vulnerability scanner
* `.table` - human-readable table format
* `.sarif` - SARIF format
* `.cdx` - CycloneDX format

### `POST /download-token`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth     | `?token=`    | Machine scope | Ownership      |
| -------- | ------------ | ------------- | -------------- |
| required | not accepted | denied        | not applicable |

Issues a short-lived JWT that authenticates image downloads through the URL alone.
The token is scoped to the calling identity.

```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 300
}
```

`expires_in` is `authentication.downloadTokenTTL` in seconds, `5m` by default.

The token is appended to an image URL as `?token=<access_token>` and is accepted only on `GET` and `HEAD` under `/image/`.
One token covers every schematic owned by the caller, not just the URL it is used with; see [Authentication](authentication.md#download-tokens).

### `GET /.well-known/jwks.json`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access: `public`.

Returns the JSON Web Key Set containing the ECDSA P-256 public key that download tokens are signed with, so that a proxy can verify a token without holding the private key.

## HTTP Frontend API

### `POST /schematics`

Create a new image schematic.

Access:

| Auth     | `?token=`    | Machine scope | Ownership    |
| -------- | ------------ | ------------- | ------------ |
| required | not accepted | denied        | sets `owner` |

The request body is a YAML (JSON) encoded schematic description:

```yaml
customization:
    extraKernelArgs: # optional
        - vga=791
    meta: # optional, allows to set initial Talos META
      - key: 0xa
        value: "{}"
    systemExtensions: # optional
      officialExtensions: # optional
        - siderolabs/gvisor
        - siderolabs/amd-ucode
    secureboot: # optional, only applies to SecureBoot images
       # optional, include well-known UEFI certificates into auto-enrollment database (SecureBoot ISO only)
      includeWellKnownCertificates: true
       # optional, how systemd-boot enrolls SecureBoot keys on first boot: off, manual, if-safe, force
       # defaults to if-safe (auto-enrolls only in a VM); use force for unattended bare-metal enrollment in setup mode
      enrollKeys: force
    bootloader: sd-boot # optional, defaults to auto (bootloader chosen by imager), other options: dual-boot, grub
    embeddedMachineConfiguration: | # optional, embedded machine configuration (YAML-encoded)
      apiVersion: v1alpha1
      kind: HostnameConfig
      hostname: my-custom-hostname
      auto: off
      ---
      apiVersion: v1alpha1
      kind: KmsgLogConfig
      name: remote-log
      url: tcp://10.0.0.50:5044/
    diskImage: # optional, only applies to disk images
      sectorSize: 4096 # optional, disk image sector size in bytes, defaults to 512 if not set
overlay: # optional
  image: ghcr.io/siderolabs/sbc-raspberry-pi # overlay image
  name: rpi_generic # overlay name
  options: # optional, any valid yaml, depends on the overlay implementation
    data: "mydata"
```

Output is a JSON object containing the schematic ID and the canonical schematic body as YAML:

```json
{
  "id": "2a63b6e7dab90ec9d44f213339b9545bd39c6499b22a14cf575c1ca4b6e39ff8",
  "schematic": "customization:\n    extraKernelArgs:\n        - vga=791\n"
}
```

The `schematic` field is the canonical representation used to compute the ID.
Callers should treat it as authoritative, since the factory may modify or add fields to the submitted schematic, for example setting `owner` for authenticated requests.

This ID can be used to download images with this schematic.

Well-known schematic IDs:

* `376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba` - default schematic (without any customizations)

The `owner` is part of the canonical body, so it changes the ID.
A deployment with authentication enabled therefore has its own ID per identity for an otherwise identical schematic, and the well-known IDs above, which carry no owner, are not reachable there; see [Authentication](authentication.md#ownership).

The schematic in Enterprise edition may contain an `owner` field, which restricts access to the schematic to the specified owner only.
This requires authentication to be enabled.
When authentication is enabled, the factory sets `owner` to the authenticated user.
If the request body specifies an `owner` that does not match the authenticated user, the request is rejected.

### `GET /schematics/:schematic`

Retrieve a specific schematic by its ID.

Access:

| Auth     | `?token=`    | Machine scope | Ownership |
| -------- | ------------ | ------------- | --------- |
| required | not accepted | denied        | enforced  |

If the schematic is found, the response body contains the YAML-encoded schematic representation.
Otherwise a `404 Not Found` status code is returned.

### `GET /image/:schematic/:version/:path`

Download a Talos Linux boot image with the specified schematic and Talos Linux version.

Access:

| Auth     | `?token=` | Machine scope | Ownership |
| -------- | --------- | ------------- | --------- |
| required | accepted  | allowed       | enforced  |

* `:schematic` is a schematic ID returned by `POST /schematic`
* `:version` is a Talos Linux version, e.g. `v1.5.0`
* `:path` is a specific image path (details below)

In Enterprise edition this route also accepts a [download token](authentication.md#download-tokens) as `?token=<jwt>` in place of an `Authorization` header.

Common used parameters:

* `<arch>` image architecture: `amd64` or `arm64`
* `<platform>` Talos Linux platform, e.g. `metal`, `aws`, `gcp`, etc.
* `-secureboot` identifies a Secure Boot asset

Supported image paths:

* `kernel-<arch>` (e.g. `kernel-amd64`) - raw kernel image
* `cmdline-<platform>-<arch>[-secureboot]` (e.g. `cmdline-metal-amd64`) - kernel command line
* `initramfs-<arch>.xz` (e.g. `initramfs-amd64.xz`) - initramfs image (including system extensions if configured)
* `<platform>-<arch>[-secureboot].iso` (e.g. `metal-amd64.iso`) - ISO image
* `<platform>-<arch>[-secureboot]-uki.efi` (e.g. `metal-amd64-secureboot-uki.efi`) UEFI UKI image (Secure Boot compatible)
* `installer-<arch>[-secureboot].tar` (e.g. `installer-amd64.tar`) is a custom Talos Linux installer image for `metal` platform (including system extensions if configured)
* `<platform>-installer-<arch>[-secureboot].tar` (e.g. `aws-installer-amd64.tar`) is a custom Talos Linux installer image for specific platform (including system extensions if configured)
* disk images in different formats (see Talos Linux documentation for a full list):
  * `metal-<arch>[-secureboot].raw.xz` (e.g. `metal-amd64.raw.xz`) - raw disk image for metal platform
  * `aws-<arch>.raw.xz` (e.g. `aws-amd64.raw.xz`) - raw disk image for AWS platform, that can be imported as an AMI
  * `gcp-<arch>.raw.tar.gz` (e.g. `gcp-amd64.raw.tar.gz`) - raw disk image for GCP platform, that can be imported as a GCE image
  * ... other support image types

#### Checksums

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Appending a checksum suffix to any `:path` returns a checksum file instead of the asset itself.

| Suffix    | Algorithm | Verify with    |
| --------- | --------- | -------------- |
| `.sha256` | SHA-256   | `sha256sum -c` |
| `.sha512` | SHA-512   | `sha512sum -c` |

The response is a single line in the standard checksum tool format:

```text
<hexhash>  <filename>
```

Download an image and verify its integrity:

```shell
curl -LO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz
curl -LO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz.sha256
sha256sum -c metal-amd64.raw.xz.sha256
```

Using `curl -JLO` (filename from `Content-Disposition` header):

```shell
curl -JLO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz
curl -JLO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz.sha256
sha256sum -c metal-amd64.raw.xz.sha256
```

With `wget`:

```shell
wget https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz
wget https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz.sha256
sha256sum -c metal-amd64.raw.xz.sha256
```

#### Signatures

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Appending a `.sigstore.json` suffix to any `:path` returns a detached Sigstore bundle instead of the asset itself.
The response is a Sigstore bundle v0.3 JSON document with the `application/vnd.dev.sigstore.bundle.v0.3+json` content type.

Download an image and its signature bundle:

```shell
curl -LO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz
curl -LO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz.sigstore.json
```

Using `curl -JLO` (filename from `Content-Disposition` header):

```shell
curl -JLO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz
curl -JLO https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz.sigstore.json
```

With `wget`:

```shell
wget https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz
wget https://factory.talos.dev/image/<schematic>/<version>/metal-amd64.raw.xz.sigstore.json
```

Verify a bundle produced by key-based cache signing:

```shell
curl -LO https://factory.talos.dev/oci/cosign/signing-key.pub
cosign verify-blob \
  --key signing-key.pub \
  --bundle metal-amd64.raw.xz.sigstore.json \
  --insecure-ignore-tlog \
  metal-amd64.raw.xz
```

Verify a bundle produced by Google Service Account keyless signing:

```shell
cosign verify-blob \
  --bundle metal-amd64.raw.xz.sigstore.json \
  --certificate-identity <service-account-email> \
  --certificate-oidc-issuer https://accounts.google.com \
  metal-amd64.raw.xz
```

### `GET /versions`

Access: `public`.

Returns a list of Talos Linux versions available for image generation.

```json
["v1.5.0","v1.5.1", "v1.5.2"]
```

### `GET /version/:version/extensions/official`

Access: `public`.

Returns a list of official system extensions available for the specified Talos Linux version.

```json
[
  {
    "name": "siderolabs/amd-ucode",
    "ref": "ghcr.io/siderolabs/amd-ucode:20230804",
    "digest": "sha256:761a5290a4bae9ceca11468d2ba8ca7b0f94e6e3a107ede2349ae26520682832",
  },

]
```

### `GET /version/:version/overlays/official`

Access: `public`.

Returns a list of official overlays available for the specified Talos Linux version.

```json
[
  {
    "name": "rpi_generic",
    "image": "siderolabs/sbc-raspberrypi",
    "ref": "ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0",
    "digest": "sha256:849ace01b9af514d817b05a9c5963a35202e09a4807d12f8a3ea83657c76c863",
  },

]
```

### `GET /talosctl/:version`

Access: `public`.

Returns a list of download URLs for `talosctl` binaries for the specified Talos Linux version.

* `:version` is a Talos Linux version, e.g. `v1.11.0`

`talosctl` downloads are available for Talos versions **v1.11.0** and later.

```json
[
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-darwin-amd64",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-darwin-arm64",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-freebsd-amd64",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-freebsd-arm64",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-linux-amd64",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-linux-arm64",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-linux-armv7",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-windows-amd64.exe",
  "https://factory.talos.dev/talosctl/v1.11.0/talosctl-windows-arm64.exe"
]
```

### `GET /talosctl/:version/:path`

Access: `public`.

Download a `talosctl` binary for the specified Talos Linux version and platform/architecture.

* `:version` is a Talos Linux version, e.g. `v1.11.0`
* `:path` is a binary name, e.g. `talosctl-linux-amd64` (use `GET /talosctl/:version` to list available paths)

### `GET /secureboot/signing-cert.pem`

Access: `public`.

Returns PEM-encoded SecureBoot signing certificate used by the Image Factory.

It might be used to manually enroll the certificate into the UEFI firmware.
Talos Linux SecureBoot ISOs come with an option for automatic enrollment of the certificate, but if that is not desired, the certificate can be manually enrolled.

### `GET /llms.txt`

Access: `public`.

Returns a `llms.txt` file describing Image Factory's API for LLM agents and AI tooling.

## PXE Frontend API

The PXE frontend provides an [iPXE script](https://ipxe.org/scripting) that automatically downloads and boots Talos Linux.
The bare metal machine should be configured to boot from the URL provided by this API, e.g.:

```text
#!ipxe
chain --replace --autofree https://pxe.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/v1.5.0/metal-${buildarch}
```

### `GET /pxe/:schematic/:version/:path`

Returns an iPXE script which downloads and boots Talos Linux with the specified schematic and Talos Linux version, architecture and platform.

Access:

| Auth     | `?token=`    | Machine scope | Ownership |
| -------- | ------------ | ------------- | --------- |
| required | not accepted | denied        | enforced  |

The script embeds the schematic's kernel command line, which is schematic-derived customization, so a machine-scoped token is denied here for the same reason it cannot read a schematic definition.
Download tokens are not accepted either.
iPXE cannot send request headers, so an authenticated boot puts the credential in the URL: `https://<user>:<password>@pxe.talos.dev/pxe/...`.

* `:schematic` is a schematic ID returned by `POST /schematic`
* `:version` is a Talos Linux version, e.g. `v1.5.0`
* `:path` is a `<platform>-<arch>[-secureboot]` path, e.g. `metal-amd64`

In non-SecureBoot schematic, the following iPXE script is returned:

```text
#!ipxe
kernel https://pxe.talos.dev/image/:schematic/:version/kernel-<arch> <kernel-cmdline>
initrd https://pxe.talos.dev/image/:schematic/:version/initramfs-<arch>.xz
boot
```

For SecureBoot schematic, the following iPXE script is returned:

```text
#!ipxe
kernel https://pxe.talos.dev/image/:schematic/:version/<platform>-<arch>-secureboot.uki.efi
boot
```

## OCI Registry Frontend API

The Talos Linux `installer` image is used for the initial install and upgrades.
It can be pulled from the Image Factory OCI registry.
If the image hasn't been created yet, it will be built on demand automatically.

Access, for every schematic-scoped `/v2/` route in this section, including `docker pull` and referrer discovery:

| Auth     | `?token=`    | Machine scope | Ownership |
| -------- | ------------ | ------------- | --------- |
| required | not accepted | allowed       | enforced  |

Registry clients speak Basic auth, so an Auth0 JWT is presented as the Basic password with any username.

Two routes in this section differ from the table:

* `GET`, `HEAD` `/v2` is the OCI ping.
  It takes a credential, and answers with a `401` challenge without one, but carries no schematic, so ownership does not apply.
* The [Source Image Proxy](#source-image-proxy) carries no schematic either; see the table in that section.

### Legacy `installer` Image

#### `docker pull <registry>/installer[-secureboot]/<schematic>:<version>`

Example: `docker pull factory.talos.dev/installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`

### `installer` Image

#### `docker pull <registry>/<platform>-installer[-secureboot]/<version>`

Examples:

* `docker pull factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`
* `docker pull factory.talos.dev/aws-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`

Pulls the Talos Linux `installer` image with the specified schematic and Talos Linux version.
The image platform (architecture) will be determined by the architecture of the Talos Linux Linux machine.

Enterprise Installer images for stable Talos 1.13.0 and newer releases also publish per-platform SPDX SBOM attestations and index-level SLSA provenance through OCI referrers.
Talos 1.13 prereleases are outside the evidence contract.
Image Factory uses the native OCI 1.1 API when available and the OCI referrers-tag schema for registries such as `registry:2` and `registry:3`.
See [Enterprise Installer Build Evidence v1](attestations/installer-build-v1.md) for the evidence contract and verification commands.

#### `GET /v2/<installer>/<schematic>/referrers/<digest>`

Discovers referrers for a generated Installer index or platform manifest.
The response is an OCI image index regardless of whether the backing registry uses the native referrers API or the referrers-tag schema.
The standard `artifactType` query parameter filters the returned descriptors.

#### `latest` Tag Resolution

The `latest` tag automatically resolves to the latest stable (non-prerelease) Talos Linux version available:

* `docker pull factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest`

This is equivalent to pulling with an explicit stable version, ensuring that prerelease versions (e.g. `v1.5.0-alpha.1`) are not used.

### Source Image Proxy

Access:

| Auth     | `?token=`    | Machine scope | Ownership      |
| -------- | ------------ | ------------- | -------------- |
| required | not accepted | allowed       | not applicable |

These are upstream Talos Linux images forwarded as-is, with nothing schematic-derived in them, so there is no `owner` to match and authentication alone gates them.
Any authenticated caller can pull any proxied source image.

Image Factory proxies the core Talos Linux source images from its backing registry under the `siderolabs/` prefix.
The images are pulled through the backing registry specified with the `artifacts.core.registry` option.
This feature only works if the backing registry is insecure (a local pull-through cache registry is recommended).
The image mapping is defined via the `artifacts.core.components` option.
This lets clients pull them through the factory.
Images are forwarded as-is, keeping their original signatures.

#### `docker pull <registry>/siderolabs/<image>:<version>`

`<image>` is one of:

* `installer-base`
* `installer`
* `imager`
* `extensions`
* `overlays`
* `talosctl-all`

Example: `docker pull factory.talos.dev/siderolabs/installer-base:v1.13.5`

### `GET /oci/cosign/signing-key.pub`

Access: `public`.

Returns the PEM-encoded public key used to sign the Talos Linux `installer` images and detached asset bundles when key-based cache signing is configured.

The key can be used to verify the installer images with `cosign`:

```shell
cosign verify --offline --insecure-ignore-tlog --insecure-ignore-sct --key signing-key.pub factory.talos.dev/...
```
