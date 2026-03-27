#!/usr/bin/env bash
# generate-cilium-cm.sh
#
# Renders the Cilium Helm chart into a ConfigMap that ClusterResourceSet can
# apply to workload clusters.  Run this once per Cilium version bump.
#
# Prerequisites: helm (https://helm.sh/docs/intro/install/)
#
# Usage:
#   ./examples/generate-cilium-cm.sh              # uses CILIUM_VERSION below
#   CILIUM_VERSION=1.17.0 ./examples/generate-cilium-cm.sh

set -euo pipefail

CILIUM_VERSION="${CILIUM_VERSION:-1.16.5}"

echo "==> Adding Cilium Helm repo"
helm repo add cilium https://helm.cilium.io/ --force-update
helm repo update cilium

echo "==> Rendering Cilium ${CILIUM_VERSION} manifests"
helm template cilium cilium/cilium \
  --version "${CILIUM_VERSION}" \
  --namespace kube-system \
  --set ipam.mode=kubernetes \
  --set kubeProxyReplacement=false \
  | kubectl create configmap cilium-cni \
      --namespace default \
      --from-file=cilium.yaml=/dev/stdin \
      --dry-run=client -o yaml \
  | kubectl apply -f -

echo "==> Done. ConfigMap 'cilium-cni' is ready in the management cluster."
echo "    Apply the ClusterResourceSet: kubectl apply -f examples/cni-cilium.yaml"
