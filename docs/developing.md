# Developing Image Factory

## Running with Docker Compose

`docker-compose-{up|down}` make targets are available for running Image Factory.

To run:

```bash
# Generate signing key
openssl ecparam -name prime256v1 -genkey -noout -out _out/cache-signing-key.key

# Build and run
make docker-compose-up

# Build and run (enterprise)
make docker-compose-up WITH_ENTERPRISE=true

# Build and run with debug symbols
make docker-compose-up WITH_DEBUG=true
```

To stop:

```bash
make docker-compose-down
```

### Local overrides

If `hack/dev/compose.override.yaml` exists, `make docker-compose-{up|down}` passes it to Compose after `hack/dev/compose.yaml`.
This file is git-ignored.
Use it for local changes that should not be committed, such as extra environment variables, a different config file or a different port.

Configuration keys can be set with an `IF_`-prefixed environment variable (for example, `registry.debug` becomes `IF_REGISTRY_DEBUG`).
The config file is loaded after the environment, so this only works for keys that `hack/dev/config.yaml` does not already set; to change those, replace the config file as shown below.

```yaml
services:
  image-factory:
    environment:
      - IF_REGISTRY_DEBUG=true
```

To replace the whole configuration file instead:

```yaml
configs:
  if_config:
    file: ./config.local.yaml # relative to hack/dev/
```

See the [Compose merge rules](https://docs.docker.com/reference/compose-file/merge/) for how the two files are combined.

Local development enables authentication by default and uses the htpasswd provider.
The Compose configuration mounts `hack/dev/htpasswd` at the configured `/etc/image-factory/htpasswd` path.

When running `make docker-compose-up` or the integration tests, the extra extension catalog and an extra extension image and their signatures are automatically pushed to the local registry.
You can push the extra images manually with `push-extra-extensions` (requires [`crane`](https://github.com/google/go-containerregistry/tree/main/cmd/crane)), optionally setting the `EXTRA_EXTENSIONS_REGISTRY`.
The extra images are located under [internal/integration/testdata/extra-extensions/](../internal/integration/testdata/extra-extensions/).

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

## Browser UI

The browser UI is a Vue single-page app in [frontend/](../frontend/), served at `/` and `/ui/tokens`.

kres picks the frontend up from `frontend/package.json`: `make frontend` builds it, `make unit-tests-frontend` runs its tests, and `make lint-eslint` lints it.
The Docker build embeds the result through [internal/frontend/frontend.go](../internal/frontend/frontend.go); the committed `internal/frontend/dist/index.html` is only a placeholder, so that the Go code compiles without a frontend build.
Built files are served from `/assets/`, the files in `frontend/public/` (the favicons) at their own paths, and `index.html` is a Go template, filled in on every request with the configuration the UI needs.

The UI uses the [Talos design system](https://github.com/siderolabs/talos-design-system).
Components use its color roles (`bg-surface-card`), type roles (`type-body`) and spacing steps (`p-compact`) instead of raw palette values, so both themes come from the same classes.
The theme choice is stored in the browser and applied before first paint by `frontend/src/theme-init.ts`, which a Vite plugin inlines into `index.html`.
The language choice is stored in the browser too: the UI starts in the language chosen in its language picker or with a `?lang=` link, else the first of the browser's languages it has translations for, else English.

With Auth0 browser login, the backend lets signed-out visitors through to `/` and `/sign-in-error`, where the UI shows the landing and sign-in error pages; `authenticated` in the configuration in `index.html` tells it whether the visitor is signed in.

For UI development, run the Vite dev server and point Image Factory at it:

```bash
cd frontend
npm install
npm run serve
```

```yaml
http:
  spa:
    dev:
      serverURL: http://127.0.0.1:8121
```

`make docker-compose-up` runs the Vite dev server in the `node-dev` container and publishes it on port 8121.
`hack/dev/config.yaml` already points `http.spa.dev.serverURL` at `http://host.docker.internal:8121`.

To run Vite on the host instead, disable `node-dev` so that port 8121 is free, and run `npm run serve` as above; the URL stays the same.
On Linux, run it with `HOST=0.0.0.0 npm run serve`, as `host.docker.internal` is the Docker bridge address there rather than the host loopback.

```yaml
services:
  node-dev:
    profiles: [disabled]
```

Open the UI through Image Factory, not through Vite directly: Image Factory fills in `index.html` and forwards the source modules, hot reloading and the favicons to Vite.

## HTTP identity and operation ownership

Authentication publishes the authoritative typed `authn.Principal` to the request context.
Request middleware does not keep a separate authentication identity or capture a username through an authentication callback.
After request execution, audit projects the username from that principal into `audit.Record.Username`.
This audit-time projection is intentional: removing legacy middleware-local username capture does not mean removing authenticated identity from audit records.
The post-authentication request context is used, not the earlier context created before authentication.
Requests rejected before authentication do not acquire an identity; public requests retain their existing audit bypass.
Audit sink failures are logged and do not change the response.

`TestRequestMiddlewareOrderingAndTypedAudit` protects these rules alongside contract-before-auth ordering, streaming responses, and authenticated `no-store`.

Operation ownership checks compare runtime descriptors with the OpenAPI document, not an expected set inferred from those descriptors.
Community and Enterprise assemblies are checked with browser capability both disabled and enabled.
Every enabled operation must have exactly one owner; disabled and unknown operations must have none.
Enterprise contract annotations describe product availability: the token UI page remains registered in Community, and browser routes are registered only when the provider exposes browser capability.
These exceptions are explicit in the configuration tests.
Counterexamples exercise missing dispatched owners, duplicates, unknown operations, and disabled capabilities.

## Running Integration Tests

Integration tests can be run with specific targets:

- `integration-direct`
- `integration-s3`
- `integration-cdn`
- `integration-proxy-installer`
- `integration-enterprise`

Example running direct integration tests with registry mirrors (`127.0.0.1:5004` is a registry mirror for `ghcr.io`, `127.0.0.1:5100` is an ephemeral local registry brought up by `make` automatically, and `127.0.0.1:5005` is a local registry for pushing images):

```bash
make integration-direct TEST_FLAGS="-test.image-registry=127.0.0.1:5004 -test.schematic-service-repository=127.0.0.1:5100/image-factory/schematic -test.installer-external-repository=127.0.0.1:5100/test -test.installer-internal-repository=127.0.0.1:5100/test -test.cache-repository=127.0.0.1:5100/image-factory/cache -test.signing-cache-repository=127.0.0.1:5100/image-factory/signing-cache -test.extra-extensions-manifest=127.0.0.1:5100/extension-testing/extensions" REGISTRY=127.0.0.1:5005
```

A test focus can be set with:

```bash
...  RUN_TESTS_DIRECT='TestIntegration/Schematic'
```

For Enterprise tests, use the following command:

```bash
make integration-enterprise TEST_FLAGS="-test.image-registry=127.0.0.1:5004 -test.schematic-service-repository=127.0.0.1:5100/image-factory/schematic -test.installer-external-repository=127.0.0.1:5100/test -test.installer-internal-repository=127.0.0.1:5100/test -test.cache-repository=127.0.0.1:5100/image-factory/cache -test.signing-cache-repository=127.0.0.1:5100/image-factory/signing-cache -test.extra-extensions-manifest=127.0.0.1:5100/extension-testing/extensions" REGISTRY=127.0.0.1:5005
```

(The only change is `s/integration-direct/integration-enterprise/` in the target name, and the test focus variable will be `RUN_TESTS_ENTERPRISE` instead of `RUN_TESTS_DIRECT`.)
