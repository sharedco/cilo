# Self-Hosting Cilo Server

This guide helps you deploy Cilo Server on your own infrastructure.

## Prerequisites

- Docker and Docker Compose
- A domain name pointing to your server
- (Optional) Hetzner Cloud account for auto-provisioning VMs

## Quick Start

### 1. Clone and Configure

```bash
cd deploy/self-host
cp .env.example .env
```

Edit `.env` with your configuration:
- Set `CILO_DOMAIN` to your domain
- Set a strong `POSTGRES_PASSWORD`
- Choose your provider (`manual` or `hetzner`)

### 2. Start the Server

```bash
docker compose up -d
```

### 3. Create Your First API Key

```bash
docker compose exec server cilo-server admin create-key \
  --scope admin \
  --name "admin-key"
```

Save the generated API key securely.

### 4. (Manual Provider) Register Machines

If using the manual provider, register your existing machines:

```bash
docker compose exec server cilo-server machines add \
  --name build-1 \
  --host 192.168.1.100 \
  --ssh-user deploy
```

### 5. (Hetzner Provider) Build VM Image

If using Hetzner, build the VM image first:

```bash
cd ../../packer
packer build cilo-vm.pkr.hcl
```

Note the image ID and add it to your `.env`:
```bash
CILO_POOL_IMAGE_ID=123456789
```

Then restart:
```bash
docker compose up -d
```

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `CILO_DOMAIN` | (required) | Your server's domain name |
| `POSTGRES_PASSWORD` | (required) | Database password |
| `CILO_PROVIDER` | `manual` | `manual` or `hetzner` |
| `HETZNER_API_TOKEN` | - | Hetzner Cloud API token |
| `CILO_POOL_MIN_READY` | `3` | Minimum ready VMs in pool |
| `CILO_POOL_MAX_TOTAL` | `20` | Maximum total VMs |
| `CILO_POOL_VM_SIZE` | `cx31` | Hetzner server type |
| `CILO_POOL_REGION` | `nbg1` | Hetzner datacenter |
| `CILO_POOL_IMAGE_ID` | - | Custom VM image ID |
| `CILO_AUTO_DESTROY_HOURS` | `8` | Auto-cleanup idle envs |

## Connecting from CLI

On each developer's machine:

```bash
cilo cloud login --server https://your-cilo-domain.com
# Enter your API key when prompted
```

Then use:
```bash
cilo cloud up my-env --from .
```

## Monitoring

Prometheus metrics are available at `/metrics` when `CILO_METRICS_ENABLED=true`.

Key metrics:
- `cilo_environments_total` - Total environments by status
- `cilo_machines_total` - Total machines by status
- `cilo_api_requests_total` - API request count
- `cilo_api_request_duration_seconds` - API latency

## Backup

Back up the PostgreSQL database regularly:

```bash
docker compose exec postgres pg_dump -U cilo cilo > backup.sql
```

## Upgrading

```bash
docker compose pull
docker compose up -d
```

Migrations run automatically on startup.

## Troubleshooting

### Check logs
```bash
docker compose logs -f server
```

### Check database
```bash
docker compose exec postgres psql -U cilo -d cilo
```

### Health check
```bash
curl http://localhost:8080/health
```

## Security Considerations

1. **HTTPS**: The Caddy reverse proxy handles TLS automatically
2. **API Keys**: Store keys securely, use scoped keys for CI
3. **Network**: Consider running behind a firewall
4. **Updates**: Keep the server image updated
