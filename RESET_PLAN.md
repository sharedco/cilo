# Cilo `just server-up` Reset & Fix Plan

## Current State Analysis

### What's Running Now
- **Server**: Running on PID 382804, healthy at :8080
- **Agent**: Running on PID 389998, healthy at :8081 (but machine_id is empty)
- **Containers**: No cilo-related containers currently running
- **Database**: Postgres running on port 5433 (mapped from container's 5432)

### Issues Identified

#### 1. **Port Confusion** (CRITICAL)
- Server runs on `:8080` (correct)
- Agent runs on `:8081` (correct in start-server.sh)
- BUT agent config defaults to `:8080` in `config/config.go` line 37
- This creates a conflict if agent starts without CILO_AGENT_LISTEN env var

#### 2. **WireGuard Private Key Missing** (CRITICAL)
- Agent config requires `CILO_WG_PRIVATE_KEY` (wireguard.go line 31-33)
- start-server.sh does NOT generate or set this key
- Agent starts but WireGuard manager fails silently (server.go line 28-30)
- Machine registration will fail because no wg_public_key available

#### 3. **Agent Environment Variables Incomplete** (HIGH)
- start-server.sh sets `CILO_AGENT_LISTEN=0.0.0.0:8081`
- Missing: `CILO_WG_PRIVATE_KEY`, `CILO_SERVER_URL`, `CILO_MACHINE_ID`
- Agent runs but can't properly register with server

#### 4. **Database Schema Mismatch** (HIGH)
- Machines table requires: `wg_public_key`, `wg_endpoint` (NOT NULL)
- start-server.sh tries to register machine but doesn't provide WireGuard info
- Machine add will fail with constraint violations

#### 5. **Workspace Directory Not Created** (MEDIUM)
- Agent defaults to `/var/cilo/envs` (config.go line 40)
- start-server.sh doesn't create this directory
- Environment operations will fail

#### 6. **Missing Caddyfile** (LOW)
- docker-compose.yml references `./caddy/Caddyfile`
- File likely doesn't exist in fresh clone
- Caddy container will fail to start

---

## Detailed Fix Plan

### Phase 1: Cleanup (Current State)

```bash
# Stop all cilo processes
sudo pkill -f "cilo-agent" || true
sudo pkill -f "cilo-server" || true  # if running outside docker

# Stop and remove containers
cd /var/deployment/sharedco/cilo/deploy/self-host
docker compose down -v  # -v removes volumes too

# Remove old env file (optional - will be regenerated)
rm -f .env

# Clean up workspace directory
sudo rm -rf /var/cilo/envs

# Remove installed binary
sudo rm -f /usr/local/bin/cilo-agent
```

### Phase 2: Fix start-server.sh

**Changes needed:**

1. **Generate WireGuard keys** (add after line 12):
```bash
# Generate WireGuard keys if not present
WG_PRIVATE_KEY=$(wg genkey 2>/dev/null || openssl rand -base64 32)
WG_PUBLIC_KEY=$(echo "$WG_PRIVATE_KEY" | wg pubkey 2>/dev/null || echo "")
```

2. **Store keys in env file** (add to .env creation, line 18-28):
```bash
CILO_WG_PRIVATE_KEY=${WG_PRIVATE_KEY}
CILO_WG_PUBLIC_KEY=${WG_PUBLIC_KEY}
```

3. **Create workspace directory** (after line 34):
```bash
# Create agent workspace directory
sudo mkdir -p /var/cilo/envs
sudo chmod 755 /var/cilo/envs
```

4. **Fix agent startup with proper env vars** (replace lines 74-86):
```bash
if ! pgrep -x "cilo-agent" > /dev/null; then
  echo "Starting cilo-agent..."
  
  # Get machine ID after registration (we need to register first or use hostname)
  MACHINE_ID="$(hostname)-self"
  
  export CILO_AGENT_LISTEN=0.0.0.0:8081
  export CILO_WG_PRIVATE_KEY="${WG_PRIVATE_KEY}"
  export CILO_WG_ADDRESS="10.225.0.100/16"
  export CILO_SERVER_URL="http://localhost:8080"
  export CILO_MACHINE_ID="${MACHINE_ID}"
  export CILO_WORKSPACE_DIR="/var/cilo/envs"
  
  # Ensure interface is created (requires sudo)
  sudo /usr/local/bin/cilo-agent > ~/cilo-agent.log 2>&1 &
  
  sleep 2
  if pgrep -x "cilo-agent" > /dev/null; then
    echo "✓ cilo-agent started (PID: $(pgrep -x cilo-agent))"
  else
    echo "⚠ Failed to start cilo-agent, check ~/cilo-agent.log"
    cat ~/cilo-agent.log
  fi
else
  echo "✓ cilo-agent already running (PID: $(pgrep -x cilo-agent))"
fi
```

5. **Fix machine registration** (lines 88-99):
- Need to get the agent's WireGuard public key first
- Then register with the server including wg_public_key

```bash
# Register this machine as an agent
echo ""
echo "Registering this machine as an agent..."

TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "127.0.0.1")
CURRENT_USER=$(whoami)
MACHINE_NAME="$(hostname)-self"

# Get WireGuard public key from agent
WG_PUBKEY=$(echo "$WG_PRIVATE_KEY" | wg pubkey 2>/dev/null || echo "")

# Remove existing machine entry if present
docker compose exec server cilo-server machines remove "$MACHINE_NAME" 2>/dev/null || true

# Add machine with WireGuard info
docker compose exec server cilo-server machines add \
  --name "$MACHINE_NAME" \
  --host "$TAILSCALE_IP" \
  --ssh-user "$CURRENT_USER" \
  --size manual \
  --wg-public-key "$WG_PUBKEY" \
  --wg-endpoint "$TAILSCALE_IP:51820"
```

### Phase 3: Fix Agent Config Defaults

**File: `internal/agent/config/config.go`**

Change line 37:
```go
ListenAddr:   getEnv("CILO_AGENT_LISTEN", "0.0.0.0:8081"),  // Changed from 8080
```

Add key generation helper (optional - could be in startup script):
The config should NOT generate keys, but should validate they're present.

### Phase 4: Fix docker-compose.yml

**File: `deploy/self-host/docker-compose.yml`**

1. **Add Caddyfile creation** (needs to be done in start-server.sh or exist in repo)

2. **Consider removing Caddy for local dev** (simpler setup):
   - For localhost development, Caddy adds complexity
   - Can access server directly at :8080
   - Keep Caddy for production deployments

3. **Add volume for agent workspace** (if agent runs in container):
   - Not needed for host-running agent

### Phase 5: Update Justfile

**File: `Justfile`**

Add cleanup command:
```just
# Clean up all server state (containers, volumes, processes)
server-clean:
  @echo "Cleaning up Cilo server state..."
  @sudo pkill -f "cilo-agent" 2>/dev/null || true
  @cd deploy/self-host && docker compose down -v 2>/dev/null || true
  @sudo rm -rf /var/cilo/envs 2>/dev/null || true
  @sudo rm -f /usr/local/bin/cilo-agent 2>/dev/null || true
  @rm -f deploy/self-host/.env
  @echo "✓ Cleanup complete"
```

Update server-up to show better errors:
```just
# Start self-hosted server and agent (requires sudo for agent install)
server-up:
  @echo "Starting Cilo Server + Agent..."
  @echo "This requires sudo to install cilo-agent to /usr/local/bin"
  @if ! command -v wg &> /dev/null; then \
    echo "✗ WireGuard (wg) not installed. Please install first:"; \
    echo "  Ubuntu/Debian: sudo apt install wireguard"; \
    exit 1; \
  fi
  @sudo ./scripts/start-server.sh
```

---

## Implementation Priority

### Must Fix (Blocking)
1. WireGuard key generation in start-server.sh
2. Agent environment variables (CILO_WG_PRIVATE_KEY, etc.)
3. Machine registration with WireGuard params
4. Workspace directory creation

### Should Fix (Better UX)
5. Agent config default port (8081 not 8080)
6. Caddyfile existence check/creation
7. Pre-flight checks (wg installed, docker running)

### Nice to Have
8. Better logging and error messages
9. Health check improvements
10. Idempotent setup (can run multiple times safely)

---

## Testing Checklist

After fixes, verify:
- [ ] `just server-clean` works
- [ ] `just server-up` completes without errors
- [ ] Server responds at :8080/health
- [ ] Agent responds at :8081/health with machine_id set
- [ ] Machine appears in `just machines` output
- [ ] Machine has wg_public_key populated
- [ ] Can create environment via API
- [ ] Agent can create Docker networks
- [ ] Workspace directory /var/cilo/envs exists and is writable

---

## Configuration Summary

### Automated (Generated by start-server.sh)
- `POSTGRES_PASSWORD` - Random 32-byte base64
- `CILO_WG_PRIVATE_KEY` - Generated via `wg genkey`
- `CILO_WG_PUBLIC_KEY` - Derived from private key
- `CILO_DOMAIN` - Set to "localhost" for local dev

### Manual (User can override in .env)
- `CILO_PROVIDER` - "manual" or "hetzner"
- `CILO_BILLING_ENABLED` - true/false
- `CILO_METRICS_ENABLED` - true/false
- `CILO_AUTO_DESTROY_HOURS` - Number or 0 to disable

### Runtime (Set by start-server.sh for agent)
- `CILO_AGENT_LISTEN` - "0.0.0.0:8081"
- `CILO_WG_ADDRESS` - "10.225.0.100/16"
- `CILO_SERVER_URL` - "http://localhost:8080"
- `CILO_MACHINE_ID` - "$(hostname)-self"
- `CILO_WORKSPACE_DIR` - "/var/cilo/envs"
