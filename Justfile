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
test-e2e:
  export CILO_E2E_ENABLED=true && export CILO_BINARY=./cilo && go test -tags e2e ./test/e2e/...

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
