packer {
  required_plugins {
    hcloud = {
      source  = "github.com/hetznercloud/hcloud"
      version = ">= 1.2.0"
    }
  }
}

variable "hcloud_token" {
  type      = string
  sensitive = true
}

variable "image_name" {
  type    = string
  default = "cilo-vm"
}

variable "server_type" {
  type    = string
  default = "cx21"
}

variable "location" {
  type    = string
  default = "nbg1"
}

variable "agent_version" {
  type    = string
  default = "latest"
}

source "hcloud" "ubuntu" {
  token        = var.hcloud_token
  image        = "ubuntu-24.04"
  location     = var.location
  server_type  = var.server_type
  server_name  = "cilo-packer-${formatdate("YYYYMMDDhhmmss", timestamp())}"
  snapshot_name = "${var.image_name}-${formatdate("YYYY-MM-DD-hhmm", timestamp())}"
  snapshot_labels = {
    "managed-by" = "packer"
    "purpose"    = "cilo-vm"
    "built"      = formatdate("YYYY-MM-DD", timestamp())
  }
  ssh_username = "root"
}

build {
  sources = ["source.hcloud.ubuntu"]

  # Wait for cloud-init
  provisioner "shell" {
    inline = [
      "cloud-init status --wait || true",
      "sleep 10"
    ]
  }

  # Copy installation script
  provisioner "file" {
    source      = "scripts/install.sh"
    destination = "/tmp/install.sh"
  }

  # Copy agent service file
  provisioner "file" {
    source      = "scripts/cilo-agent.service"
    destination = "/tmp/cilo-agent.service"
  }

  # Run installation
  provisioner "shell" {
    inline = [
      "chmod +x /tmp/install.sh",
      "AGENT_VERSION=${var.agent_version} /tmp/install.sh"
    ]
  }

  # Clean up
  provisioner "shell" {
    inline = [
      "rm -rf /tmp/*.sh /tmp/*.service",
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/*",
      "sync"
    ]
  }
}
