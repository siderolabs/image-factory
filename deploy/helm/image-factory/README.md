# image-factory

![Version: v1.0.2](https://img.shields.io/badge/Version-v1.0.2-informational?style=flat) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat) ![AppVersion: v1.0.2](https://img.shields.io/badge/AppVersion-v1.0.2-informational?style=flat)

A Helm chart to deploy Sidero Image Factory on a Kubernetes cluster

**Homepage:** <https://github.com/siderolabs/image-factory>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Sidero Labs |  | <https://www.siderolabs.com> |

## Source Code

* <https://github.com/siderolabs/image-factory>

## Prerequisites

- Kubernetes 1.23+
- Helm 3.0+
- For user namespace isolation (`hostUsers: false`): Kubernetes 1.25+ is required

## Configuration

### Cache Signing Key

The Image Factory requires a signing key to sign cached artifacts. You can either provide an existing secret or let the chart create one for you.

#### Using an Existing Secret

```yaml
cacheSigningKey:
  existingSecret: "my-existing-secret"
```

> **Note:** The existing secret MUST contain a data key named exactly `cache-signing.key`.

#### Generating a New Key

Generate a new ECDSA private key:

```bash
openssl ecparam -name prime256v1 -genkey -noout -out cache-signing.key
```

Then create the secret:

```bash
kubectl create secret generic image-factory-cache-signing-key \
  --from-file=cache-signing.key=./cache-signing.key
```

Or include the key directly in your values:

```yaml
cacheSigningKey:
  key: |
    -----BEGIN EC PRIVATE KEY-----
    MHcCAQEEIK...
    -----END EC PRIVATE KEY-----
```

### Registry Configuration

Configure the schematic storage registry where Image Factory will store and retrieve schematics:

```yaml
config:
  artifacts:
    schematic:
      registry: registry.example.com
      namespace: siderolabs/image-factory
      repository: schematics
      insecure: false
```

### OCI Cache Configuration

Enable OCI registry caching for built artifacts:

```yaml
config:
  cache:
    oci:
      enabled: true
      registry: ghcr.io
      namespace: siderolabs/image-factory
      repository: cache
      insecure: false
```

### S3 Cache Configuration

Enable S3 bucket caching for built artifacts:

```yaml
config:
  cache:
    s3:
      enabled: true
      bucket: image-factory
      region: us-east-1
      endpoint: ""
      insecure: false
```

> **Note:** You'll need to configure AWS credentials via environment variables or IAM roles.

## Ingress

The chart supports both standard Kubernetes Ingress and Gateway API (experimental).

### Standard Ingress

#### UI Ingress

```yaml
ingress:
  ui:
    enabled: true
    className: "nginx"
    host: factory.example.com
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt-prod
    tls:
      - secretName: image-factory-ui-tls
        hosts:
          - factory.example.com
```

#### PXE Ingress

```yaml
ingress:
  pxe:
    enabled: true
    className: "nginx"
    host: pxe-factory.example.com
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt-prod
    tls:
      - secretName: image-factory-pxe-tls
        hosts:
          - pxe-factory.example.com
```

### Gateway API (Experimental)

> **Warning:** Gateway API support is in EXPERIMENTAL status. Support depends on your Gateway controller implementation.

```yaml
gatewayApi:
  ui:
    enabled: true
    hostnames:
      - factory.example.com
    parentRefs:
      - name: my-gateway
        namespace: gateway-system
        sectionName: https-listener
```

## Security

### User Namespaces

The chart supports Kubernetes user namespace isolation for enhanced security. When enabled (`hostUsers: false`), the pod uses a separate user namespace instead of the host's user namespace.

```yaml
hostUsers: false
```

> **Note:** User namespace support requires Kubernetes 1.25 or higher. The feature became GA (enabled by default) in Kubernetes 1.33.

> **Warning:** When set to `false` on Kubernetes versions below 1.25, the Helm deployment will fail with a validation error.

#### Running on Talos Linux

If you're running Image Factory on Talos Linux and want to use user namespaces, you need to enable user namespace support in your Talos machine configuration:

```yaml
cluster:
  apiServer:
    extraArgs:
      feature-gates: UserNamespacesSupport=true
  kubelet:
    extraConfig:
      featureGates:
        UserNamespacesSupport: true
machine:
  sysctls:
    user.max_user_namespaces: "11255"
```

For more details, see the [Talos Linux User Namespace documentation](https://docs.siderolabs.com/kubernetes-guides/security/usernamespace).

### Security Context

The chart includes a secure default security context:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  privileged: false
  runAsUser: 1000
  runAsGroup: 1000
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault
```

## Secure Boot

The Image Factory can generate Unified Kernel Images (UKI) and sign them for Secure Boot.

### Using Local Files

```yaml
config:
  secureBoot:
    enabled: true
    file:
      signingCertPath: /path/to/cert.pem
      signingKeyPath: /path/to/key.pem
      pcrKeyPath: /path/to/pcr-key.pem
```

### Using AWS KMS

```yaml
config:
  secureBoot:
    enabled: true
    awsKMS:
      region: us-east-1
      keyID: "arn:aws:kms:us-east-1:123456789012:key/..."
      pcrKeyID: "arn:aws:kms:us-east-1:123456789012:key/..."
      certPath: /path/to/cert.pem
```

### Using Azure Key Vault

```yaml
config:
  secureBoot:
    enabled: true
    azureKeyVault:
      url: https://myvault.vault.azure.net
      keyName: signing-key
      certificateName: signing-cert
```

## Monitoring

### Prometheus ServiceMonitor

Enable Prometheus metrics collection:

```yaml
service:
  metrics:
    enabled: true
    serviceMonitor:
      enabled: true
      interval: 30s
      namespace: monitoring
      selector:
        prometheus: kube-prometheus
```

## Advanced Configuration

### External URLs

Configure external URLs for the HTTP and PXE frontends:

```yaml
config:
  http:
    externalURL: https://factory.example.com/
    externalPXEURL: https://pxe-factory.example.com/
    httpListenAddr: 0.0.0.0:8080
```

### Build Configuration

Configure build concurrency and minimum Talos version:

```yaml
config:
  build:
    maxConcurrency: 6
    minTalosVersion: 1.2.0
```

### Custom Volumes

Add custom volumes and volume mounts:

```yaml
extraVolumes:
  - name: custom-config
    configMap:
      name: my-config

extraVolumeMounts:
  - name: custom-config
    mountPath: /custom
    readOnly: true
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| args[0] | string | `"--config=/config.yaml"` | Path to the configuration file (mounted via ConfigMap). Do not remove this unless you are using args only. |
| args[1] | string | `"--log-level=info"` | Log level [debug info warn error dpanic panic fatal] (default info) |
| cacheSigningKey | object | `{"existingSecret":"","key":""}` | Image Cache Signing Key Configuration # This secret contains the ECDSA private key used to sign cached Talos image artifacts. # This ensures that nodes can verify the integrity of images served by the Image Factory. # If you are running a self-hosted Image Factory, this key is required. |
| cacheSigningKey.existingSecret | string | `""` | Name of an existing Secret containing the ECDSA private key. IMPORTANT: The existing secret MUST contain a data key named exactly "cache-signing.key". If your secret uses a different key, Image Factory will not find the file. Example creation: kubectl create secret generic image-factory-cache-signing-key --from-file=cache-signing.key=./signing-key.key |
| cacheSigningKey.key | string | `""` | If 'existingSecret' is empty, a new Secret will be created. The ECDSA private key content (multiline string). Generate using: openssl ecparam -name prime256v1 -genkey -noout -out cache-signing.key |
| config | object | `{"artifacts":{"schematic":{"insecure":false,"namespace":"siderolabs/image-factory","registry":"registry.example.com","repository":"schematics"}},"cache":{"signingKeyPath":"/etc/image-factory/keys/cache-signing.key"}}` | Sidero Image-Factory Configuration |
| env | list | `[]` | Environment variables to pass to Image Factory |
| envFrom | list | `[]` | envFrom to pass to Image Factory |
| extraObjects | list | `[]` |  |
| extraVolumeMounts | list | `[]` | List of additional mounts to add (normally used with extraVolumes) |
| extraVolumes | list | `[]` | List of extra volumes to add |
| fullnameOverride | string | `""` | String to fully override `"image-factory.fullname"` |
| gatewayApi | object | `{"pxe":{"annotations":{},"enabled":false,"hostnames":["pxe-factory.example.com"],"labels":{},"parentRefs":[]},"ui":{"annotations":{},"enabled":false,"hostnames":["factory.example.com"],"labels":{},"parentRefs":[]}}` | Gateway API Configuration. NOTE: Gateway API support is in EXPERIMENTAL status. Support depends on your Gateway controller implementation. |
| gatewayApi.pxe.annotations | object | `{}` | Additional Annotations |
| gatewayApi.pxe.hostnames | list | `["pxe-factory.example.com"]` | Image Factory PXE hostname |
| gatewayApi.pxe.labels | object | `{}` | Additional Labels |
| gatewayApi.pxe.parentRefs | list | `[]` | The Gateway(s) to attach this route to. You MUST define at least one parentRef for the route to be active. |
| gatewayApi.ui.annotations | object | `{}` | Additional Annotations |
| gatewayApi.ui.hostnames | list | `["factory.example.com"]` | Image Factory UI hostname |
| gatewayApi.ui.labels | object | `{}` | Additional Labels |
| gatewayApi.ui.parentRefs | list | `[]` | The Gateway(s) to attach this route to. You MUST define at least one parentRef for the route to be active. |
| hostUsers | bool | `true` | Controls whether the pod uses the host's user namespace. When true (default), the pod uses the host user namespace. When false, the pod uses a separate user namespace for enhanced security isolation. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy for Image Factory |
| image.repository | string | `"ghcr.io/siderolabs/image-factory"` | Repository to use for Image Factory |
| image.tag | string | `""` | Tag to use for Image Factory |
| imagePullSecrets | list | `[]` | Secrets with credentials to pull images from a private registry |
| ingress | object | `{"pxe":{"annotations":{},"className":"","enabled":false,"host":"pxe-factory.example.com","labels":{},"tls":[]},"ui":{"annotations":{},"className":"","enabled":false,"host":"factory.example.com","labels":{},"tls":[]}}` | Ingress Configuration This section configures standard Kubernetes Ingress resources. Use this if you are using an Ingress Controller like NGINX, Traefik, or HAProxy. |
| ingress.pxe.annotations | object | `{}` | Additional Annotations |
| ingress.pxe.className | string | `""` | Ingress Class Name |
| ingress.pxe.host | string | `"pxe-factory.example.com"` | Image Factory PXE hostname |
| ingress.pxe.labels | object | `{}` | Additional Labels |
| ingress.pxe.tls | list | `[]` | TLS configuration |
| ingress.ui.annotations | object | `{}` | Additional Annotations |
| ingress.ui.className | string | `""` | Ingress Class Name |
| ingress.ui.host | string | `"factory.example.com"` | Image Factory UI hostname |
| ingress.ui.labels | object | `{}` | Additional Labels |
| ingress.ui.tls | list | `[]` | TLS configuration |
| livenessProbe | object | `{}` | Probes Configuration |
| nameOverride | string | `"image-factory"` | Provide a name in place of `image-factory` |
| namespaceOverride | string | `""` | Provide a namespace in place of the release namespace. |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext | object | `{}` |  |
| readinessProbe | object | `{}` |  |
| replicaCount | int | `1` |  |
| resources | object | `{}` | Resources Configuration Set CPU and Memory resource requests and limits for the Image Factory pod. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"privileged":false,"runAsGroup":1000,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod Security Context Image Factory container-level security context |
| service | object | `{"main":{"annotations":{},"labels":{},"loadBalancerIP":"","port":8080,"type":"ClusterIP"},"metrics":{"enabled":false,"rules":{"annotations":{},"enabled":false,"labels":{},"namespace":"","selector":{},"spec":[]},"service":{"annotations":{},"clusterIP":"","labels":{},"servicePort":2122,"type":"ClusterIP"},"serviceMonitor":{"annotations":{},"enabled":false,"honorLabels":false,"interval":"30s","labels":{},"metricRelabelings":[],"namespace":"","relabelings":[],"scheme":"","scrapeTimeout":"","selector":{},"tlsConfig":{}}}}` | Service Configuration Configures the Kubernetes Services to expose Image Factory's network endpoints. - 'main': Exposes the UI and PXE services (ClusterIP by default). - 'metrics': Exposes Prometheus metrics endpoint (ClusterIP by default). |
| service.main | object | `{"annotations":{},"labels":{},"loadBalancerIP":"","port":8080,"type":"ClusterIP"}` | Main Service (Image-Factory) |
| service.main.annotations | object | `{}` | Additional Annotations |
| service.main.labels | object | `{}` | Additional Labels |
| service.main.loadBalancerIP | string | `""` | If type is Loadbalancer |
| service.main.port | int | `8080` | Web UI |
| service.main.type | string | `"ClusterIP"` | Default: ClusterIP |
| service.metrics | object | `{"enabled":false,"rules":{"annotations":{},"enabled":false,"labels":{},"namespace":"","selector":{},"spec":[]},"service":{"annotations":{},"clusterIP":"","labels":{},"servicePort":2122,"type":"ClusterIP"},"serviceMonitor":{"annotations":{},"enabled":false,"honorLabels":false,"interval":"30s","labels":{},"metricRelabelings":[],"namespace":"","relabelings":[],"scheme":"","scrapeTimeout":"","selector":{},"tlsConfig":{}}}` | Image Factory metrics configuration |
| service.metrics.enabled | bool | `false` | Deploy metrics service |
| service.metrics.rules.annotations | object | `{}` | PrometheusRule annotations |
| service.metrics.rules.enabled | bool | `false` | Deploy a PrometheusRule for Image Factory |
| service.metrics.rules.labels | object | `{}` | PrometheusRule labels |
| service.metrics.rules.namespace | string | `""` | PrometheusRule namespace |
| service.metrics.rules.selector | object | `{}` | PrometheusRule selector |
| service.metrics.rules.spec | list | `[]` | PrometheusRule.Spec for Image Factory |
| service.metrics.service.annotations | object | `{}` | Metrics service annotations |
| service.metrics.service.clusterIP | string | `""` | Metrics service clusterIP. `None` makes a "headless service" (no virtual IP) |
| service.metrics.service.labels | object | `{}` | Metrics service labels |
| service.metrics.service.servicePort | int | `2122` | Metrics service port |
| service.metrics.service.type | string | `"ClusterIP"` | Metrics service type |
| service.metrics.serviceMonitor.annotations | object | `{}` | Prometheus ServiceMonitor annotations |
| service.metrics.serviceMonitor.enabled | bool | `false` | Enable a prometheus ServiceMonitor |
| service.metrics.serviceMonitor.honorLabels | bool | `false` | When true, honorLabels preserves the metric’s labels when they collide with the target’s labels. |
| service.metrics.serviceMonitor.interval | string | `"30s"` | Prometheus ServiceMonitor interval |
| service.metrics.serviceMonitor.metricRelabelings | list | `[]` | Prometheus [MetricRelabelConfigs] to apply to samples before ingestion |
| service.metrics.serviceMonitor.namespace | string | `""` | Prometheus ServiceMonitor namespace |
| service.metrics.serviceMonitor.relabelings | list | `[]` | Prometheus [RelabelConfigs] to apply to samples before scraping |
| service.metrics.serviceMonitor.scheme | string | `""` | Prometheus ServiceMonitor selector |
| service.metrics.serviceMonitor.scrapeTimeout | string | `""` | Prometheus ServiceMonitor scrapeTimeout. If empty, Prometheus uses the global scrape timeout unless it is less than the target's scrape interval value in which the latter is used. |
| service.metrics.serviceMonitor.selector | object | `{}` | Prometheus ServiceMonitor labels |
| service.metrics.serviceMonitor.tlsConfig | object | `{}` | Prometheus ServiceMonitor tlsConfig |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.automount | bool | `false` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| strategy | object | `{"type":"Recreate"}` | Deployment strategy. Image Factory currently only supports Recreate strategy. |
| tolerations | list | `[]` |  |
