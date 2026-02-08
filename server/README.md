# Cilo Server

The Cilo Cloud API server provides centralized environment management, WireGuard networking, and VM orchestration for remote Cilo environments.

## Architecture

```
server/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── pkg/
│   ├── api/
│   │   ├── server.go            # HTTP server setup
│   │   ├── handlers.go          # API endpoint handlers
│   │   └── middleware.go        # Authentication middleware
│   ├── config/
│   │   └── config.go            # Configuration management
│   └── store/
│       ├── store.go             # Database connection
│       ├── models.go            # Data models
│       ├── migrations.go        # Migration runner
│       └── migrations/
│           ├── 000001_init.up.sql
│           └── 000001_init.down.sql
└── go.mod
```

## Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL 14+

### Build

```bash
cd server
go mod download
go build -o bin/cilo-server ./cmd/server
```

### Configuration

Set environment variables:

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/cilo?sslmode=disable"
export LISTEN_ADDR=":8080"
export HETZNER_TOKEN="your-token"
export POOL_MIN_READY=2
export POOL_MAX_TOTAL=10
```

### Run

```bash
./bin/cilo-server
```

## API Endpoints

### Health & Status

- `GET /health` - Health check
- `GET /status` - Server status
- `GET /metrics` - Prometheus metrics (if enabled)

### Authentication (v1)

- `POST /v1/auth/keys` - Create API key
- `GET /v1/auth/keys` - List API keys
- `DELETE /v1/auth/keys/{keyID}` - Revoke API key

### Environments (v1)

- `POST /v1/environments` - Create environment
- `GET /v1/environments` - List environments
- `GET /v1/environments/{envID}` - Get environment details
- `DELETE /v1/environments/{envID}` - Destroy environment
- `POST /v1/environments/{envID}/sync` - Sync environment state

### WireGuard (v1)

- `POST /v1/wireguard/exchange` - Exchange WireGuard keys
- `DELETE /v1/wireguard/peers/{peerID}` - Remove peer
- `GET /v1/wireguard/status` - WireGuard status

### Machines (v1)

- `POST /v1/machines` - Register machine
- `GET /v1/machines` - List machines
- `DELETE /v1/machines/{machineID}` - Remove machine

## Database Schema

### Tables

- **teams** - Customer organizations
- **api_keys** - Authentication tokens
- **machines** - VM hosts for environments
- **environments** - Cilo environments with services and peers
- **usage_records** - Billing/usage tracking

See `pkg/store/migrations/000001_init.up.sql` for full schema.

## Development Status

**Current:** Scaffold complete with placeholder handlers

**Next Steps:**
1. Implement authentication logic
2. Add VM provider implementations (Hetzner, DigitalOcean)
3. Implement WireGuard key exchange
4. Add environment provisioning logic
5. Implement billing/usage tracking

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `LISTEN_ADDR` | `:8080` | HTTP server listen address |
| `READ_TIMEOUT` | `15s` | HTTP read timeout |
| `WRITE_TIMEOUT` | `15s` | HTTP write timeout |
| `IDLE_TIMEOUT` | `60s` | HTTP idle timeout |
| `POOL_MIN_READY` | `2` | Minimum ready VMs in pool |
| `POOL_MAX_TOTAL` | `10` | Maximum total VMs |
| `POOL_VM_SIZE` | `cx11` | VM size/type |
| `POOL_REGION` | `nbg1` | VM region |
| `POOL_IMAGE_ID` | (required) | VM image ID |
| `PROVIDER_TYPE` | `hetzner` | Cloud provider type |
| `HETZNER_TOKEN` | (required) | Hetzner API token |
| `BILLING_ENABLED` | `false` | Enable billing features |
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `AUTO_DESTROY_HOURS` | `24` | Auto-destroy idle environments after N hours |

## License

MIT © 2026 Cilo Authors
