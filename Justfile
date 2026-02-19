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

# Build cilo-agent binary (with version info)
build-agent:
  go build -ldflags "-X github.com/sharedco/cilo/internal/version.Version={{VERSION}} -X github.com/sharedco/cilo/internal/version.Commit={{COMMIT}} -X github.com/sharedco/cilo/internal/version.BuildTime={{BUILDTIME}}" -o cilo-agent ./cmd/cilo-agent

# Build all binaries (with version info)
build-all: build build-agent

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

# Start the agent locally (requires sudo, reads keys from /etc/cilo/)
run-agent:
  #!/usr/bin/env bash
  TAILSCALE_IP=$(tailscale ip -4 2>/dev/null || hostname -I | awk '{print $1}')
  sudo -E bash -c "\
    export CILO_AGENT_LISTEN=0.0.0.0:8081 \
    CILO_WORKSPACE_DIR=/var/cilo/envs \
    CILO_WG_INTERFACE=wg0 \
    CILO_WG_PORT=51820 \
    CILO_WG_ADDRESS=10.225.0.100/16 \
    CILO_WG_ENDPOINT=${TAILSCALE_IP}:51820 \
    CILO_WG_PRIVATE_KEY=\$(cat /etc/cilo/agent-private.key); \
    /usr/local/bin/cilo-agent > /tmp/cilo-agent.log 2>&1 &"
  sleep 2
  if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✓ Agent running (PID: $(pgrep -x cilo-agent))"
  else
    echo "✗ Agent failed to start — check /tmp/cilo-agent.log"
  fi

# Reinstall and restart the agent (build + install + start)
restart-agent: install-agent run-agent

# Stop the agent locally (requires sudo)
stop-agent:
  sudo pkill -x cilo-agent 2>/dev/null || true
  @echo "✓ Stopped cilo-agent (if running)"

# Deploy agent to a remote Linux machine (cross-compile, scp, restart)
# Usage: just deploy-agent user@host
deploy-agent target: build-agent-linux
  @echo "Deploying cilo-agent to {{target}}..."
  scp cilo-agent-linux {{target}}:/tmp/cilo-agent
  ssh {{target}} 'sudo pkill -x cilo-agent 2>/dev/null; sleep 1; sudo cp /tmp/cilo-agent /usr/local/bin/cilo-agent && sudo chmod +x /usr/local/bin/cilo-agent && rm /tmp/cilo-agent && echo "✓ Agent binary installed"'
  @echo "✓ Deployed. Restart the agent on {{target}} with the appropriate env vars."
  @echo "  Or run: just start-remote-agent {{target}}"

# Start the agent on a remote machine (requires keys at /etc/cilo/)
start-remote-agent target:
  ssh -t {{target}} 'TAILSCALE_IP=$$(tailscale ip -4 2>/dev/null || hostname -I | awk "{print \$$1}") && \
    sudo -E bash -c "export CILO_AGENT_LISTEN=0.0.0.0:8081 CILO_WORKSPACE_DIR=/var/cilo/envs CILO_WG_INTERFACE=wg0 CILO_WG_PORT=51820 CILO_WG_ADDRESS=10.225.0.100/16 CILO_WG_ENDPOINT=$$TAILSCALE_IP:51820 CILO_WG_PRIVATE_KEY=\$$(cat /etc/cilo/agent-private.key); /usr/local/bin/cilo-agent > /tmp/cilo-agent.log 2>&1 &" && \
    sleep 2 && curl -s http://localhost:8081/health && echo ""'

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
  rm -f cilo cilo-agent
  go clean -cache

# Run cilo doctor
doctor:
  ./cilo doctor

# Initialize cilo (requires sudo)
init:
  sudo ./cilo init

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
