# Build cilo binary
build:
  go build -o cilo ./cmd/cilo

# Build cilo-server binary
build-server:
  go build -o cilo-server ./cmd/cilo-server

# Build cilo-agent binary
build-agent:
  go build -o cilo-agent ./cmd/cilo-agent

# Build all binaries
build-all: build build-server build-agent

# Quick dev build (no version info)
dev:
  go build -o cilo ./cmd/cilo

# Install cilo to ~/.local/bin for development
dev-install: dev
  mkdir -p ~/.local/bin
  cp cilo ~/.local/bin/
  @echo "Installed to ~/.local/bin/cilo"

# Install cilo to /usr/local/bin (requires sudo)
install: build
  sudo cp cilo /usr/local/bin/
  @echo "Installed to /usr/local/bin/cilo"

# Show current version
version:
  ./cilo version

# Run unit tests
test *args:
  go test {{args}} ./internal/...

# Run tests with verbose output
test-verbose:
  go test -v ./internal/...

# Run E2E tests
test-e2e: build
  export CILO_E2E_ENABLED=true && export CILO_BINARY=$(pwd)/cilo && go test -tags e2e ./test/e2e/...

# Run integration tests (shared services - quick)
test-integration:
  ./test/integration/verify-shared-services.sh

# Run integration tests (full suite)
test-integration-full:
  export CILO_E2E=1 && ./test/integration/test-shared-services.sh

# Run all tests (unit + e2e + integration)
test-all: test test-e2e test-integration

# Format Go code
fmt:
  go fmt ./...

# Run Go linter
lint:
  golangci-lint run || go vet ./...

# Check for issues before commit
check: fmt lint test
  @echo "✓ All checks passed"

# Clean build artifacts
clean:
  rm -f cilo cilo-server cilo-agent
  go clean -cache

# Run cilo doctor
doctor:
  ./cilo doctor

# Initialize cilo (requires sudo)
init:
  sudo ./cilo init

# Start self-hosted server ONLY (no agent)
# Use this if you plan to add external agents only
server-up:
  @echo "Starting Cilo Server..."
  @cd deploy/self-host && \
  if [ ! -f .env ]; then \
    POSTGRES_PASSWORD=$$(openssl rand -base64 32); \
    echo "CILO_DOMAIN=localhost" > .env; \
    echo "POSTGRES_PASSWORD=$$POSTGRES_PASSWORD" >> .env; \
    echo "CILO_PROVIDER=manual" >> .env; \
    echo "CILO_BILLING_ENABLED=false" >> .env; \
    echo "CILO_METRICS_ENABLED=true" >> .env; \
    echo "CILO_AUTO_DESTROY_HOURS=8" >> .env; \
    echo "✓ Created .env file"; \
  fi
  @cd /var/deployment/sharedco/cilo && DOCKER_BUILDKIT=0 docker build -t cilo-server:local -f server/Dockerfile . > /dev/null 2>&1
  @cd deploy/self-host && docker compose up -d
  @echo "Waiting for server to start..."
  @sleep 5
  @for i in {1..30}; do \
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then \
      echo "✓ Server is healthy"; \
      break; \
    fi; \
    if [ $$i -eq 30 ]; then \
      echo "✗ Server failed to start"; \
      docker compose logs server; \
      exit 1; \
    fi; \
    sleep 1; \
  done
  @docker exec self-host-postgres-1 psql -U cilo -d cilo -c "INSERT INTO teams (id, name, created_at) VALUES ('team-default', 'Default Team', NOW()) ON CONFLICT DO NOTHING;" > /dev/null 2>&1 || true
  @echo ""
  @echo "Creating admin API key..."
  @cd deploy/self-host && docker compose exec server cilo-server admin create-key --team team-default --scope admin --name "admin-key"
  @echo ""
  @echo "Server running at http://localhost:8080"
  @echo ""
  @echo "Next steps:"
  @echo "  just server-add-self    # Register this machine as an agent"
  @echo "  just add-machine-ts     # Add external machines"

# Start server AND register this machine as an agent (recommended for single-machine setup)
server-up-self: server-up server-add-self

# Register this machine as an agent (after server-up)
server-add-self:
  @echo "Registering this machine as an agent..."
  @cd deploy/self-host
  @TAILSCALE_IP=$$(tailscale ip -4 2>/dev/null || echo "127.0.0.1"); \
  CURRENT_USER=$$(whoami); \
  MACHINE_NAME="$$(hostname)-self"; \
  docker compose exec server cilo-server machines remove "$$MACHINE_NAME" 2>/dev/null || true; \
  docker compose exec server cilo-server machines add --name "$$MACHINE_NAME" --host "$$TAILSCALE_IP" --ssh-user "$$CURRENT_USER" --size manual; \
  echo ""; \
  echo "✓ This machine (\033[1m$$MACHINE_NAME\033[0m) is now an agent!"; \
  echo ""
  @just machines

# Stop self-hosted server
server-down:
  cd deploy/self-host && docker compose down

# View server logs
server-logs:
  cd deploy/self-host && docker compose logs -f server

# Check server health
server-status:
  @curl -s http://localhost:8080/health || echo "Server not running"
  @echo "Server URL: http://localhost:8080"

# List registered machines
machines:
  cd deploy/self-host && docker compose exec server cilo-server machines list

# Remove a machine from the pool
remove-machine name:
  cd deploy/self-host && docker compose exec server cilo-server machines remove {{name}}

# Register an external machine via Tailscale
add-machine-ts name tailscale-ip user:
  cd deploy/self-host && docker compose exec server cilo-server machines add --name {{name}} --host {{tailscale-ip}} --ssh-user {{user}} --size manual
  @echo "✓ Machine {{name}} added. Run 'just machines' to verify."
