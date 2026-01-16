# KUTTL End-to-End Tests

This directory contains KUTTL (Kubernetes Test Tool) integration tests for the Image Factory Helm chart.

## Prerequisites

1. **Install KUTTL**:
   ```bash
   kubectl krew install kuttl
   ```

2. **Helm**: Helm v3+ installed

## Running Tests

KUTTL is configured to automatically create and tear down a KIND cluster for testing.

### Run all tests:
```bash
cd deploy/helm/e2e
kubectl kuttl test
```

### Run with custom timeout:
```bash
kubectl kuttl test --timeout 600
```

### Skip cluster cleanup (for debugging):
```bash
kubectl kuttl test --skip-cluster-delete
```

## Test Structure

```
e2e/
├── kuttl-test.yaml              # KUTTL test suite configuration
├── _crds/                       # Custom Resource Definitions
│   └── prom-operator-crds.yaml  # Prometheus Operator CRDs
├── _manifests/                  # Pre-test manifests (currently empty)
└── tests/
    └── 01-image-factory/        # Main test suite
        ├── 01-install.yaml      # Install chart with signing key
        ├── 02-assert.yaml       # Verify initial deployment
        ├── 11-upgrade.yaml      # Enable UI and PXE ingress
        ├── 12-assert.yaml       # Verify ingress resources
        ├── 21-upgrade.yaml      # Enable metrics, disable ingress
        ├── 22-assert.yaml       # Verify metrics resources
        ├── 23-error.yaml        # Verify ingresses removed
        ├── 98-delete.yaml       # Uninstall chart
        └── 99-error.yaml        # Verify cleanup
```

## Test Scenarios

The test suite runs a single comprehensive test (`01-image-factory`) with multiple steps:

### Step 01: Initial Installation
- Generates an ECPARAM signing key for cache signatures
- Installs chart with container signature verification enabled
- Uses custom signing key via `--set-file`

### Step 02: Assert Initial State
- Deployment is ready with 1 replica
- Main service exists and is ClusterIP
- ConfigMap contains configuration
- Secret for cache signing key is created
- ServiceAccount is created

### Step 11: Enable Ingress
- Upgrades chart with `--reuse-values`
- Enables UI ingress
- Enables PXE ingress

### Step 12: Assert Ingress Configuration
- UI ingress exists with correct host
- PXE ingress exists with correct host
- Both ingresses have correct backend configuration

### Step 21: Enable Metrics, Disable Ingress
- Enables metrics service
- Enables ServiceMonitor for Prometheus
- Disables UI and PXE ingress

### Step 22: Assert Metrics Configuration
- Metrics service exists
- ServiceMonitor is created
- Monitoring configuration is correct

### Step 23: Verify Ingress Removal
- UI ingress should not exist
- PXE ingress should not exist

### Step 98: Uninstall
- Uninstalls the Helm release

### Step 99: Verify Cleanup
- All resources should be removed
- Deployment, services, configmap, secret, and serviceaccount gone

## Configuration

### Test Suite Settings (kuttl-test.yaml)

- **startKIND**: `true` - Automatically creates KIND cluster
- **namespace**: `image-factory-e2e` - Fixed namespace for tests
- **crdDir**: `./_crds` - Prometheus Operator CRDs installed before tests
- **timeout**: `300` - Each step has 300 second timeout
- **parallel**: `1` - Tests run sequentially

## CI/CD Integration

### GitHub Actions Example:
```yaml
name: E2E Tests
on: [push, pull_request]
jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install KUTTL
        run: |
          VERSION=0.24.0
          curl -LO https://github.com/kudobuilder/kuttl/releases/download/v${VERSION}/kubectl-kuttl_${VERSION}_linux_x86_64
          chmod +x kubectl-kuttl_${VERSION}_linux_x86_64
          sudo mv kubectl-kuttl_${VERSION}_linux_x86_64 /usr/local/bin/kubectl-kuttl
      
      - name: Install KIND
        uses: helm/kind-action@v1
        with:
          install_only: true
      
      - name: Run E2E tests
        run: |
          cd deploy/helm/e2e
          kubectl kuttl test
```

## Debugging

### View detailed output:
```bash
kubectl kuttl test --config kuttl-test.yaml
```

### Keep cluster after test:
```bash
kubectl kuttl test --skip-cluster-delete
```

Then inspect resources:
```bash
export KUBECONFIG=./kubeconfig
kubectl get all -n image-factory-e2e
kubectl logs -n image-factory-e2e deployment/image-factory
```

### Manual cleanup:
```bash
kind delete cluster --name kind
```

## Notes

- Tests automatically create a fresh KIND cluster
- Prometheus Operator CRDs are pre-installed from `_crds/`
- Each test step has a 300-second timeout
- Tests run in fixed namespace `image-factory-e2e`
- Ingress tests require Kubernetes 1.19+ (KIND uses recent version)
- Tests use `--reuse-values` for upgrades to maintain state
- ECPARAM signing key is generated at test runtime
