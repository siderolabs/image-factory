# Image Factory Helm Chart

If you want to deploy Image Factory on a Kubernetes cluster, you can use the provided Helm chart located in the `deploy/helm/image-factory` directory.
It is also distributed via the OCI registry.

## E2E Tests

Image Factory uses [Chainsaw](https://kyverno.github.io/chainsaw/) integration tests for the Image Factory Helm chart.

### Prerequisites

1. **Install Chainsaw**:

   ```bash
   make chainsaw-install
   ```

   This downloads the `chainsaw` CLI into `_out/`.

2. **Helm**: Helm v4+ installed

### Running Tests

Before running any tests, prepare your development cluster.
You can do this by running:

```bash
make k8s-up
```

To tear down the cluster after testing, run:

```bash
make k8s-down
```

#### Run all tests

```bash
export KUBECONFIG="$(pwd)/_out/kubeconfig"
cd deploy/helm/e2e
$(pwd)/../../../_out/chainsaw test
```

Or via the Makefile target:

```bash
make chart-e2e-chainsaw
```

#### Run a single test

```bash
cd deploy/helm/e2e
$(pwd)/../../../_out/chainsaw test tests/01-image-factory
```

#### Run with custom timeouts

```bash
chainsaw test --exec-timeout 600s --assert-timeout 600s
```

### Configuration

#### Test Suite Settings (`deploy/helm/e2e/.chainsaw.yaml`)

- **timeouts.apply**: `60s` - per-apply operation timeout
- **timeouts.assert**: `600s` - per-assert operation timeout
- **timeouts.cleanup**: `300s` - cleanup timeout
- **timeouts.exec**: `600s` - script/command timeout (covers `helm install --wait`)
- **execution.parallel**: `1` - tests run sequentially
- **execution.failFast**: `true` - stop on first failure
- **templating.enabled**: `true` - enables `($namespace)` jmespath expressions in resources

Each test runs in its own ephemeral namespace (`chainsaw-<random>`) which Chainsaw creates and deletes automatically.

### Layout

```text
deploy/helm/e2e/
├── .chainsaw.yaml          # root configuration
├── _crds/                  # cluster-scoped CRDs applied as test setup
├── _manifests/             # cluster-scoped manifests (local-path-storage, ...)
├── _lib/                   # shared StepTemplates referenced via `use:` in tests
├── testdata/               # signing keys, htpasswd, cosign keys
└── tests/
    ├── 01-image-factory/   # basic install/upgrade/uninstall
    ├── 02-upstream/        # internal registry, schematic from upstream
    ├── 03-airgapped/       # mirror + cosign Talos images, airgapped install
    └── 04-enterprise/      # enterprise build (auth, grypeDB PVC)
```

### Debugging

```bash
export KUBECONFIG=./_out/kubeconfig
kubectl get ns | grep chainsaw-       # find ephemeral namespace
kubectl get all -n chainsaw-<suffix>
kubectl logs -n chainsaw-<suffix> deployment/image-factory
```

Failing steps automatically dump describes and pod logs via Chainsaw `catch:` blocks defined in `_lib/*.step.yaml` and per-test `try.catch` sections.

To keep resources around for inspection after a failure, pass `--skip-delete`:

```bash
chainsaw test --skip-delete --pause-on-failure
```

## Notes

- Prometheus Operator CRDs are pre-applied from `_crds/` by the `01-image-factory` test (required for ServiceMonitor assertions).
- `local-path-storage` is pre-applied from `_manifests/` by the `04-enterprise` test (required for the grypeDB PVC).
- Tests use ephemeral namespaces; service references in tests resolve the namespace via the `($namespace)` jmespath binding.
- Helm upgrades use `--reuse-values` to maintain state across steps.
- The ECDSA signing key is provided via a pre-generated key file in `testdata/`.
- Cosign keys for the airgapped test are in `testdata/cosign.key` and `testdata/cosign.pub`.
