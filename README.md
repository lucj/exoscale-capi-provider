# Cluster API Provider Exoscale

Kubernetes Cluster API **infrastructure provider** for [Exoscale](https://www.exoscale.com/).

## Overview

This is an **infrastructure provider** for [Cluster API](https://cluster-api.sigs.k8s.io/) that manages Exoscale cloud resources. It handles provisioning and lifecycle management of compute instances, networking, and security groups on Exoscale.

### What This Provider Does

This infrastructure provider:
- Provisions and manages Exoscale compute instances
- Creates and configures security groups
- Allocates Elastic IPs for control plane endpoints
- Provides the `ExoscaleCluster` and `ExoscaleMachine` custom resources

### What This Provider Does NOT Include

This is **only** an infrastructure provider. To create a complete Kubernetes cluster, you also need:
- **Control Plane Provider** - manages Kubernetes control plane lifecycle (e.g., kubeadm control plane provider)
- **Bootstrap Provider** - generates cloud-init data for node initialization (e.g., kubeadm bootstrap provider)

The examples in this README use the standard kubeadm control plane and bootstrap providers alongside this Exoscale infrastructure provider.

## Prerequisites

- A Kubernetes cluster (v1.28+) to run the provider (management cluster)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl) installed
- An Exoscale account with API credentials
- [kustomize](https://kustomize.io/) (optional, for development)

## Cluster API Provider Architecture

Cluster API uses a modular architecture with separate providers for different responsibilities:

```
┌─────────────────────────────────────────────────────────┐
│                    Cluster API Core                      │
│                  (Cluster, Machine, etc)                 │
└─────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
           ▼               ▼               ▼
┌──────────────────┐ ┌──────────────┐ ┌──────────────────┐
│   Infrastructure │ │  Control     │ │    Bootstrap     │
│     Provider     │ │    Plane     │ │     Provider     │
│                  │ │   Provider   │ │                  │
│  THIS PROVIDER   │ │              │ │                  │
│  (Exoscale)      │ │  (Kubeadm)   │ │   (Kubeadm)      │
│                  │ │              │ │                  │
│ - Instances      │ │ - etcd       │ │ - cloud-init     │
│ - Security Groups│ │ - API server │ │ - node join      │
│ - Elastic IPs    │ │ - Controllers│ │                  │
└──────────────────┘ └──────────────┘ └──────────────────┘
```

## Supported Exoscale Zones

The provider supports the following Exoscale zones:
- `ch-gva-2` (Geneva, Switzerland)
- `ch-dk-2` (Zurich, Switzerland)
- `de-fra-1` (Frankfurt, Germany)
- `de-muc-1` (Munich, Germany)
- `at-vie-1` (Vienna, Austria)
- `at-vie-2` (Vienna, Austria)
- `bg-sof-1` (Sofia, Bulgaria)

## Installation

### 1. Initialize Cluster API with required providers

Since this is only an infrastructure provider, you need to install it alongside control plane and bootstrap providers. The most common setup uses the kubeadm providers:

```bash
# Initialize Cluster API with core components and kubeadm providers
clusterctl init --bootstrap kubeadm --control-plane kubeadm
```

This installs:
- Cluster API core components
- Kubeadm bootstrap provider (generates cloud-init for nodes)
- Kubeadm control plane provider (manages control plane lifecycle)

### 2. Build and deploy the Exoscale infrastructure provider

```bash
# Build the manager binary
make build

# Build and push the Docker image (update IMG variable as needed)
export IMG=your-registry/cluster-api-provider-exoscale:latest
make docker-build docker-push

# Deploy to your management cluster
make deploy
```

### 3. Configure Exoscale credentials

Create a secret with your Exoscale API credentials:

```bash
kubectl create secret generic exoscale-credentials \
  --from-literal=EXOSCALE_API_KEY=your-api-key \
  --from-literal=EXOSCALE_API_SECRET=your-api-secret \
  -n cluster-api-provider-exoscale-system
```

The provider will automatically use these credentials from the environment.

## Usage

### Understanding the Resource Types

When creating a cluster, you'll use resources from multiple providers:

| Resource | Provider | Purpose |
|----------|----------|---------|
| `Cluster` | Cluster API Core | Top-level cluster definition |
| `ExoscaleCluster` | **This provider** | Exoscale infrastructure (security groups, EIP) |
| `ExoscaleMachine` | **This provider** | Exoscale compute instances |
| `ExoscaleMachineTemplate` | **This provider** | Template for Exoscale machines |
| `KubeadmControlPlane` | Kubeadm Control Plane | Control plane management |
| `KubeadmConfigTemplate` | Kubeadm Bootstrap | Node bootstrap configuration |
| `MachineDeployment` | Cluster API Core | Worker node deployment |

### Creating a Workload Cluster

Create a workload cluster configuration. Here's a complete example showing all three providers working together:

```yaml
# Cluster resource (Cluster API Core)
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 10.244.0.0/16
    services:
      cidrBlocks:
      - 10.96.0.0/12
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ExoscaleCluster  # References Exoscale infrastructure provider
    name: my-cluster
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane  # References kubeadm control plane provider
    name: my-cluster-control-plane
---
# ExoscaleCluster resource (Exoscale Infrastructure Provider)
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ExoscaleCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  zone: ch-gva-2
  masterSecurityGroup: my-cluster-master  # optional, defaults to <cluster-name>-master
  nodeSecurityGroup: my-cluster-node      # optional, defaults to <cluster-name>-node
---
# KubeadmControlPlane resource (Kubeadm Control Plane Provider)
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlane
metadata:
  name: my-cluster-control-plane
  namespace: default
spec:
  version: v1.28.0
  replicas: 3
  machineTemplate:
    infrastructureRef:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: ExoscaleMachineTemplate  # References Exoscale infrastructure provider
      name: my-cluster-control-plane
  kubeadmConfigSpec:
    initConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          cloud-provider: external
    joinConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          cloud-provider: external
---
# ExoscaleMachineTemplate resource (Exoscale Infrastructure Provider)
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ExoscaleMachineTemplate
metadata:
  name: my-cluster-control-plane
  namespace: default
spec:
  template:
    spec:
      zone: ch-gva-2
      instanceType: standard.medium
      template: "Linux Ubuntu 22.04 LTS 64-bit"
      diskSize: 50
      sshKey: my-ssh-key
      ipv6: false
---
# MachineDeployment resource (Cluster API Core)
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: my-cluster-workers
  namespace: default
spec:
  clusterName: my-cluster
  replicas: 3
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
          kind: KubeadmConfigTemplate  # References kubeadm bootstrap provider
          name: my-cluster-workers
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: ExoscaleMachineTemplate  # References Exoscale infrastructure provider
        name: my-cluster-workers
---
# ExoscaleMachineTemplate resource (Exoscale Infrastructure Provider)
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
      ipv6: false
---
# KubeadmConfigTemplate resource (Kubeadm Bootstrap Provider)
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
          kubeletExtraArgs:
            cloud-provider: external
```

Apply the configuration:

```bash
kubectl apply -f cluster.yaml
```

### Monitoring Cluster Creation

Watch the cluster creation progress:

```bash
# Watch cluster status
kubectl get clusters -w

# Watch machines
kubectl get machines -w

# View detailed status
kubectl describe cluster my-cluster
kubectl describe exoscalecluster my-cluster
```

### Accessing the Workload Cluster

Once the cluster is ready, retrieve the kubeconfig:

```bash
clusterctl get kubeconfig my-cluster > my-cluster.kubeconfig
kubectl --kubeconfig=my-cluster.kubeconfig get nodes
```

### Deleting a Cluster

```bash
kubectl delete cluster my-cluster
```

The provider will automatically clean up all Exoscale resources (instances, security groups, elastic IP).

## Configuration Reference

### ExoscaleCluster

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.zone` | string | Yes | Exoscale zone (e.g., `ch-gva-2`) |
| `spec.masterSecurityGroup` | string | No | Name for control plane security group (default: `<cluster-name>-master`) |
| `spec.nodeSecurityGroup` | string | No | Name for worker node security group (default: `<cluster-name>-node`) |

### ExoscaleMachine

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.zone` | string | Yes | Exoscale zone (e.g., `ch-gva-2`) |
| `spec.instanceType` | string | Yes | Instance type (e.g., `standard.medium`, `cpu.large`) |
| `spec.template` | string | Yes | OS template name or UUID |
| `spec.diskSize` | int64 | Yes | Root disk size in GB |
| `spec.sshKey` | string | Yes | SSH key name (must exist in Exoscale) |
| `spec.antiAffinityGroup` | string | No | Anti-affinity group name or UUID |
| `spec.ipv6` | bool | No | Enable IPv6 (default: false) |

### Instance Types

Common instance types:
- `standard.tiny` - 1 vCPU, 1 GB RAM
- `standard.small` - 2 vCPU, 2 GB RAM
- `standard.medium` - 2 vCPU, 4 GB RAM
- `standard.large` - 4 vCPU, 8 GB RAM
- `standard.extra-large` - 8 vCPU, 16 GB RAM
- `cpu.large` - 4 vCPU, 4 GB RAM
- `cpu.huge` - 12 vCPU, 12 GB RAM
- `memory.large` - 4 vCPU, 32 GB RAM

See [Exoscale documentation](https://www.exoscale.com/pricing/) for the full list.

## Security Groups

The provider automatically creates and manages two security groups per cluster:

**Master Security Group** (Control Plane)
- SSH (port 22) from 0.0.0.0/0
- Kubernetes API (port 6443) from 0.0.0.0/0

**Node Security Group** (Workers)
- SSH (port 22) from 0.0.0.0/0

You can add additional rules manually or customize the security groups as needed.

## Development

### Prerequisites for Development

- Go 1.23+
- Docker
- kubectl
- kustomize

### Building Locally

```bash
# Build the manager binary
make build

# Run tests
make test

# Run the controller locally (requires kubeconfig)
make run
```

### Project Structure

```
.
├── api/v1beta1/              # API types (CRDs)
├── cmd/manager/              # Manager entry point
├── internal/
│   ├── cloud/                # Exoscale API client wrapper
│   └── controller/           # Reconciliation logic
├── config/                   # Kustomize manifests
├── Dockerfile                # Container image
└── Makefile                  # Build targets
```

### Architecture

This **infrastructure provider** consists of two main controllers that manage Exoscale cloud resources:

**ExoscaleClusterReconciler**
- Creates/manages security groups with baseline rules
- Provisions an Elastic IP for the control plane endpoint
- Sets the API endpoint in the Cluster status
- Reconciles `ExoscaleCluster` resources

**ExoscaleMachineReconciler**
- Provisions compute instances with the specified configuration
- Assigns instances to the appropriate security group (master/node)
- Updates machine status with IP addresses and ready state
- Handles instance deletion during cluster teardown
- Reconciles `ExoscaleMachine` resources

Both controllers use finalizers to ensure proper cleanup of Exoscale resources when objects are deleted.

**Important**: This provider does NOT manage:
- Kubernetes control plane components (handled by control plane provider)
- Node bootstrapping/cloud-init (handled by bootstrap provider)
- Cluster API core resources like `Cluster` and `Machine`

The provider integrates with Cluster API core and other providers through standard interfaces defined by the Cluster API contract.

## Troubleshooting

### Checking Provider Logs

```bash
kubectl logs -n cluster-api-provider-exoscale-system \
  deployment/cluster-api-provider-exoscale-controller-manager \
  -f
```

### Common Issues

**Instance creation fails with "template not found"**
- Verify the template name matches exactly (case-sensitive)
- List available templates in Exoscale console

**Instance creation fails with "SSH key not found"**
- Ensure the SSH key exists in your Exoscale account for the specified zone
- SSH keys are zone-specific

**Credentials error**
- Verify the `exoscale-credentials` secret exists and contains valid credentials
- Ensure the secret is in the correct namespace

**Security group errors during deletion**
- Security groups with attached instances cannot be deleted
- The controller will retry deletion once instances are removed

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

This project is licensed under the Apache License 2.0.
