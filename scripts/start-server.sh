#!/bin/bash
set -e

echo "Starting Cilo Server with self-registration..."
echo ""

cd /var/deployment/sharedco/cilo/deploy/self-host

# Create env file if needed
if [ ! -f .env ]; then
  POSTGRES_PASSWORD=$(openssl rand -base64 32)
  cat > .env << EOF
CILO_DOMAIN=localhost
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
CILO_PROVIDER=manual
CILO_BILLING_ENABLED=false
CILO_METRICS_ENABLED=true
CILO_AUTO_DESTROY_HOURS=8
EOF
  echo "✓ Created .env file"
fi

# Build image
echo "Building cilo-server image..."
cd /var/deployment/sharedco/cilo
DOCKER_BUILDKIT=0 docker build -t cilo-server:local -f server/Dockerfile . > /dev/null 2>&1

# Start services
echo "Starting services..."
cd /var/deployment/sharedco/cilo/deploy/self-host
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
docker compose exec server cilo-server admin create-key --team team-default --scope admin --name "admin-key"

# Register this machine as agent
echo ""
echo "Registering this machine as an agent..."
TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || echo "127.0.0.1")
CURRENT_USER=$(whoami)
MACHINE_NAME="$(hostname)-self"

docker compose exec server cilo-server machines remove "$MACHINE_NAME" 2>/dev/null || true
docker compose exec server cilo-server machines add --name "$MACHINE_NAME" --host "$TAILSCALE_IP" --ssh-user "$CURRENT_USER" --size manual

echo ""
docker compose exec server cilo-server machines list

echo ""
echo "====================================="
echo "Server + Agent Setup Complete!"
echo "====================================="
echo ""
echo "This Linux machine is now:"
echo "  ✓ Server (API + Database)"
echo "  ✓ Agent (runs containers)"
echo ""
echo "From your Mac:"
echo "  export CILO_API_KEY=<key-shown-above>"
echo "  cilo cloud login --server http://$TAILSCALE_IP:8080"
echo "  cilo cloud up my-app --from ./project"
echo ""
echo "Commands will run on THIS machine!"
echo "====================================="
