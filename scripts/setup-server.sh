#!/bin/bash
set -e

echo "Setting up Cilo Server on Linux..."

cd /var/deployment/sharedco/cilo/deploy/self-host

if [ ! -f .env ]; then
  POSTGRES_PASSWORD=$(openssl rand -base64 32)
  cat > .env << EOF
CILO_DOMAIN=localhost
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
CILO_PROVIDER=manual
CILO_BILLING_ENABLED=false
CILO_METRICS_ENABLED=true
CILO_AUTO_DESTROY_HOURS=8
EOF
  echo "✓ Created .env file"
fi

echo "Building cilo-server image..."
cd /var/deployment/sharedco/cilo
DOCKER_BUILDKIT=0 docker build -t cilo-server:local -f server/Dockerfile .

echo "Starting Cilo Server..."
cd /var/deployment/sharedco/cilo/deploy/self-host
docker compose up -d

echo "Waiting for server to start..."
sleep 5
for i in {1..30}; do
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

echo ""
echo "Creating default team and API key..."
docker exec self-host-postgres-1 psql -U cilo -d cilo -c "INSERT INTO teams (id, name, created_at) VALUES ('team-default', 'Default Team', NOW()) ON CONFLICT DO NOTHING;" > /dev/null 2>&1
docker exec self-host-server-1 /app/cilo-server admin create-key --team team-default --scope admin --name "test-admin-key"

echo ""
echo "====================================="
echo "Server Setup Complete!"
echo "====================================="
echo "Server URL: http://localhost:8080"
echo ""
echo "Next steps:"
echo "1. Save the API key shown above"
echo "2. Set: export CILO_API_KEY=<key-from-above>"
echo "3. Login: cilo cloud login --server http://localhost:8080"
echo "4. Add your Mac: ./scripts/add-machine.sh mac-agent <ip> <user>"
echo "====================================="
