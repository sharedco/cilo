# Export version info for all builds
export VERSION := `cat VERSION`
export COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
export BUILDTIME := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

# Show current version from VERSION file
show-version:
  @echo "Version: {{VERSION}}"
  @echo "Commit:  {{COMMIT}}"
  @echo "Build:   {{BUILDTIME}}"

# Bump patch version (e.g., 0.2.1 -> 0.2.2)
bump-version:
  @current=$(cat VERSION); \
  major=$(echo $current | cut -d. -f1); \
  minor=$(echo $current | cut -d. -f2); \
  patch=$(echo $current | cut -d. -f3); \
  new_patch=$((patch + 1)); \
  new_version="${major}.${minor}.${new_patch}"; \
  echo "$new_version" > VERSION; \
  echo "Bumped version: $current -> $new_version"

# Build cilo binary (with version info)
build:
  go build -ldflags "-X github.com/sharedco/cilo/internal/version.Version={{VERSION}} -X github.com/sharedco/cilo/internal/version.Commit={{COMMIT}} -X github.com/sharedco/cilo/internal/version.BuildTime={{BUILDTIME}}" -o cilo ./cmd/cilo

# Build cilo-server binary (with version info)
build-server:
  go build -ldflags "-X github.com/sharedco/cilo/internal/version.Version={{VERSION}} -X github.com/sharedco/cilo/internal/version.Commit={{COMMIT}} -X github.com/sharedco/cilo/internal/version.BuildTime={{BUILDTIME}}" -o cilo-server ./cmd/cilo-server

# Build cilo-agent binary (with version info)
build-agent:
  go build -ldflags "-X github.com/sharedco/cilo/internal/version.Version={{VERSION}} -X github.com/sharedco/cilo/internal/version.Commit={{COMMIT}} -X github.com/sharedco/cilo/internal/version.BuildTime={{BUILDTIME}}" -o cilo-agent ./cmd/cilo-agent

# Build all binaries (with version info)
build-all: build build-server build-agent

# Build agent for Linux (cross-compile)
build-agent-linux:
  GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/sharedco/cilo/internal/version.Version={{VERSION}} -X github.com/sharedco/cilo/internal/version.Commit={{COMMIT}} -X github.com/sharedco/cilo/internal/version.BuildTime={{BUILDTIME}}" -o cilo-agent-linux ./cmd/cilo-agent

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
  sudo codesign --force --sign - /usr/local/bin/cilo 2>/dev/null || true
  @echo "Installed to /usr/local/bin/cilo"

# Install cilo-agent to /usr/local/bin (requires sudo, forces rebuild)
install-agent: build-agent
  sudo pkill -x cilo-agent 2>/dev/null || true
  sudo cp cilo-agent /usr/local/bin/
  sudo chmod +x /usr/local/bin/cilo-agent
  @echo "Installed cilo-agent to /usr/local/bin/cilo-agent"

# Show version reported by cilo binary
version: build
  @./cilo --version

# Run unit tests
test *args:
  go test {{args}} ./internal/...

# Run tests with verbose output
test-verbose:
  go test -v ./internal/...

# Run E2E tests
test-e2e: build
  export CILO_E2E_ENABLED=true && export CILO_BINARY=$(pwd)/cilo && go test -tags e2e ./test/e2e/...

# Run E2E tests (local only, skip cloud auth tests)
test-e2e-local: build
  export CILO_E2E_ENABLED=true && export CILO_BINARY=$(pwd)/cilo && go test -tags e2e ./test/e2e/... -skip TestCloudLoginCommand

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

# Start self-hosted server and agent (requires sudo for agent install)
server-up:
  @echo "Starting Cilo Server + Agent..."
  @echo "This requires sudo to install cilo-agent to /usr/local/bin"
  @sudo ./scripts/start-server.sh

# Stop self-hosted server
server-down:
  cd deploy/self-host && docker compose down

# Clean up server completely (containers, volumes, envs, agent, wireguard)
server-clean:
  @echo "Cleaning up Cilo Server..."
  @echo "Stopping containers..."
  cd deploy/self-host && docker compose down -v 2>/dev/null || true
  @echo "Removing environment containers..."
  docker ps -aq --filter "name=cilo_" | xargs -r docker stop 2>/dev/null || true
  docker ps -aq --filter "name=cilo_" | xargs -r docker rm 2>/dev/null || true
  @echo "Removing environment networks..."
  docker network ls --format "{{{{.Name}}}}" | grep -E "^cilo_" | xargs -r docker network rm 2>/dev/null || true
  docker network ls --format "{{{{.Name}}}}" | grep -E "^[a-f0-9-]{36}_default$$" | xargs -r docker network rm 2>/dev/null || true
  @echo "Stopping cilo-agent..."
  sudo pkill -x cilo-agent 2>/dev/null || true
  @echo "Cleaning up WireGuard..."
  sudo ip link del wg0 2>/dev/null || true
  @echo "Cleaning up workspace..."
  sudo rm -rf /var/cilo/envs/* 2>/dev/null || true
  @echo "Cleaning up .env..."
  rm -f deploy/self-host/.env 2>/dev/null || true
  @echo "✓ Server cleaned up"

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

# Clean up all tunnel state (kills processes, removes state files)
tunnel-clean:
  @echo "Cleaning up tunnel state..."
  sudo cilo tunnel clean
  @echo "✓ Tunnel cleaned. Run 'sudo cilo cloud up <name>' to restart."

# Clean slate - destroy all environments, remove all state, ready for fresh init
clean-slate:
  @echo "🧹 Clean slate - removing all cilo environments and state..."
  @echo ""
  @echo "  → Stopping all cilo containers..."
  docker ps -aq --filter "name=cilo_" | xargs -r docker stop 2>/dev/null || true
  @echo "  → Removing all cilo containers..."
  docker ps -aq --filter "name=cilo_" | xargs -r docker rm 2>/dev/null || true
  @echo "  → Removing all cilo networks..."
  docker network ls --format "{{{{.Name}}}}" | grep -E "^cilo_" | xargs -r docker network rm 2>/dev/null || true
  @echo "  → Stopping dnsmasq..."
  sudo pkill -x dnsmasq 2>/dev/null || true
  @echo "  → Removing cilo state..."
  rm -rf ~/.cilo/envs/* ~/.cilo/state.json ~/.cilo/state.json.lock 2>/dev/null || true
  @echo "  → Preserving DNS config (will be updated on next init)..."
  @echo ""
  @echo "✓ Clean slate complete! Ready for fresh start:"
  @echo "   sudo cilo init"
  @echo "   cd examples/basic && cilo create myenv && cilo up myenv"

# Nuclear clean - wipe ALL cilo state on this machine (tunnel, cloud, dns, auth)
clean-all:
  @echo "🧹 Nuclear clean - removing ALL cilo state..."
  @echo "  → Killing tunnel processes..."
  sudo cilo tunnel clean 2>/dev/null || true
  @echo "  → Removing cloud state and auth..."
  rm -f ~/.cilo/state.json ~/.cilo/cloud-auth.json
  @echo "  → Removing DNS config..."
  rm -rf ~/.cilo/dns
  @echo "  → Removing local environments..."
  rm -rf ~/.cilo/envs
  @echo "  → Flushing DNS cache..."
  sudo dscacheutil -flushcache 2>/dev/null || true
  sudo killall -HUP mDNSResponder 2>/dev/null || true
  @echo "✓ All cilo state removed. Run 'just init' to start fresh."
