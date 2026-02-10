#!/bin/bash
set -e

echo "Starting Cilo Server with self-registration..."
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Build cilo-agent first (as regular user with full PATH)
if [ ! -f "$PROJECT_ROOT/cilo-agent" ]; then
  echo "Building cilo-agent..."
  cd "$PROJECT_ROOT"
  go build -o cilo-agent ./cmd/cilo-agent
  echo "✓ cilo-agent built"
fi

# Create workspace directory
echo "Creating workspace directory..."
sudo mkdir -p /var/cilo/envs
sudo chown -R $(whoami):$(whoami) /var/cilo/envs 2>/dev/null || sudo chown -R $(whoami) /var/cilo/envs

# Check for WireGuard tools
if ! command -v wg &> /dev/null; then
  echo "⚠ WireGuard not installed. Installing..."
  if command -v apt-get &> /dev/null; then
    sudo apt-get update && sudo apt-get install -y wireguard
  elif command -v yum &> /dev/null; then
    sudo yum install -y wireguard-tools
  else
    echo "⚠ Could not install WireGuard automatically. Please install manually."
  fi
fi

# Generate WireGuard keys (reuse existing if available)
WG_PRIVATE_KEY=""
WG_PUBLIC_KEY=""
if command -v wg &> /dev/null; then
  if [ -f /etc/cilo/agent-private.key ]; then
    WG_PRIVATE_KEY=$(sudo cat /etc/cilo/agent-private.key)
    WG_PUBLIC_KEY=$(echo "$WG_PRIVATE_KEY" | wg pubkey)
    echo "✓ Reusing existing WireGuard keys"
  else
    WG_PRIVATE_KEY=$(wg genkey)
    WG_PUBLIC_KEY=$(echo "$WG_PRIVATE_KEY" | wg pubkey)
    echo "✓ Generated new WireGuard keys"
  fi
else
  echo "⚠ WireGuard tools not available - will generate keys later"
fi

# Get machine info
TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || hostname -I | awk '{print $1}')
CURRENT_USER=$(whoami)
MACHINE_NAME="$(hostname)-self"

# Create env file
cd "$PROJECT_ROOT/deploy/self-host"
if [ ! -f .env ]; then
  POSTGRES_PASSWORD=$(openssl rand -hex 32)
  POSTGRES_PASSWORD_URLENCODED=$(python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$POSTGRES_PASSWORD")
  cat > .env << EOF
 CILO_DOMAIN=localhost
 POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
 POSTGRES_PASSWORD_URLENCODED=${POSTGRES_PASSWORD_URLENCODED}
 CILO_PROVIDER=manual
 CILO_BILLING_ENABLED=false
 CILO_METRICS_ENABLED=true
 CILO_AUTO_DESTROY_HOURS=8
EOF
  echo "✓ Created .env file"
fi

# Add agent environment to .env
cat >> .env << EOF

# Agent Configuration
CILO_AGENT_LISTEN=0.0.0.0:8081
CILO_WORKSPACE_DIR=/var/cilo/envs
CILO_WG_INTERFACE=wg0
CILO_WG_PORT=51820
CILO_WG_ADDRESS=10.225.0.100/16
EOF

# Save WireGuard keys separately if generated
if [ -n "$WG_PRIVATE_KEY" ]; then
  sudo mkdir -p /etc/cilo
  echo "$WG_PRIVATE_KEY" | sudo tee /etc/cilo/agent-private.key > /dev/null
  echo "$WG_PUBLIC_KEY" | sudo tee /etc/cilo/agent-public.key > /dev/null
  sudo chmod 600 /etc/cilo/agent-private.key
  echo "✓ WireGuard keys saved to /etc/cilo/"
fi

# Build server image
echo "Building cilo-server image..."
cd "$PROJECT_ROOT"
DOCKER_BUILDKIT=0 docker build -t cilo-server:local -f server/Dockerfile . > /dev/null 2>&1

# Start services
echo "Starting services..."
cd "$PROJECT_ROOT/deploy/self-host"
docker compose up -d

# Wait for server
echo "Waiting for server..."
sleep 5
for i in $(seq 1 30); do
  if curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "✓ Server is healthy"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "✗ Server failed to start"
    docker compose logs server
    exit 1
  fi
  sleep 1
done

# Create default team
docker exec self-host-postgres-1 psql -U cilo -d cilo -c "INSERT INTO teams (id, name, created_at) VALUES ('team-default', 'Default Team', NOW()) ON CONFLICT DO NOTHING;" > /dev/null 2>&1 || true

# Create API key
echo ""
echo "Creating admin API key..."
API_KEY_OUTPUT=$(docker compose exec server cilo-server admin create-key --team team-default --scope admin --name "admin-key" 2>&1)
echo "$API_KEY_OUTPUT"

# Extract API key from output
API_KEY=$(echo "$API_KEY_OUTPUT" | grep -oE 'cilo_[A-Za-z0-9_-]+' | head -1)

# Install and start agent
echo ""
echo "Installing cilo-agent..."
if [ ! -f /usr/local/bin/cilo-agent ]; then
  echo "Copying cilo-agent to /usr/local/bin (requires sudo)..."
  sudo cp "$PROJECT_ROOT/cilo-agent" /usr/local/bin/
  sudo chmod +x /usr/local/bin/cilo-agent
  echo "✓ cilo-agent installed"
fi

# Stop any existing agent
if pgrep -x "cilo-agent" > /dev/null; then
  echo "Stopping existing cilo-agent..."
  sudo pkill -x cilo-agent || true
  sleep 1
fi

# Register machine
echo ""
echo "Registering this machine as an agent..."
docker compose exec server cilo-server machines remove "$MACHINE_NAME" 2>/dev/null || true
docker compose exec server cilo-server machines add \
  --name "$MACHINE_NAME" \
  --host "$TAILSCALE_IP" \
  --ssh-user "$CURRENT_USER" \
  --size manual

# Update machine with WireGuard key if available
if [ -n "$WG_PUBLIC_KEY" ]; then
  echo "Updating machine with WireGuard public key..."
  docker exec self-host-postgres-1 psql -U cilo -d cilo -c "
    UPDATE machines 
    SET wg_public_key = '$WG_PUBLIC_KEY', 
        wg_endpoint = '$TAILSCALE_IP:51820' 
    WHERE id = '$MACHINE_NAME';
  " > /dev/null 2>&1 || echo "⚠ Could not update WireGuard key in database"
fi

echo ""
docker compose exec server cilo-server machines list

# Agent requires root for WireGuard interface creation (ip link add, wg set)
echo ""
echo "Starting cilo-agent with sudo (required for WireGuard)..."
sudo -E bash -c '
  export CILO_AGENT_LISTEN="0.0.0.0:8081"
  export CILO_WORKSPACE_DIR="/var/cilo/envs"
  export CILO_WG_INTERFACE="wg0"
  export CILO_WG_PORT="51820"
  export CILO_WG_ADDRESS="10.225.0.100/16"
  export CILO_SERVER_URL="http://localhost:8080"
  export CILO_MACHINE_ID="'"$MACHINE_NAME"'"
  
  if [ -f /etc/cilo/agent-private.key ]; then
    export CILO_WG_PRIVATE_KEY="$(cat /etc/cilo/agent-private.key)"
    echo "✓ Loaded WireGuard private key"
  else
    echo "⚠ WireGuard private key not found at /etc/cilo/agent-private.key"
  fi
  
  /usr/local/bin/cilo-agent > /tmp/cilo-agent.log 2>&1 &
'
sleep 3

if pgrep -x "cilo-agent" > /dev/null; then
  echo "✓ cilo-agent started (PID: $(pgrep -x cilo-agent))"
  echo "  - HTTP API: http://$TAILSCALE_IP:8081"
  if [ -n "$WG_PUBLIC_KEY" ]; then
    echo "  - WireGuard: $TAILSCALE_IP:51820 (pubkey: ${WG_PUBLIC_KEY:0:20}...)"
  fi
else
  echo "✗ Failed to start cilo-agent"
  echo "Check logs: tail -f /tmp/cilo-agent.log"
  exit 1
fi

# Test agent health
echo ""
echo "Testing agent..."
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
  echo "✓ Agent is healthy"
else
  echo "⚠ Agent health check failed (may need WireGuard setup)"
fi

echo ""
echo "====================================="
echo "Server + Agent Setup Complete!"
echo "====================================="
echo ""
echo "This Linux machine is now:"
echo "  ✓ Server (API + Database) on http://$TAILSCALE_IP:8080"
echo "  ✓ Agent (runs containers) on http://$TAILSCALE_IP:8081"
if [ -n "$WG_PUBLIC_KEY" ]; then
  echo "  ✓ WireGuard endpoint: $TAILSCALE_IP:51820"
fi
echo ""
echo "API Key:"
echo "  $API_KEY"
echo ""
echo "From your Mac:"
echo "  cilo cloud login --server http://$TAILSCALE_IP:8080"
echo "  # Enter API key above when prompted"
echo "  cilo cloud up my-app --from ./project"
echo ""
echo "Commands will run on THIS machine!"
echo "====================================="
