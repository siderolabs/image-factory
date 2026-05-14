# Developing Image Factory

## Running with Docker Compose

`docker-compose-{up|down}` make targets are available for running Image Factory.

To run:

```bash
# Generate signing key
openssl ecparam -name prime256v1 -genkey -noout -out _out/cache-signing-key.key

# Build and run
make docker-compose-up REGISTRY=127.0.0.1:5005

# Build and run (enterprise)
make docker-compose-up REGISTRY=127.0.0.1:5005 WITH_ENTERPRISE=true
```

To stop:

```bash
make docker-compose-down
```

By default, authentication is disabled for local development, enable it by setting `authentication.enabled` to `true` in the config file (`hack/dev/config.yaml`), and providing a valid `htpasswd` file at the path specified by `authentication.htpasswdPath`.
Optionally use the existing htpasswd file at `hack/dev/htpasswd` for testing.

## Running Image Factory Manually

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

## Running Integration Tests

Integration tests can be run with specific targets:

- `integration-direct`
- `integration-s3`
- `integration-cdn`
- `integration-proxy-installer`
- `integration-enterprise`

Example running direct integration tests with registry mirrors
(`127.0.0.1:5004` is a registry mirror for `ghcr.io`, `127.0.0.1:5100` is an ephemeral local registry brought up by `make` automatically, and `127.0.0.1:5005` is a local registry for pushing images):

```bash
make integration-direct TEST_FLAGS="-test.image-registry=127.0.0.1:5004 -test.schematic-service-repository=127.0.0.1:5100/image-factory/schematic -test.installer-external-repository=127.0.0.1:5100/test -test.installer-internal-repository=127.0.0.1:5100/test -test.cache-repository=127.0.0.1:5100/image-factory/cache" REGISTRY=127.0.0.1:5005
```

A test focus can be set with:

```bash
...  RUN_TESTS_DIRECT='TestIntegration/Schematic'
```

For Enterprise tests, use the following command:

```bash
make integration-enterprise TEST_FLAGS="-test.image-registry=127.0.0.1:5004 -test.schematic-service-repository=127.0.0.1:5100/image-factory/schematic -test.installer-external-repository=127.0.0.1:5100/test -test.installer-internal-repository=127.0.0.1:5100/test -test.cache-repository=127.0.0.1:5100/image-factory/cache" REGISTRY=127.0.0.1:5005
```

(The only change is `s/integration-direct/integration-enterprise/` in the target name, and the test focus variable will be `RUN_TESTS_ENTERPRISE` instead of `RUN_TESTS_DIRECT`.)
