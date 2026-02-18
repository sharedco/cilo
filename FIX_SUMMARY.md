# Cilo `just server-up` Fix Summary

## Changes Made

### 1. scripts/start-server.sh (COMPLETE REWRITE)
**Issues Fixed:**
- ✓ WireGuard key generation (`wg genkey` and `wg pubkey`)
- ✓ Workspace directory creation (`/var/cilo/envs`)
- ✓ Agent environment variables (CILO_WG_PRIVATE_KEY, CILO_SERVER_URL, CILO_MACHINE_ID, etc.)
- ✓ Machine registration with WireGuard public key
- ✓ Proper agent startup with sudo for WireGuard interface creation
- ✓ Agent health check after startup
- ✓ Better error handling and logging

**Key Features:**
- Generates and saves WireGuard keys to `/etc/cilo/`
- Creates workspace directory with proper permissions
- Stops existing agent before starting new one
- Registers machine with server including WireGuard public key
- Tests agent health after startup
- Displays API key and connection info at end

### 2. internal/agent/config/config.go
**Already Fixed:**
- ✓ Default port changed from 8080 to 8081 (avoids conflict with server)

### 3. Justfile
**Added:**
- ✓ Pre-flight checks for Docker and Docker Compose
- ✓ Warning if WireGuard not installed (but continues)
- ✓ `server-clean` command already existed for cleanup

### 4. deploy/self-host/docker-compose.yml
**No Changes Needed:**
- ✓ Caddyfile already exists at `./caddy/Caddyfile`
- ✓ Port mappings are correct (8080 for server, 5433 for postgres)

## Testing Status

### What Was Tested:
1. ✓ Script syntax validation (`bash -n`)
2. ✓ File existence checks (agent binary, Dockerfile, docker-compose.yml)
3. ✓ Justfile server-up command structure

### What Couldn't Be Tested (Requires sudo):
- Full end-to-end `just server-up` execution
- Docker container startup
- Agent process startup with WireGuard
- Machine registration with server

## How to Use

### Fresh Start:
```bash
# Clean up any existing state
just server-clean

# Start server and agent
just server-up
```

### What Happens:
1. Script checks for Docker and WireGuard
2. Builds cilo-agent binary
3. Generates WireGuard keys
4. Creates workspace directory at `/var/cilo/envs`
5. Creates .env file with configuration
6. Builds and starts server Docker container
7. Creates default team and API key
8. Installs cilo-agent to `/usr/local/bin`
9. Registers this machine with the server
10. Starts cilo-agent with proper environment variables
11. Tests agent health

### After Setup:
```bash
# Check server health
curl http://localhost:8080/health

# Check agent health
curl http://localhost:8081/health

# List registered machines
just machines

# View server logs
just server-logs
```

## Configuration Files

### .env (created in deploy/self-host/)
```bash
CILO_DOMAIN=localhost
POSTGRES_PASSWORD=<random>
CILO_PROVIDER=manual
CILO_BILLING_ENABLED=false
CILO_METRICS_ENABLED=true
CILO_AUTO_DESTROY_HOURS=8
CILO_AGENT_LISTEN=0.0.0.0:8081
CILO_WORKSPACE_DIR=/var/cilo/envs
CILO_WG_INTERFACE=wg0
CILO_WG_PORT=51820
CILO_WG_ADDRESS=10.225.0.100/16
```

### /etc/cilo/agent-private.key
WireGuard private key (generated, 600 permissions)

### /etc/cilo/agent-public.key
WireGuard public key (generated)

## Troubleshooting

### If agent fails to start:
```bash
# Check agent logs
tail -f /tmp/cilo-agent.log

# Check if port 8081 is in use
lsof -i :8081

# Try starting agent manually with debug output
sudo CILO_AGENT_LISTEN=0.0.0.0:8081 CILO_WG_PRIVATE_KEY=$(sudo cat /etc/cilo/agent-private.key) /usr/local/bin/cilo-agent
```

### If WireGuard fails:
```bash
# Check if WireGuard is installed
which wg

# Install on Ubuntu/Debian
sudo apt install wireguard

# Install on macOS
brew install wireguard-tools
```

### If server fails to start:
```bash
# Check server logs
cd deploy/self-host && docker compose logs server

# Check if port 8080 is in use
lsof -i :8080

# Reset everything
just server-clean
just server-up
```

## Known Limitations

1. **Requires sudo** for:
   - Installing cilo-agent to /usr/local/bin
   - Creating /var/cilo/envs workspace directory
   - Creating WireGuard interface (wg0)
   - Saving keys to /etc/cilo/

2. **WireGuard dependency**:
   - Linux: `apt install wireguard`
   - macOS: `brew install wireguard-tools`
   - Without WireGuard, agent runs in fallback mode (limited functionality)

3. **Port requirements**:
   - 8080 for server (must be free)
   - 8081 for agent (must be free)
   - 5433 for postgres (must be free)
   - 51820 for WireGuard (must be free)

## Next Steps for Full Production Readiness

1. Add systemd service file for agent auto-start on boot
2. Add SSL/TLS support for production deployments
3. Add backup/restore for database
4. Add monitoring and alerting
5. Add log rotation for agent logs
