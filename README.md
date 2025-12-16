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

## Documentation

* [API reference](docs/api.md)
* [Configuration](docs/configuration.md)
* [Required Source Container Images](docs/sources.md)
* [Air-gapped Deployment](docs/air-gapped.md)
* [Cache](docs/cache.md)
* [Developing Image Factory](docs/developing.md)

## License

The Image Factory is licensed under the [Mozilla Public License, version 2.0](LICENSE), except for the code in the `enterprise/` folder,
which is licensed under the [Business Source License 1.1](enterprise/LICENSE).

The enterprise code is not included in the open source version of Image Factory, and it is not built by default.
