# Air-gapped Mode

The Image Factory can be run in air-gapped mode, where it uses a local registry to pull the required images and their signatures.

Let's assume that the local registry is running at `localhost:5000`, and official images from `ghcr.io/siderolabs` are copied to it.
Use the script [`hack/copy-artifacts.sh`](../hack/copy-artifacts.sh) to copy the required images and their signatures to the local registry, replacing `TALOS_VERSION` with the desired Talos Linux version:

```bash
hack/copy-artifacts.sh ghcr.io localhost:5000 TALOS_VERSION
```

If you need to copy more versions, you can run the script multiple times with different Talos Linux versions.

After that, you can run the Image Factory with the following config to use the local registry for pulling the required images and their signatures.

```yaml
artifacts:
  core:
    registry: localhost:5000
```

If the local registry prefixes the upstream repository path (e.g. a Harbor proxy-cache project for `ghcr.io` named `ghcrio`), set the namespace so it is prepended to every pulled image, including the extension and overlay images referenced by the manifests:

```yaml
artifacts:
  core:
    registry: harbor.example.com
    namespace: ghcrio
```
