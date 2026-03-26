# Cluster API Provider Exoscale

Kubernetes Cluster API **infrastructure provider** for [Exoscale](https://www.exoscale.com/).

## How it works

Cluster API (CAPI) runs inside a **management cluster** — any Kubernetes cluster you already have access to (k3s, GKE, EKS, kind, etc.). Three providers collaborate inside it to create workload clusters:

```
┌─────────────────────────────────────────────────────────┐
│                    Cluster API Core                      │
└─────────────────────────────────────────────────────────┘
           │                   │                   │
           ▼                   ▼                   ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│  Infrastructure  │ │  Control Plane   │ │    Bootstrap     │
│    Provider      │ │    Provider      │ │    Provider      │
│  (this repo)     │ │   (kubeadm)      │ │   (kubeadm)      │
│                  │ │                  │ │                  │
│ - Instances      │ │ - etcd           │ │ - cloud-init     │
│ - Security Groups│ │ - API server     │ │ - node join      │
│ - Elastic IPs    │ │ - Controllers    │ │                  │
└──────────────────┘ └──────────────────┘ └──────────────────┘
```

Because this provider is not published to the clusterctl registry, the kubeadm providers are installed via `clusterctl init` while this one is deployed separately using the manifests in `config/`.

## Supported zones

`ch-gva-2` · `ch-dk-2` · `de-fra-1` · `de-muc-1` · `at-vie-1` · `at-vie-2` · `bg-sof-1`

---

## Getting started

### Prerequisites

- A running Kubernetes cluster to use as the management cluster, with `kubectl` pointing to it
- [clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl)
- An Exoscale account with an API key/secret and an SSH key uploaded in the target zone

### 1 — Install CAPI core + kubeadm providers

```bash
clusterctl init --bootstrap kubeadm --control-plane kubeadm
kubectl get pods -A | grep capi  # wait until all Running
```

### 2 — Deploy the Exoscale provider

The image is published to GHCR automatically on every push to `main`. Open `config/default/kustomization.yaml` and replace the `newName` / `newTag` fields with your image reference, then apply:

```yaml
# config/default/kustomization.yaml  (edit before applying)
images:
  - name: controller
    newName: ghcr.io/<owner>/cluster-api-provider-exoscale
    newTag: latest
```

```bash
kubectl apply -k config/default
kubectl get pods -n cluster-api-provider-exoscale-system  # wait until Running
```

### 3 — Add Exoscale credentials

```bash
kubectl create secret generic exoscale-credentials \
  --from-literal=EXOSCALE_API_KEY=$EXOSCALE_API_KEY$ \
  --from-literal=EXOSCALE_API_SECRET=$EXOSCALE_API_SECRET$ \
  -n cluster-api-provider-exoscale-system
```

### 4 — Create a workload cluster

Save the manifest below as `my-cluster.yaml`, replacing `ch-gva-2` and `my-ssh-key` with your values, then apply it.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:      { cidrBlocks: ["10.244.0.0/16"] }
    services:  { cidrBlocks: ["10.96.0.0/12"] }
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ExoscaleCluster
    name: my-cluster
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    name: my-cluster-control-plane
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ExoscaleCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  zone: ch-gva-2
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ExoscaleMachineTemplate
metadata:
  name: my-cluster-control-plane
  namespace: default
spec:
  template:
    spec:
      zone: ch-gva-2
      instanceType: standard.medium   # 2 vCPU / 4 GB — minimum for control plane
      template: "Linux Ubuntu 22.04 LTS 64-bit"
      diskSize: 50
      sshKey: my-ssh-key
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlane
metadata:
  name: my-cluster-control-plane
  namespace: default
spec:
  replicas: 1
  version: v1.28.0
  machineTemplate:
    infrastructureRef:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: ExoscaleMachineTemplate
      name: my-cluster-control-plane
  kubeadmConfigSpec:
    initConfiguration:
      nodeRegistration:
        kubeletExtraArgs: { cloud-provider: external }
    joinConfiguration:
      nodeRegistration:
        kubeletExtraArgs: { cloud-provider: external }
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ExoscaleMachineTemplate
metadata:
  name: my-cluster-workers
  namespace: default
spec:
  template:
    spec:
      zone: ch-gva-2
      instanceType: standard.medium
      template: "Linux Ubuntu 22.04 LTS 64-bit"
      diskSize: 50
      sshKey: my-ssh-key
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: my-cluster-workers
  namespace: default
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs: { cloud-provider: external }
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: my-cluster-workers
  namespace: default
spec:
  clusterName: my-cluster
  replicas: 2
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: my-cluster
  template:
    spec:
      clusterName: my-cluster
      version: v1.28.0
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: my-cluster-workers
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: ExoscaleMachineTemplate
        name: my-cluster-workers
```

```bash
kubectl apply -f my-cluster.yaml
```

### 5 — Watch provisioning (~5–10 min)

```bash
kubectl get cluster,exoscalecluster,machines -n default -w
```

Expected sequence: `ExoscaleCluster` becomes ready (security groups + EIP created) → control-plane VM boots and gets the EIP attached → kubeadm initialises → workers join → `Cluster` becomes `Ready=true`. Check provider logs if something seems stuck:

```bash
kubectl logs -n cluster-api-provider-exoscale-system \
  deployment/cluster-api-provider-exoscale-controller-manager --container manager -f
```

### 6 — Access the cluster

```bash
clusterctl get kubeconfig my-cluster > my-cluster.kubeconfig
kubectl --kubeconfig=my-cluster.kubeconfig get nodes
```

If nodes stay `NotReady`, install a CNI plugin. Example with Flannel:

```bash
kubectl --kubeconfig=my-cluster.kubeconfig apply \
  -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
```

> Flannel uses VXLAN (UDP 8472). If overlay traffic is blocked, add that port to the node security group in the Exoscale portal.

### 7 — Cleanup

```bash
kubectl delete cluster my-cluster   # removes all Exoscale resources (VMs, SGs, EIP)
```

---

## Configuration reference

### ExoscaleCluster

| Field | Required | Description |
|-------|----------|-------------|
| `spec.zone` | Yes | Exoscale zone (e.g. `ch-gva-2`) |
| `spec.masterSecurityGroup` | No | Control plane security group name (default: `<cluster-name>-master`) |
| `spec.nodeSecurityGroup` | No | Worker security group name (default: `<cluster-name>-node`) |

### ExoscaleMachine / ExoscaleMachineTemplate

| Field | Required | Description |
|-------|----------|-------------|
| `spec.zone` | Yes | Exoscale zone |
| `spec.instanceType` | Yes | Instance type, e.g. `standard.medium` |
| `spec.template` | Yes | OS template name or UUID |
| `spec.diskSize` | Yes | Root disk size in GB |
| `spec.sshKey` | Yes | SSH key name (must exist in Exoscale for that zone) |
| `spec.antiAffinityGroup` | No | Anti-affinity group name or UUID |
| `spec.ipv6` | No | Enable IPv6 (default: false) |

Common instance types: `standard.tiny` (1c/1GB) · `standard.small` (2c/2GB) · `standard.medium` (2c/4GB) · `standard.large` (4c/8GB) · `cpu.large` (4c/4GB) · `memory.large` (4c/32GB).

### Security groups

The provider creates two security groups per cluster with the following baseline ingress rules (all from `0.0.0.0/0`):

| Group | Ports |
|-------|-------|
| Master | 22 (SSH), 6443 (API server), 2379–2380 (etcd), 10250 (kubelet), 10257 (controller-manager), 10259 (scheduler) |
| Node | 22 (SSH), 10250 (kubelet), 30000–32767 (NodePort) |

For production clusters, restrict etcd and kubelet ports to internal CIDRs. CNI-specific ports (e.g. Flannel VXLAN UDP 8472) must be added manually.

---

## Development

### Prerequisites

- Go 1.23+
- [controller-gen](https://github.com/kubernetes-sigs/controller-tools) — regenerates CRDs and deepcopy methods after API type changes
- kubectl + kustomize

### Common commands

```bash
# Build and test
go build -o bin/manager ./cmd/manager/
go test ./...

# Regenerate CRDs and RBAC after changing types in api/
go run sigs.k8s.io/controller-tools/cmd/controller-gen \
  rbac:roleName=manager-role crd webhook paths="./..." \
  output:crd:artifacts:config=config/crd/bases

# Regenerate deepcopy methods after changing types in api/
go run sigs.k8s.io/controller-tools/cmd/controller-gen \
  object:headerFile="hack/boilerplate.go.txt" paths="./..."
```

The container image is built and pushed to GHCR automatically by the CI workflow on every push to `main`.

### Project structure

```
├── api/v1beta1/        # CRD types
├── cmd/manager/        # Entry point
├── internal/
│   ├── cloud/          # Exoscale API client wrapper
│   └── controller/     # Reconciliation logic
├── config/             # Kustomize manifests
└── .github/workflows/  # CI (image build + push)
```

---

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `ExoscaleCluster` stays `Ready=false` | `kubectl describe exoscalecluster my-cluster` → `Conditions`; usually a credentials issue |
| Machine stays `Ready=false` | `kubectl get machine -o jsonpath='{.items[*].spec.bootstrap.dataSecretName}'` — empty means the bootstrap provider is still generating the cloud-init script; wait a moment |
| "template not found" in logs | Template names are case-sensitive and zone-specific; check the Exoscale portal under Compute → Templates |
| "SSH key not found" in logs | SSH keys are zone-specific; ensure the key exists in the same zone as the machines |
| Nodes `NotReady` after joining | Install a CNI plugin (step 6) |
| Security group deletion fails | SGs with attached instances cannot be deleted; the controller retries once instances are removed |

---

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

Apache License 2.0.
