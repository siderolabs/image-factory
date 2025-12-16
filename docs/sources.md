# Required Source Container Images

The Image Factory uses the following container images to build Talos Linux artifacts:

* `siderolabs/imager`: this is the base image which contains vanilla Talos Linux boot image artifacts.
  Image Factory determines the list of available Talos Linux versions by listing the tags of this image.
* `siderolabs/installer-base` (for Talos Linux >= 1.10.0, for older versions it is `siderolabs/installer`): this image contains the Talos Linux installer base image.
  The available tags should match the tag of `siderolabs/imager` image.
* `siderolabs/extensions`: catalog of available system extensions for Talos Linux.
  The available tags should match the tag of `siderolabs/imager` image.
* `siderolabs/overlays`: catalog of available overlays for Talos Linux.
  The available tags should match the tag of `siderolabs/imager` image.
* `siderolabs/talosctl-all` (since Talos Linux 1.11.0): provides the `talosctl` binaries for various architectures and operating systems.
* extension images and overlay images: referenced from the `extensions` and `overlays` catalogs, respectively.
  The available images and tags should match the catalog entries.

These images will be pulled from the registry specified with the `-image-registry` flag when running the Image Factory.
Default value of this flag is `ghcr.io`, so the `siderolabs/imager` translates to `ghcr.io/siderolabs/imager`.
If `-image-registry` is set to `my.registry`, then the `siderolabs/imager` translates to `my.registry/siderolabs/imager`.

Each input image should be signed with `cosign` and the Image Factory will verify the signatures before using the images.

When running air-gapped, the `-image-registry` should contain a copy of the aforementioned images, and their signatures.
For example, to copy the `ghcr.io/siderolabs/imager:v1.10.0` image and its signature to a local registry, you can use the following commands:

```bash
crane cp ghcr.io/siderolabs/imager:v1.10.0 my.registry/siderolabs/imager:v1.10.0
sig="$(crane digest ghcr.io/siderolabs/imager:v1.10.0 | sed 's/:/-/').sig"
crane cp ghcr.io/siderolabs/imager:$sig my.registry/siderolabs/imager:$sig
```
