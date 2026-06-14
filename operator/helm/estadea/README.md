# estadea

Kubernetes operator that automatically force-deletes pods stuck in `Terminating`,
`Pending`, `Unknown` or `Failed` state for longer than a configurable grace period.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.10+

## Installation

```bash
helm repo add estadea https://heimops.github.io/estadea
helm repo update
helm install estadea estadea/estadea -n estadea-system --create-namespace
```

## Uninstallation

```bash
helm uninstall estadea -n estadea-system
# CRDs are kept by default (crds.keep=true). Remove manually if needed:
kubectl delete crd podreapers.reaper.estadea.io
```

## Usage

After installing the operator, create a `PodReaper` resource to define which pods
to clean up and how aggressively:

```yaml
apiVersion: reaper.estadea.io/v1alpha1
kind: PodReaper
metadata:
  name: cluster-reaper
spec:
  # Leave empty to watch all namespaces
  namespaces:
    - default
    - production
  podStates:
    - Terminating
    - Unknown
  graceTimeout: "5m"
  checkInterval: "30s"
```

```bash
kubectl apply -f podreaper.yaml
kubectl get podreaper          # or: kubectl get pr
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Operator image repository | `ghcr.io/heimops/estadea` |
| `image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `namespace.create` | Create the operator namespace | `true` |
| `namespace.name` | Namespace for the operator | `estadea-system` |
| `leaderElect` | Enable leader election (recommended in production) | `true` |
| `resources.limits.cpu` | CPU limit | `100m` |
| `resources.limits.memory` | Memory limit | `64Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `32Mi` |
| `metrics.enabled` | Expose Prometheus metrics on `:8080` | `true` |
| `metrics.serviceMonitor.enabled` | Create a Prometheus `ServiceMonitor` | `false` |
| `crds.install` | Install CRDs via Helm | `true` |
| `crds.keep` | Keep CRDs on `helm uninstall` | `true` |
| `defaultPodReaper.enabled` | Deploy a default `PodReaper` instance | `false` |

## PodReaper spec reference

| Field | Description | Default |
|-------|-------------|---------|
| `namespaces` | Namespaces to watch. Empty = all namespaces | `[]` |
| `podStates` | Pod states that trigger cleanup (`Terminating`, `Pending`, `Unknown`, `Failed`) | `["Terminating"]` |
| `graceTimeout` | Minimum time a pod must be stuck before deletion | `"5m"` |
| `checkInterval` | How often the operator scans for stuck pods | `"30s"` |

## License

Apache-2.0
