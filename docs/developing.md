# Developing Image Factory

Run integration tests in local mode, with registry mirrors:

```bash
make integration TEST_FLAGS="-test.image-registry=127.0.0.1:5004 -test.schematic-service-repository=127.0.0.1:5005/image-factory/schematic -test.installer-external-repository=127.0.0.1:5005/test -test.installer-internal-repository=127.0.0.1:5005/test -test.cache-repository=127.0.0.1:5005/image-factory/cache" REGISTRY=127.0.0.1:5005
```

In order to run the Image Factory, generate a ECDSA key pair:

```bash
openssl ecparam -name prime256v1 -genkey -noout -out cache-signing-key.key
```

Run the Image Factory using the following config:

```yaml
artifacts:
  # registry mirror for ghcr.io
  core:
    registry: 127.0.0.1:5004
  
  # private registry repository for schematics
  #
  # resolves to 127.0.0.1:5005/image-factory/schematic
  schematic:
    registry: 127.0.0.1:5005
    namespace: image-factory
    repository: schematic
  
  installer:
    # internal registry namespace to push installer images to
    internal:
      registry: 127.0.0.1:5005
      namespace: siderolabs
    
    # external registry namespace to redirect users to pull installer
    external:
      registry: 127.0.0.1:5005
      namespace: siderolabs

cache:
  oci:
    # private registry repository for cached assets
    registry: 127.0.0.1:5005
    namespace: image-factory
    repository: cache

  # path to the ECDSA private key (to sign cached assets)
  signingKeyPath: ./cache-signing-key.key

http:
  # external URL the Image Factory is available at
  externalURL: https://example.com/
```
