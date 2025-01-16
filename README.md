# Image Factory

The Image Factory provides a way to download Talos Linux artifacts.
Artifacts can be generated with customizations defined by a "schematic".
A schematic can be applied to any of the versions of Talos Linux offered by the Image Factory to produce a "model".

The following assets are provided:

* ISO
* kernel, initramfs, and kernel command line
* UKI
* disk images in various formats (e.g. AWS, GCP, VMware, etc.)
* `installer` container images

The supported frontends are:

* HTTP
* PXE
* Container Registry

The official Image Factory is available at [https://factory.talos.dev](https://factory.talos.dev).

## HTTP Frontend API

### `POST /schematics`

Create a new image schematic.

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
overlay: # optional
  image: ghcr.io/siderolabs/sbc-raspberry-pi # overlay image
  name: rpi_generic # overlay name
  options: # optional, any valid yaml, depends on the overlay implementation
    data: "mydata"
```

Output is a JSON-encoded schematic ID:

```json
{"id":"2a63b6e7dab90ec9d44f213339b9545bd39c6499b22a14cf575c1ca4b6e39ff8"}
```

This ID can be used to download images with this schematic.

Well-known schematic IDs:

* `376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba` - default schematic (without any customizations)

### `GET /image/:schematic/:version/:path`

Download a Talos Linux boot image with the specified schematic and Talos Linux version.

* `:schematic` is a schematic ID returned by `POST /schematic`
* `:version` is a Talos Linux version, e.g. `v1.5.0`
* `:path` is a specific image path (details below)

Common used parameters:

* `<arch>` image architecture: `amd64` or `arm64`
* `<platform>` Talos Linux platform, e.g. `metal`, `aws`, `gcp`, etc.
* `<board>` is a board name (only for `arm64` `metal` platform), e.g. `rpi_generic` # for talos versions >= v1.7.0 this is deprecated, use metal image instead
* `-secureboot` identifies a Secure Boot asset

Supported image paths:

* `kernel-<arch>` (e.g. `kernel-amd64`) - raw kernel image
* `cmdline-<platform>[-<board>]-<arch>[-secureboot]` (e.g. `cmdline-metal-amd64`) - kernel command line
* `initramfs-<arch>.xz` (e.g. `initramfs-amd64.xz`) - initramfs image (including system extensions if configured)
* `<platform>-<arch>[-secureboot].iso` (e.g. `metal-amd64.iso`) - ISO image
* `<platform>-<arch>[-secureboot]-uki.efi` (e.g. `metal-amd64-secureboot-uki.efi`) UEFI UKI image (Secure Boot compatible)
* `installer-<arch>[-secureboot].tar` (e.g. `installer-amd64.tar`) is a custom Talos Linux installer image (including system extensions if configured)
* disk images in different formats (see Talos Linux documentation for a full list):
  * `metal-<arch>[-secureboot].raw.xz` (e.g. `metal-amd64.raw.xz`) - raw disk image for metal platform
  * `aws-<arch>.raw.xz` (e.g. `aws-amd64.raw.xz`) - raw disk image for AWS platform, that can be imported as an AMI
  * `gcp-<arch>.raw.tar.gz` (e.g. `gcp-amd64.raw.tar.gz`) - raw disk image for GCP platform, that can be imported as a GCE image
  * ... other support image types

### `GET /versions`

Returns a list of Talos Linux versions available for image generation.

```json
["v1.5.0","v1.5.1", "v1.5.2"]
```

### `GET /version/:version/extensions/official`

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

### `GET /secureboot/signing-cert.pem`

Returns PEM-encoded SecureBoot signing certificate used by the Image Factory.

It might be used to manually enroll the certificate into the UEFI firmware.
Talos Linux SecureBoot ISOs come with an option for automatic enrollment of the certificate, but if that is not desired, the certificate can be manually enrolled.

## PXE Frontend API

The PXE frontend provides an [iPXE script](https://ipxe.org/scripting) that automatically downloads and boots Talos Linux.
The bare metal machine should be configured to boot from the URL provided by this API, e.g.:

```text
#!ipxe
chain --replace --autofree https://pxe.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/v1.5.0/metal-${buildarch}
```

### `GET /pxe/:schematic/:version/:path`

Returns an iPXE script which downloads and boots Talos Linux with the specified schematic and Talos Linux version, architecture and platform.

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

### `docker pull <registry>/installer[-secureboot]/<schematic>:<version>`

Example: `docker pull factory.talos.dev/installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:v1.5.0`

Pulls the Talos Linux `installer` image with the specified schematic and Talos Linux version.
The image platform (architecture) will be determined by the architecture of the Talos Linux Linux machine.

### `GET /oci/cosign/signing-key.pub`

Returns PEM-encoded public key used to sign the Talos Linux `installer` images.

The key can be used to verify the installer images with `cosign`:

```shell
cosign verify --offline --insecure-ignore-tlog --insecure-ignore-sct --key signing-key.pub factory.talos.dev/...
```

## Development

Run integration tests in local mode, with registry mirrors:

```bash
make integration TEST_FLAGS="-test.image-registry=127.0.0.1:5004 -test.schematic-service-repository=127.0.0.1:5005/image-factory/schematic -test.installer-external-repository=127.0.0.1:5005/test -test.installer-internal-repository=127.0.0.1:5005/test -test.cache-repository=127.0.0.1:5005/cache" REGISTRY=127.0.0.1:5005
```

In order to run the Image Factory, generate a ECDSA key pair:

```bash
openssl ecparam -name prime256v1 -genkey -noout -out cache-signing-key.key
```

Run the Image Factory passing the flags:

```text
-image-registry 127.0.0.1:5004 # registry mirror for ghcr.io
-external-url https://example.com/ # external URL the Image Factory is available at
-schematic-service-repository 127.0.0.1:5005/image-factory/schematic # private registry for schematics
-installer-internal-repository 127.0.0.1:5005/siderolabs # internal registry to push installer images to
-installer-external-repository 127.0.0.1:5005/siderolabs # external registry to redirect users to pull installer
-cache-repository 127.0.0.1:5005/cache # private registry for cached assets
-cache-signing-key-path ./cache-signing-key.key # path to the ECDSA private key (to sign cached assets)
```
