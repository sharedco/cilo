@_default:
  just --list

# Build cilo binary
build:
  cd cilo && make build

# Quick dev build (no version info)
dev:
  cd cilo && make dev

# Install cilo to ~/.local/bin for development
dev-install:
  cd cilo && make dev-install

# Install cilo to /usr/local/bin (requires sudo)
install:
  cd cilo && make install

# Show current version
version:
  cd cilo && make version

# Bump patch version and build (0.2.5 -> 0.2.6)
patch:
  cd cilo && make patch

# Bump minor version and build (0.2.5 -> 0.3.0)
minor:
  cd cilo && make minor

# Bump major version and build (0.2.5 -> 1.0.0)
major:
  cd cilo && make major

# Quick development cycle: bump patch, build, and dev-install
ship:
  cd cilo && make patch
  cd cilo && make dev-install
  @echo ""
  @echo "🚀 Ready to test!"
  @cd cilo && make version

# Clean build artifacts
clean:
  cd cilo && make clean

# Run unit tests
test *args:
  cd cilo && go test {{args}} ./...

# Run tests with verbose output
test-verbose:
  cd cilo && go test -v ./...

# Run unit tests only
test-unit *args:
  cd cilo && make test {{args}}

# Run E2E tests
test-e2e:
  cd cilo && CILO_E2E=1 go test -tags e2e ./tests/e2e

# Run integration tests (shared services - quick)
test-integration:
  ./tests/integration/verify-shared-services.sh

# Run integration tests (full suite)
test-integration-full:
  CILO_E2E=1 ./tests/integration/test-shared-services.sh

# Run all tests (unit + e2e + integration)
test-all: test-unit test-e2e test-integration

# Format Go code
fmt:
  cd cilo && go fmt ./...

# Run Go linter
lint:
  cd cilo && golangci-lint run || go vet ./...

# Check for issues before commit
check: fmt lint test-unit
  @echo "✓ All checks passed"
