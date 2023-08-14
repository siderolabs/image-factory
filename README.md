# Image Service

Image Service provides a way to download Talos boot and install images generated with specific customizations.

The list of provided assets:

* ISO
* kernel/initramfs/kernel command line
* UKI
* disk images in various formats
* `installer` container images

Supported frontends:

* HTTP
* PXE service
* Container Registry

Official Image Service is available at [https://imager.talos.dev](https://imager.talos.dev).

## HTTP Frontend API

### `POST /configuration`

Create a new image configuration.

The request body is a YAML (JSON) encoded configuration description:

```yaml
customization:
    extraKernelArgs: # optional
        - vga=791
```

Output is JSON-encoded configuration ID:

```json
{"id":"2a63b6e7dab90ec9d44f213339b9545bd39c6499b22a14cf575c1ca4b6e39ff8"}
```

This ID can be used to download images with this configuration.

### `GET /image/:configuration/:version/:path`

Download a Talos boot image with the specified configuration and Talos version.

* `:configuration` is a configuration ID returned by `POST /configuration`
* `:version` is a Talos version, e.g. `v1.5.0`
* `:path` is a specific image path, details below

Common used parameters:

* `<arch>` image architecture: `amd64` or `arm64`
* `<platform>` Talos platform, e.g. `metal`, `aws`, `gcp`, etc.
* `<board>` is a board name (only for `arm64` `metal` platform), e.g. `rpi_generic`
* `-secureboot` identifies a Secure Boot asset

Supported image paths:

* `kernel-<arch>` (e.g. `kernel-amd64`) - raw kernel image
* `cmdline-<platform>[-<board>]-<arch>[-secureboot]` (e.g. `cmdline-metal-amd64`) - kernel command line
* `initramfs-<arch>.xz` (e.g. `initramfs-amd64.xz`) - initramfs image (including system extensions if configured)
* `<platform>-<arch>[-secureboot].iso` (e.g. `metal-amd64.iso`) - ISO image
* `<platform>-<arch>-secureboot-uki.efi` (e.g. `metal-amd64-secureboot-uki.efi) UEFI UKI image (Secure Boot compatible)
* `installer-<arch>[-secureboot].tar` (e.g. `installer-amd64.tar`) is a custom Talos installer image (including system extensions if configured)
* disk images in different formats (see Talos documentation for a full list):
  * `metal-<arch>[-secureboot].raw.xz` (e.g. `metal-amd64.raw.xz`) - raw disk image for metal platform
  * `aws-<arch>.raw.xz` (e.g. `aws-amd64.raw.xz`) - raw disk image for AWS platform, that can be imported as an AMI
  * `gcp-<arch>.raw.tar.gz` (e.g. `gcp-amd64.raw.tar.gz`) - raw disk image for GCP platform, that can be imported as a GCE image
  * ... other support image types

## PXE Frontend API

TBD

## OCI Registry Frontend API

TBD

## Development

Run integration tests in local mode, with registry mirrors:

```bash
make integration TEST_FLAGS=-test.image-prefix=127.0.0.1:5004/siderolabs/ REGISTRY=127.0.0.1:5005
```
