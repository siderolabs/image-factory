# API

This reference covers the user-facing HTTP, PXE and OCI registry APIs.
It intentionally excludes the HTML UI and its supporting routes and static assets, as well as the operational health, readiness and metrics endpoints.

## Authentication

Authentication is an Enterprise feature, enabled with `authentication.enabled`.
When it is off, every registered endpoint in this reference is public, except that the token and JWKS routes are not registered at all.
When it is on, each endpoint below carries an **Access** table with these four fields; endpoints that never take a credential are marked `Access: public` instead.
See [Authentication](authentication.md) for the providers, the token formats and the scopes.

With [browser login](authentication.md#browser-login) configured, every endpoint below also accepts the session cookie, and a browser navigation without a credential is redirected to `/login` rather than answered `401`.
A client that does not ask for `text/html` gets the `401` described here.

| Field | Values |
| --- | --- |
| **Auth** | `public` takes no credential, `required` takes an [`Authorization` header](authentication.md#the-authorization-header) for the configured provider, and a missing or invalid credential is `401`. |
| **Scopes** | Which [API token](authentication.md#api-tokens) capabilities may call the endpoint, or `none` when only a full provider credential can. |
| **Ownership** | Whether the schematic's `owner` must match the caller identity, where a mismatch is `403`; an unauthenticated caller gets `401` whether or not the schematic exists, so existence is not leaked. |

The factory recognizes eight atomic, resource-first scopes: `image:read`, `source:pull`, `schematic:create`, `schematic:read`, `report:read`, `token:issue`, `token:read` and `token:revoke`.
A single code-defined map controls what each reaches; see [Scopes](authentication.md#scopes).
A self-issued credential carries scopes and is limited to the endpoints those scopes allow.
Auth0 and htpasswd provider credentials have no Image Factory scopes and receive full provider access.

## Common HTTP behavior

Responses handled directly by the factory include a `Server` header and an `X-Request-ID` correlation ID.
An incoming `X-Request-ID` is echoed; otherwise the factory generates one.
Unless an endpoint or proxied OCI response defines another representation, errors are plain-text bodies with a trailing newline.

The common error statuses are:

| Status | Meaning |
| ------ | ------- |
| `400 Bad Request` | An input explicitly classified as an invalid path, architecture, report format, image profile or schematic is invalid. |
| `401 Unauthorized` | Authentication is enabled and the credential is missing or invalid. |
| `402 Payment Required` | The requested Enterprise-only asset feature is not enabled. |
| `403 Forbidden` | The credential is valid but its API-token scopes or schematic ownership deny access. |
| `404 Not Found` | The schematic, Talos artifact or API/OCI route does not exist. |
| `500 Internal Server Error` | The request failed unexpectedly. |
| `503 Service Unavailable` | A required proxy or Enterprise service is temporarily unavailable. |

Some core handlers currently wrap malformed semantic versions as unclassified errors and therefore return `500 Internal Server Error`, while the Enterprise SPDX, VEX and scan handlers classify malformed or too-old versions as `400 Bad Request`.
This distinction is part of the current behavior to preserve or deliberately correct in the future OpenAPI-backed implementation.

For configured cross-origin callers, CORS preflight advertises only `GET`, `HEAD` and `OPTIONS`.
It permits the `Cache-Control` request header and exposes `Content-Disposition`, `Content-Length` and `Content-Type` response headers.

## Edition and feature availability

| Feature | Community build or disabled-feature behavior |
| ------- | -------------------------------------------- |
| `/spdx`, `/vex` and `/scans` | Enterprise plugins are not registered, so the routes return `404 Not Found`. |
| `/tokens` and `/.well-known/jwks.json` | Registered only in an Enterprise build with authentication enabled; otherwise the routes return `404 Not Found`. |
| `/image/.../<asset>.sha256` and `.sha512` suffixes | The image route remains registered but returns `402 Payment Required` without Enterprise checksum support. |
| `/image/.../<asset>.sigstore.json` suffix | The image route remains registered but returns `402 Payment Required` when Enterprise asset signing is unavailable. |

## Enterprise Frontend API

### `GET /spdx/:schematic/:version/:arch`, `HEAD /spdx/:schematic/:version/:arch`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `report:read` | enforced |

Returns an SPDX 2.3 JSON document containing all packages from Talos Linux and the configured system extensions for the given schematic and version.
The response is a JSON-encoded SPDX document which can be consumed directly by vulnerability scanners such as grype:

* `:version` accepts a Talos version with or without the leading `v`.
* `:arch` is `amd64` or `arm64`.
* A successful response is `200 OK` with `Content-Type: application/spdx+json`, `Content-Length` and an attachment `Content-Disposition`.
* `HEAD` returns the same status and headers without the response body.

```shell
grype sbom:response.spdx.json
```

SPDX bundles are available for Talos versions **v1.11.0** and later.

### `GET /vex/:version/vex.json`, `HEAD /vex/:version/vex.json`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `report:read` | not applicable |

The document describes a Talos Linux release, not a schematic: it is identical for every caller and contains nothing derived from a schematic, so there is no `owner` to match.
A full provider credential or a token carrying `report:read` can therefore read it for any version.

Returns a VEX JSON document containing vulnerability information for all packages in the Talos Linux release.
The response is a JSON-encoded VEX document which can be consumed directly by vulnerability scanners such as grype:

* `:version` accepts a Talos version with or without the leading `v`.
* VEX documents are available for Talos versions **v1.13.0** and later.
* A successful response is `200 OK` with `Content-Type: application/json`, `Content-Length` and an attachment `Content-Disposition`.
* `HEAD` returns the same status and headers without the response body.

```shell
grype sbom:response.spdx.json --vex response.vex.json
```

### `GET /scans/:schematic/:version/:arch/:report`, `HEAD /scans/:schematic/:version/:arch/:report`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Access:

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `report:read` | enforced |

Returns a vulnerability scan report for the specified schematic, Talos Linux version and architecture.

* `:version` accepts a Talos version with or without the leading `v`.
* `:arch` is `amd64` or `arm64`.
* `:report` is a filename whose extension selects the report format, for example `report.sarif`.
* Scan reports are available for Talos versions **v1.13.0** and later.
* A successful response is `200 OK` with `Content-Length` and an attachment `Content-Disposition`.
* `HEAD` returns the same status and headers without the response body.

Supported report formats:

| Extension | Response content type            | Format                                                               |
| --------- | -------------------------------- | -------------------------------------------------------------------- |
| `.json`   | `application/json`               | JSON-encoded report in the format provided by the underlying scanner |
| `.table`  | `text/plain; charset=utf-8`      | Human-readable table                                                 |
| `.sarif`  | `application/sarif+json`         | SARIF                                                                |
| `.cdx`    | `application/vnd.cyclonedx+json` | CycloneDX                                                            |

### `POST /tokens`, `GET /tokens`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).
> These routes are registered only when authentication is enabled.

Access:

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `POST`: `token:issue`; `GET`: `token:read` | not applicable |

Mints and lists [API tokens](authentication.md#api-tokens): self-issued JWTs whose scopes decide what they may reach.
A token is issued to the calling identity unless a CLI bootstrap credential names another in `subject`; see [Minting for another identity](authentication.md#minting-for-another-identity).

`POST` takes a JSON body and answers `200 OK` with `Content-Type: application/json` and `Cache-Control: no-store`:

```json
{"name": "rack-3", "scopes": ["image:read"], "ttl": "8760h", "stored": true}
```

The browser UI sends an `actor` instead of `scopes`.
The server expands `talos`, `automation`, `operator` or `admin` to its fixed [actor profile](authentication.md#browser-actor-profiles), including that profile's delegation ceiling.
`actor` cannot be combined with `scopes` or `issuable_scopes`; an unknown actor is `400`.
Automation can issue Talos credentials and replacement Automation credentials; Admin can issue every actor profile.

```json
{
  "id": "0d1c...",
  "name": "rack-3",
  "scopes": ["image:read"],
  "issuable_scopes": [],
  "token": "eyJ...",
  "org_id": "org_abc123",
  "created_at": "2026-08-31T09:00:00Z",
  "expires_at": "2027-08-31T09:00:00Z",
  "stored": true
}
```

`token` is returned exactly once, here; it is never stored and cannot be read back.

`scopes` contains the child's executable capabilities.
`issuable_scopes` is optional and independently limits what that child may grant later.
Supplying it requires `token:issue` in `scopes`.
For API-token callers, both lists must be subsets of the parent's `issuable_scopes`; provider credentials may choose from the current code-defined catalog for their own identity.
The API never grants `any_subject`.

`stored` decides whether the factory records the token, and so whether `GET /tokens` will list it and `POST /tokens/:id/revoke` can take it back.
It is optional and defaults to `true`, so a caller who omits it never ends up with a credential nobody can withdraw.
The response echoes what was recorded.

An ephemeral token is the only kind accepted from a `?token=` query parameter, and the only kind that may be minted without a `name`.
There is no listing for a name to distinguish it in.

`ttl` is optional and is a Go duration.
It must fall within `authentication.tokens.ttl.stored` or `.ephemeral`, selected by `stored`, and omitting it takes that policy's default.

A missing `name` on a stored token, an unknown executable or issuable scope, an `issuable_scopes` list without `token:issue`, a malformed `ttl`, and a lifetime outside the selected bounds are each `400`.

A `subject` naming another identity is `403` unless the caller is the CLI bootstrap credential, and a `subject` that is not a single-line value of at most 256 bytes is `400`.

`GET` returns the tokens the caller's organization can still revoke:

```json
{"tokens": [{"id": "0d1c...", "name": "rack-3", "scopes": ["image:read"], "issuable_scopes": [], "created_at": "...", "expires_at": "..."}]}
```

Ephemeral tokens never appear because nothing records them.
Expired stored tokens are omitted and no longer count against `authentication.tokens.maxPerOrg`; a `POST` that would exceed the active-token cap is `409`.

### `POST /tokens/:id/revoke`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).
> This route is registered only when authentication is enabled.

Access:

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `token:revoke` | not applicable |

Takes the token with `:id` out of circulation.
`:id` is the `id` from `POST /tokens`, and only tokens belonging to the caller's organization can be named; an unknown one is `404`.
A successful response is `204 No Content`.
Every verification reads the token's deterministic registry tag, so a successful revocation is visible to every replica immediately.

### `GET /.well-known/jwks.json`

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).
> This route is registered only when authentication is enabled.

Access: `public`.

Returns the JSON Web Key Set containing every configured ECDSA P-256 verification key in configuration order, so that a proxy can verify tokens issued during a key rotation without holding a private key.
The first configured key is the active signer; later keys are verification-only.
The response has `Content-Type: application/json`.

## HTTP Frontend API

### `POST /schematics`

Create a new image schematic.

Access:

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `schematic:create` | sets `owner` |

The request body is a YAML or JSON encoded schematic description.
Clients should send `Content-Type: application/yaml` for YAML.
The current handler parses the body as YAML, of which JSON is a subset, and does not select parsing behavior from the `Content-Type` header.
Unknown fields are rejected.

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

Schematic fields:

| Field | Type and constraints |
| ----- | -------------------- |
| `owner` | Enterprise-only string; when authentication is enabled, the factory sets it to the caller identity and rejects a conflicting supplied value. |
| `customization.embeddedMachineConfiguration` | String containing YAML-encoded Talos machine configuration documents. |
| `customization.extraKernelArgs` | Array of strings. |
| `customization.meta` | Array of objects with an unsigned 8-bit integer `key` (`0` through `255`) and string `value`. |
| `customization.systemExtensions.officialExtensions` | Array of official extension names. |
| `customization.bootloader` | `auto`, `dual-boot`, `grub` or `sd-boot`; an omitted value uses automatic selection. |
| `customization.secureboot.enrollKeys` | `off`, `manual`, `if-safe` or `force`; defaults to `if-safe`. |
| `customization.secureboot.includeWellKnownCertificates` | Boolean. |
| `customization.diskImage.sectorSize` | Unsigned integer number of bytes; defaults to `512` when omitted. |
| `overlay.image` | Overlay container image string. |
| `overlay.name` | Overlay name string. |
| `overlay.options` | Optional free-form YAML/JSON object interpreted by the selected overlay. |

Output is a JSON object containing the schematic ID and the canonical schematic body as YAML:

```json
{
  "id": "2a63b6e7dab90ec9d44f213339b9545bd39c6499b22a14cf575c1ca4b6e39ff8",
  "schematic": "customization:\n    extraKernelArgs:\n        - vga=791\n"
}
```

The `schematic` field is the canonical representation used to compute the ID.
Callers should treat it as authoritative, since the factory may modify or add fields to the submitted schematic, for example setting `owner` for authenticated requests.
A successful response is `201 Created` with `Content-Type: application/json`.

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

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `schematic:read` | enforced |

If the schematic is found, the response body contains the YAML-encoded schematic representation.
The successful response is `200 OK` with `Content-Type: application/yaml`.
Otherwise a `404 Not Found` status code is returned.

### `GET /image/:schematic/:version/:path`, `HEAD /image/:schematic/:version/:path`

Download a Talos Linux boot image with the specified schematic and Talos Linux version.

Access:

| Auth | Scopes | Ownership |
| --- | --- | --- |
| required | `image:read` | enforced |

* `:schematic` is a schematic ID returned by `POST /schematics`
* `:version` is a Talos Linux version, e.g. `v1.5.0`
* `:path` is a specific image path (details below)

The optional `filename` query parameter overrides the download filename used to build and serve the asset.
On a direct response, the factory returns `200 OK` with `Content-Type`, `Content-Length` and an attachment `Content-Disposition`.
A `GET` may instead return `302 Found` to a configured object-storage or CDN URL; `HEAD` is always served directly and returns the same headers as a direct `GET`, without a body.

In Enterprise edition this route also accepts an ephemeral `image:read` [API token](authentication.md#api-tokens) as `?token=<token>` in place of the `Authorization` header, as does `GET /pxe/`; see [The `?token=` query parameter](authentication.md#the-token-query-parameter).

Common parameters:

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
  * ... other supported image types

#### Checksums

> [!NOTE]
> Enterprise feature: requires [Enterprise Image Factory](https://docs.siderolabs.com/talos/latest/learn-more/enterprise-image-factory).

Appending a checksum suffix to any `:path` returns a checksum file instead of the asset itself.
The successful response is `200 OK` with `Content-Type: text/plain; charset=utf-8`, `Content-Length` and an attachment `Content-Disposition`.
`HEAD` returns the same headers without the body.
The `filename` query override changes the attachment filename and the filename written into the checksum line.
Without Enterprise checksum support, these suffixes return `402 Payment Required`.

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
It also includes `Content-Length` and an attachment `Content-Disposition`; `HEAD` returns those headers without the body.
The `filename` query override changes the attachment filename.
When Enterprise asset signing is unavailable, the suffix returns `402 Payment Required`.

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

The optional `broken=true` query parameter returns the separately configured list of Talos versions marked as broken instead of the available versions.
Other values are treated as absent.

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
    "author": "<extension author>",
    "description": "<extension description>"
  }
]
```

### `GET /version/:version/overlays/official`

Access: `public`.

Returns a list of official overlays available for the specified Talos Linux version.
Talos versions that predate overlay support return an empty JSON array.

```json
[
  {
    "name": "rpi_generic",
    "image": "siderolabs/sbc-raspberrypi",
    "ref": "ghcr.io/siderolabs/sbc-raspberrypi:v0.1.0",
    "digest": "sha256:849ace01b9af514d817b05a9c5963a35202e09a4807d12f8a3ea83657c76c863"
  }
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

### `GET /talosctl/:version/:path`, `HEAD /talosctl/:version/:path`

Access: `public`.

Download a `talosctl` binary for the specified Talos Linux version and platform/architecture.

* `:version` is a Talos Linux version, e.g. `v1.11.0`
* `:path` is a binary name, e.g. `talosctl-linux-amd64` (use `GET /talosctl/:version` to list available paths)

A successful response is `200 OK` with `Content-Length` and an attachment `Content-Disposition`.
`HEAD` returns the same status and headers without the binary body.

### `GET /secureboot/signing-cert.pem`

Access: `public`.

Returns PEM-encoded SecureBoot signing certificate used by the Image Factory.
The response has `Content-Type: application/x-pem-file`.

It might be used to manually enroll the certificate into the UEFI firmware.
Talos Linux SecureBoot ISOs come with an option for automatic enrollment of the certificate, but if that is not desired, the certificate can be manually enrolled.

### `GET /llms.txt`

Access: `public`.

Returns a `llms.txt` file describing Image Factory's API for LLM agents and AI tooling.
The response has `Content-Type: text/plain; charset=utf-8`.

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

| Auth | Scopes | Ownership |
| -------- | ------ | --------- |
| required | `image:read` | enforced |

The script is a transport for a generated schematic-derived image and therefore uses `image:read`, the same capability as generated installer downloads and OCI pulls.
Ownership remains enforced.

iPXE cannot send request headers, so the credential goes in the URL, and whichever one authenticated the request is carried into the asset URLs of the returned script:

* an [ephemeral download token](authentication.md#the-token-query-parameter) - `https://pxe.talos.dev/pxe/...?token=<token>` - is forwarded as `?token=` on the kernel, initramfs and UKI URLs, so no long-lived credential appears in the script.
  The script expires with the token; nothing is minted here, so re-fetching it with an expiring token does not extend that lifetime.
* Basic credentials - `https://<user>:<password>@pxe.talos.dev/pxe/...`, which the client encodes into `Authorization: Basic` - are embedded as userinfo on those same URLs.

Either credential makes the body worth stealing, so every authenticated response from this route is `Cache-Control: no-store`.

* `:schematic` is a schematic ID returned by `POST /schematics`
* `:version` is a Talos Linux version, e.g. `v1.5.0`
* `:path` is a `<platform>-<arch>[-secureboot]` path, e.g. `metal-amd64`

For a non-SecureBoot schematic, the following iPXE script is returned:

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

| Auth | Scopes | Ownership |
| --- | --- | --- |
| required | `image:read` | enforced |

Registry clients speak Basic auth, so an Auth0 JWT is presented as the Basic password with any username; see [The `Authorization` header](authentication.md#the-authorization-header).

Two routes in this section differ from the table:

* `GET`, `HEAD` `/v2` is the OCI ping.
  It returns an empty `200 OK`; when authentication is enabled it answers with a `401` challenge without a credential, but carries no schematic, so ownership does not apply.
* The [Source Image Proxy](#source-image-proxy) carries no schematic either; see the table in that section.

The registry implements the user-facing OCI Distribution `GET` and `HEAD` operations under `/v2/` for manifest, blob and referrer retrieval.
It also forwards `GET` and `HEAD` tag listing and referrer requests for configured source images.

Generated Installer repositories expose:

* `GET`, `HEAD /v2/<installer>/<schematic>/manifests/<reference>`
* `GET`, `HEAD /v2/<installer>/<schematic>/blobs/<digest>`
* `GET`, `HEAD /v2/<installer>/<schematic>/referrers/<digest>`

Manifest and blob requests return `307 Temporary Redirect` with `Location` when authentication is disabled and direct external-registry access is configured.
When authentication is enabled, or internal repository proxying is configured, the factory reverse-proxies the backing registry instead so the authentication boundary is not bypassed.
The proxied status, content type, body and OCI headers come from that registry, and the inbound `Authorization` header is not forwarded.

### Legacy `installer` Image

#### `docker pull <registry>/installer[-secureboot]/<schematic>:<version>`

Example: `docker pull factory.talos.dev/installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`

### `installer` Image

#### `docker pull <registry>/<platform>-installer[-secureboot]/<schematic>:<version>`

Examples:

* `docker pull factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`
* `docker pull factory.talos.dev/aws-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`

Pulls the Talos Linux `installer` image with the specified schematic and Talos Linux version.
The image platform (architecture) will be determined by the architecture of the Talos Linux machine.

Enterprise Installer images for stable Talos 1.13.0 and newer releases also publish per-platform SPDX SBOM attestations and index-level SLSA provenance through OCI referrers.
Talos 1.13 prereleases are outside the evidence contract.
Image Factory uses the native OCI 1.1 API when available and the OCI referrers-tag schema for registries such as `registry:2` and `registry:3`.
See [Enterprise Installer Build Evidence v1](attestations/installer-build-v1.md) for the evidence contract and verification commands.

#### `GET /v2/<installer>/<schematic>/referrers/<digest>`, `HEAD /v2/<installer>/<schematic>/referrers/<digest>`

Discovers referrers for a generated Installer index or platform manifest.
The response is an OCI image index regardless of whether the backing registry uses the native referrers API or the referrers-tag schema.
The standard `artifactType` query parameter filters the returned descriptors.
A successful response is `200 OK` with the OCI image-index content type, `Docker-Content-Digest` and `Content-Length`.
When `artifactType` is supplied, the response also includes `Oci-Filters-Applied: artifactType`.

#### `latest` Tag Resolution

The `latest` tag automatically resolves to the latest stable (non-prerelease) Talos Linux version available:

* `docker pull factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest`

This is equivalent to pulling with an explicit stable version, ensuring that prerelease versions (e.g. `v1.5.0-alpha.1`) are not used.

### Source Image Proxy

Access:

| Auth | Scopes | Ownership |
| --- | --- | --- |
| required | `source:pull` | not applicable |

These are upstream Talos Linux images forwarded as-is, with nothing schematic-derived in them, so there is no `owner` to match.
A full provider credential or an API token carrying `source:pull` can fetch any proxied source image.

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
* `image-factory`

Example: `docker pull factory.talos.dev/siderolabs/installer-base:v1.13.5`

The proxy forwards these OCI Distribution operations:

* `GET`, `HEAD /v2/siderolabs/<image>/manifests/<reference>`
* `GET`, `HEAD /v2/siderolabs/<image>/blobs/<digest>`
* `GET`, `HEAD /v2/siderolabs/<image>/tags/list`
* `GET`, `HEAD /v2/siderolabs/<image>/referrers/<digest>`

Query parameters are preserved, including `artifactType` on referrer discovery.
The proxied response status, content type, body and OCI headers come from the backing registry, and the inbound `Authorization` header is not forwarded.

### `GET /oci/cosign/signing-key.pub`

Access: `public`.

Returns the PEM-encoded public key used to sign the Talos Linux `installer` images and detached asset bundles when key-based cache signing is configured.
The response has `Content-Type: application/x-pem-file`.

The key can be used to verify the installer images with `cosign`:

```shell
cosign verify --offline --insecure-ignore-tlog --insecure-ignore-sct --key signing-key.pub factory.talos.dev/...
```
