# Image Factory Helm Chart

If you want to deploy Image Factory on a Kubernetes cluster, you can use the provided Helm chart located in the `deploy/helm/image-factory` directory.
It is also distributed via the OCI registry.

## E2E Tests

Image Factory uses KUTTL (Kubernetes Test Tool) integration tests for the Image Factory Helm chart.

### Prerequisites

1. **Install KUTTL**:

   ```bash
   make kuttl-plugin-install
   ```

   This will install the `kubectl kuttl` plugin using krew.

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
kubectl kuttl test
```

### Run with custom timeout

```bash
kubectl kuttl test --timeout 600
```

### Configuration

#### Test Suite Settings (kuttl-test.yaml)

- **startKIND**: `false` - Does not start KIND automatically (we do it manually to test on Talos cluster)
- **namespace**: `image-factory-e2e` - Fixed namespace for tests
- **crdDir**: `./_crds` - Prometheus Operator CRDs installed before tests
- **timeout**: `300` - Each step has 300 second timeout
- **parallel**: `1` - Tests run sequentially

### Debugging

```bash
export KUBECONFIG=./_out/kubeconfig
kubectl get all -n image-factory-e2e
kubectl logs -n image-factory-e2e deployment/image-factory
```

## Notes

- Prometheus Operator CRDs are pre-installed from `_crds/`
- Each test step has a 300-second timeout
- Tests run in fixed namespace `image-factory-e2e`
- Tests use `--reuse-values` for upgrades to maintain state
- ECPARAM signing key is provided via a pre-generated key file in `testdata/`
