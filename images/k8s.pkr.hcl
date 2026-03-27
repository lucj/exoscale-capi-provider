packer {
  required_plugins {
    exoscale = {
      source  = "github.com/exoscale/exoscale"
      version = ">= 0.5.0"
    }
  }
}

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------

variable "kubernetes_version" {
  description = "Kubernetes minor version used for the apt repository (e.g. '1.35')."
  type        = string
  default     = "1.35"
}

variable "zone" {
  description = "Exoscale zone where the build instance is created and the template is registered."
  type        = string
  default     = "ch-gva-2"
}

variable "api_key" {
  type      = string
  default   = env("EXOSCALE_API_KEY")
  sensitive = true
}

variable "api_secret" {
  type      = string
  default   = env("EXOSCALE_API_SECRET")
  sensitive = true
}

# ---------------------------------------------------------------------------
# Locals
# ---------------------------------------------------------------------------

locals {
  template_name = "k8s-${var.kubernetes_version}-ubuntu-24.04"
}

# ---------------------------------------------------------------------------
# Source
# ---------------------------------------------------------------------------

source "exoscale" "k8s" {
  api_key    = var.api_key
  api_secret = var.api_secret

  # Build instance — small is enough, it only runs apt and kubeadm.
  instance_type     = "standard.small"
  instance_template = "Linux Ubuntu 24.04 LTS 64-bit"

  ssh_username = "ubuntu"

  # Resulting private template registered in the zone.
  template_name     = local.template_name
  template_zones    = [var.zone]
  template_username = "ubuntu"
  template_description = "Kubernetes ${var.kubernetes_version} on Ubuntu 24.04 LTS — built with Packer"
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

build {
  name    = "k8s-node"
  sources = ["source.exoscale.k8s"]

  provisioner "shell" {
    environment_vars = [
      "KUBERNETES_VERSION=${var.kubernetes_version}",
      "DEBIAN_FRONTEND=noninteractive",
    ]
    execute_command = "sudo env {{ .Vars }} bash '{{ .Path }}'"
    scripts         = ["${path.root}/scripts/install-k8s.sh"]
  }
}
