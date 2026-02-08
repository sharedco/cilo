# Cilo VM Image Builder

This directory contains Packer templates for building Cilo VM images.

## Prerequisites

1. Install Packer: https://developer.hashicorp.com/packer/downloads
2. Install Hetzner Cloud plugin:
   ```bash
   packer plugins install github.com/hetznercloud/hcloud
   ```
3. Get a Hetzner Cloud API token from https://console.hetzner.cloud/

## Building the Image

### Quick Start

```bash
export HCLOUD_TOKEN="your-api-token"
packer build -var "hcloud_token=$HCLOUD_TOKEN" cilo-vm.pkr.hcl
```

### With Custom Options

```bash
packer build \
  -var "hcloud_token=$HCLOUD_TOKEN" \
  -var "image_name=cilo-vm-custom" \
  -var "server_type=cx31" \
  -var "location=hel1" \
  cilo-vm.pkr.hcl
```

### Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `hcloud_token` | (required) | Hetzner Cloud API token |
| `image_name` | `cilo-vm` | Base name for the snapshot |
| `server_type` | `cx21` | Server type for building |
| `location` | `nbg1` | Datacenter location |
| `agent_version` | `latest` | cilo-agent version to install |

## What's Included

The image includes:

- **Base OS**: Ubuntu 24.04 LTS
- **Container Runtimes**:
  - Docker Engine with Compose plugin
  - Podman with podman-compose
- **Networking**:
  - WireGuard (kernel module + tools)
  - UFW firewall (pre-configured)
- **Cilo Agent**: Machine management daemon
- **Pre-pulled Images**:
  - postgres:16-alpine
  - redis:7-alpine
  - nginx:alpine
  - node:20-alpine
  - python:3.12-slim

## Image Lifecycle

1. **Build**: Run Packer to create a snapshot
2. **Note Image ID**: Packer outputs the snapshot ID
3. **Configure Server**: Add image ID to `CILO_POOL_IMAGE_ID`
4. **First Boot**: Machine generates unique WireGuard keys

## Firewall Rules

The image comes with UFW configured:

| Port | Protocol | Description |
|------|----------|-------------|
| 22 | TCP | SSH access |
| 51820 | UDP | WireGuard |
| wg0 | * | WireGuard interface traffic |

## Updating Images

1. Build a new image with Packer
2. Update `CILO_POOL_IMAGE_ID` in your server config
3. New machines will use the new image
4. Existing machines continue with old image until recycled

## Troubleshooting

### Build Fails

Check:
- Hetzner API token is valid
- You have snapshot quota available
- Network connectivity to Hetzner

### Agent Won't Start

On the VM, check:
```bash
systemctl status cilo-agent
journalctl -u cilo-agent -f
```

### WireGuard Issues

Check:
```bash
wg show
journalctl -u cilo-first-boot
```
