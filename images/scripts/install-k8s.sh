#!/usr/bin/env bash
# install-k8s.sh — Provisions a Kubernetes node image for use with Cluster API.
#
# Installs: containerd (systemd cgroup driver), kubeadm, kubelet, kubectl.
# Pre-pulls control-plane images so kubeadm init completes in ~60 s instead of
# downloading them on first boot.
#
# Expected environment variables:
#   KUBERNETES_VERSION  — minor version, e.g. "1.35"

set -euo pipefail

echo "==> Kubernetes ${KUBERNETES_VERSION} node image build starting"

# ---------------------------------------------------------------------------
# 1. Disable swap (Kubernetes requirement)
# ---------------------------------------------------------------------------
swapoff -a
sed -i '/\bswap\b/d' /etc/fstab

# ---------------------------------------------------------------------------
# 2. Kernel modules required for Kubernetes networking
# ---------------------------------------------------------------------------
cat > /etc/modules-load.d/k8s.conf << 'EOF'
overlay
br_netfilter
EOF

modprobe overlay
modprobe br_netfilter

# ---------------------------------------------------------------------------
# 3. Sysctl — enable IP forwarding and bridge netfilter
# ---------------------------------------------------------------------------
cat > /etc/sysctl.d/k8s.conf << 'EOF'
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sysctl --system

# ---------------------------------------------------------------------------
# 4. containerd
# ---------------------------------------------------------------------------
echo "==> Installing containerd"
apt-get update -qq
apt-get install -yq containerd apt-transport-https ca-certificates curl gpg

# Configure containerd to use the systemd cgroup driver.
# Without this kubelet and containerd use different cgroup drivers and the
# node fails health checks.
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

systemctl restart containerd
systemctl enable containerd

# ---------------------------------------------------------------------------
# 5. Kubernetes packages (kubeadm, kubelet, kubectl)
# ---------------------------------------------------------------------------
echo "==> Installing Kubernetes ${KUBERNETES_VERSION} packages"
mkdir -p /etc/apt/keyrings
curl -fsSL "https://pkgs.k8s.io/core:/stable:/v${KUBERNETES_VERSION}/deb/Release.key" \
  | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] \
https://pkgs.k8s.io/core:/stable:/v${KUBERNETES_VERSION}/deb/ /" \
  > /etc/apt/sources.list.d/kubernetes.list

apt-get update -qq
apt-get install -yq kubelet kubeadm kubectl
apt-mark hold kubelet kubeadm kubectl
systemctl enable kubelet

# ---------------------------------------------------------------------------
# 6. Pre-pull control-plane images
# ---------------------------------------------------------------------------
# This is optional but strongly recommended: nodes will boot and run kubeadm
# init / join in ~60 s instead of waiting for image pulls.
echo "==> Pre-pulling control-plane images"
kubeadm config images pull

# ---------------------------------------------------------------------------
# 7. Clean up
# ---------------------------------------------------------------------------
apt-get clean
rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Reset machine-id so every clone gets a unique one on first boot.
truncate -s 0 /etc/machine-id

echo "==> Done — Kubernetes ${KUBERNETES_VERSION} node image is ready"
