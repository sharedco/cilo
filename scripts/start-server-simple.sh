#!/bin/bash
set -e

echo "Starting minimal Cilo Server..."
echo ""

if ! docker ps | grep -q cilo-postgres; then
  echo "Starting PostgreSQL on port 5433..."
  docker run -d \
    --name cilo-postgres \
    -e POSTGRES_PASSWORD=cilo \
    -e POSTGRES_USER=cilo \
    -e POSTGRES_DB=cilo \
    -p 5433:5432 \
    postgres:16-alpine
  
  echo "Waiting for PostgreSQL..."
  sleep 5
  for i in {1..30}; do
    if docker exec cilo-postgres pg_isready -U cilo > /dev/null 2>&1; then
      echo "✓ PostgreSQL ready"
      break
    fi
    if [ $i -eq 30 ]; then
      echo "✗ PostgreSQL failed to start"
      exit 1
    fi
    sleep 1
  done
fi

if [ ! -f /var/deployment/sharedco/cilo/cilo-server ]; then
  echo "Building cilo-server..."
  cd /var/deployment/sharedco/cilo
  go build -o cilo-server ./cmd/cilo-server
fi

echo "Starting cilo-server on :8080..."
export DATABASE_URL=postgres://cilo:cilo@localhost:5433/cilo?sslmode=disable
export CILO_LISTEN_ADDR=:8080
export CILO_PROVIDER=manual

echo ""
echo "Creating admin API key..."
/var/deployment/sharedco/cilo/cilo-server admin create-key --scope admin --name "dev-key" || true

echo ""
echo "========================================"
echo "Server is running!"
echo "========================================"
echo "URL: http://localhost:8080"
echo ""
echo "To use:"
echo "  cilo cloud login --server http://localhost:8080"
echo ""
echo "Press Ctrl+C to stop"
echo "========================================"

/var/deployment/sharedco/cilo/cilo-server serve
