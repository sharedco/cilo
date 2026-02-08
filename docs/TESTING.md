# Testing Guide

This document provides comprehensive information about testing in the Cilo project, including unit tests, E2E tests, and integration tests.

## Table of Contents

- [Overview](#overview)
- [Unit Tests](#unit-tests)
  - [Cilo CLI](#cilo-cli)
  - [Server](#server)
  - [Agent](#agent)
- [E2E Tests](#e2e-tests)
  - [Requirements](#requirements)
  - [Environment Variables](#environment-variables)
  - [Running E2E Tests](#running-e2e-tests)
  - [Test Coverage](#test-coverage)
- [Integration Tests](#integration-tests)
- [MacOS Testing](#macos-testing)
- [Adding New Tests](#adding-new-tests)
- [Test Commands Reference](#test-commands-reference)

---

## Overview

Cilo uses a multi-layered testing approach:

| Test Type | Location | Purpose | Speed |
|-----------|----------|---------|-------|
| **Unit Tests** | `*_test.go` files | Test individual functions/packages | Fast (< 1s) |
| **E2E Tests** | `tests/e2e/`, `cilo/tests/e2e/` | Test full CLI workflows with Docker | Slow (minutes) |
| **Integration Tests** | `tests/integration/` | Test shared services and complex scenarios | Medium (seconds) |

---

## Unit Tests

### Cilo CLI

The main Cilo CLI has unit tests across multiple packages:

```bash
# Run all unit tests for cilo
cd cilo && go test ./...

# Run with verbose output
cd cilo && go test -v ./...

# Run specific package tests
cd cilo && go test ./pkg/parsers/...
cd cilo && go test ./pkg/engine/...
cd cilo && go test ./pkg/cloud/...
```

**Key Unit Test Files:**

| Package | Test File | Coverage |
|---------|-----------|----------|
| `pkg/parsers` | `compose_test.go` | Docker Compose parsing |
| `pkg/engine` | `engine_test.go` | Project detection & parsing |
| `pkg/cloud` | `sync_test.go` | Workspace sync, rsync, ignore patterns |
| `pkg/cloud` | `auth_test.go` | Authentication |
| `pkg/cloud/tunnel` | `tunnel_test.go`, `keys_test.go` | WireGuard tunneling |
| `pkg/dns` | `render_test.go` | DNS rendering |
| `pkg/env` | `env_test.go` | Environment variable handling |
| `pkg/models` | `config_test.go` | Configuration models |
| `pkg/compose` | `loader_test.go` | Compose file loading |
| `pkg/runtimes` | `podman_test.go` | Podman runtime support |

**Example: Running specific test:**
```bash
cd cilo && go test -v ./pkg/parsers -run TestComposeParser
```

### Server

The server module contains API endpoint tests:

```bash
cd server && go test ./...
```

**Key Test Files:**

| Package | Test File | Coverage |
|---------|-----------|----------|
| `pkg/api` | `server_test.go` | HTTP server, health endpoints, auth middleware |
| `pkg/api/handlers` | `wireguard_test.go` | WireGuard handlers |
| `pkg/wireguard` | `exchange_test.go` | Key exchange logic |

**Note:** Server tests require a PostgreSQL database. Tests will skip if no database is available:

```bash
# Set up test database
export DATABASE_URL="postgres://localhost/cilo_test?sslmode=disable"
cd server && go test ./...
```

### Agent

The agent module tests the local environment agent:

```bash
cd agent && go test ./...
```

**Key Test Files:**

| Package | Test File | Coverage |
|---------|-----------|----------|
| `pkg/agent` | `agent_test.go` | HTTP server, environment lifecycle |
| `pkg/agent` | `wireguard_test.go` | WireGuard peer management |

---

## E2E Tests

E2E tests verify complete CLI workflows using real Docker containers. They are **opt-in** and require specific setup.

### Requirements

1. **Docker** - Must be running and accessible
2. **Cilo Binary** - Must be built or available in PATH
3. **DNS Setup** - `sudo cilo init` must have been run (for DNS resolution)
4. **Build Tag** - Tests use the `e2e` build tag

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `CILO_E2E` | Yes | Set to `1` to enable E2E tests |
| `CILO_E2E_ENABLED` | Alternative | Set to `true` (used in `tests/e2e/workflow_test.go`) |
| `CILO_BINARY` | No | Path to cilo binary (defaults to `cilo` in PATH) |

### Running E2E Tests

#### Root-level E2E Tests (`tests/e2e/`)

These tests use a pre-built cilo binary:

```bash
# Build cilo first
cd cilo && make build

# Set binary path and run tests
export CILO_E2E_ENABLED=true
export CILO_BINARY=../bin/cilo
go test -tags e2e ./tests/e2e -v
```

**Test: `tests/e2e/workflow_test.go`**

| Test Function | Description |
|--------------|-------------|
| `TestFullWorkflow` | Complete lifecycle: create → list → stop → destroy |
| `TestParallelEnvironments` | Multiple environments running simultaneously |
| `TestDetectProject` | Project detection with and without compose files |

#### Cilo Module E2E Tests (`cilo/tests/e2e/`)

These tests run cilo via `go run`:

```bash
export CILO_E2E=1
cd cilo && go test -tags e2e ./tests/e2e -v
```

**Test: `cilo/tests/e2e/env_render_test.go`**

| Test Function | Description |
|--------------|-------------|
| `TestEnvRenderExample` | Tests env rendering using `examples/env-render` |
| `TestCiloBasicExample` | Tests basic environment creation using `examples/basic` |

**Test: `cilo/tests/e2e/shared_services_test.go`**

| Test Function | Description |
|--------------|-------------|
| `TestSharedServicesBasic` | Tests shared service lifecycle and grace periods |
| `TestSharedServicesCLIFlags` | Tests `--shared` CLI flag |
| `TestSharedServicesDoctor` | Tests `cilo doctor` for orphaned services |

**Test: `cilo/tests/e2e/sync_test.go`**

| Test Function | Description |
|--------------|-------------|
| `TestGitSync` | Tests git sync between host and environment |
| `TestNestedGitSync` | Tests sync with nested git repositories (monorepo) |

### Test Coverage

**What's Tested in E2E:**

| Feature | Test File | Status |
|---------|-----------|--------|
| Environment creation | `workflow_test.go`, `env_render_test.go` | ✅ Covered |
| Environment listing | `workflow_test.go` | ✅ Covered |
| Environment stop/down | `workflow_test.go`, `shared_services_test.go` | ✅ Covered |
| Environment destroy | `workflow_test.go` | ✅ Covered |
| Parallel environments | `workflow_test.go` | ✅ Covered |
| Project detection | `workflow_test.go` | ✅ Covered |
| Env file rendering | `env_render_test.go` | ✅ Covered |
| Shared services | `shared_services_test.go` | ✅ Covered |
| Git sync/merge | `sync_test.go` | ✅ Covered |
| DNS resolution | Indirectly via HTTP calls | ⚠️ Requires real DNS |

**What Requires Real Infrastructure:**

| Feature | Why Not E2E Tested | Testing Approach |
|---------|-------------------|------------------|
| WireGuard networking | Requires kernel module | Unit tests only |
| Cloud sync (rsync over WG) | Requires remote host | Unit tests with mocks |
| Server API | Requires PostgreSQL | Unit tests with test DB |
| Agent WireGuard | Requires kernel module | Unit tests only |
| macOS-specific DNS | Platform-specific | Manual testing |

---

## Integration Tests

Integration tests are shell scripts that test complex scenarios:

```bash
# Quick shared services verification
./tests/integration/verify-shared-services.sh

# Full shared services test suite (requires CILO_E2E=1)
CILO_E2E=1 ./tests/integration/test-shared-services.sh
```

**Integration Test Scenarios:**

1. **Shared Services** - Verify that shared services (like Elasticsearch) are properly shared across environments and cleaned up after grace periods
2. **Environment Isolation** - Verify that environments are properly isolated
3. **Resource Cleanup** - Verify that `cilo doctor` properly cleans up orphaned resources

---

## MacOS Testing

### macOS-Specific Considerations

1. **DNS Resolution** - macOS uses `/etc/resolver/test` for `.test` domains:
   ```bash
   # Check if DNS is configured
   cat /etc/resolver/test
   # Should show: nameserver 127.0.0.1 port 5354
   ```

2. **File Cloning** - E2E tests benefit from APFS copy-on-write:
   ```bash
   # Check filesystem type
   df -T ~/.cilo
   # Should show 'apfs' for optimal performance
   ```

3. **Docker Desktop** - Required for E2E tests on macOS:
   - Ensure Docker Desktop is running
   - Check with: `docker info`

4. **WireGuard** - macOS requires WireGuard tools for some tests:
   ```bash
   # Install via Homebrew
   brew install wireguard-tools
   ```

### Running Tests on macOS

```bash
# 1. Initialize cilo (one-time, requires sudo)
sudo cilo init

# 2. Build cilo
cd cilo && make build

# 3. Run unit tests
cd cilo && go test ./...

# 4. Run E2E tests
export CILO_E2E=1
cd cilo && go test -tags e2e ./tests/e2e -v
```

---

## Adding New Tests

### Unit Test Guidelines

1. **File Naming** - Use `*_test.go` suffix
2. **Package Naming** - Use `package_test` for black-box testing or `package` for white-box
3. **Test Naming** - Use `TestFunctionName_Description` pattern

**Example:**
```go
package parsers

import (
    "testing"
)

func TestComposeParser_ParseServices(t *testing.T) {
    // Arrange
    parser := &ComposeParser{}
    
    // Act
    spec, err := parser.Parse("./testdata/compose.yml")
    
    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(spec.Services) != 2 {
        t.Errorf("expected 2 services, got %d", len(spec.Services))
    }
}
```

### E2E Test Guidelines

1. **Build Tag** - Always include `//go:build e2e` at the top
2. **Environment Check** - Check for `CILO_E2E` or `CILO_E2E_ENABLED`
3. **Docker Check** - Verify Docker is available
4. **Cleanup** - Always use `defer` to clean up environments

**Template:**
```go
//go:build e2e

package e2e

import (
    "os"
    "os/exec"
    "testing"
)

func TestMyNewFeature(t *testing.T) {
    // Skip if E2E not enabled
    if os.Getenv("CILO_E2E") != "1" {
        t.Skip("set CILO_E2E=1 to run")
    }
    
    // Verify Docker
    if err := exec.Command("docker", "info").Run(); err != nil {
        t.Fatalf("docker not available: %v", err)
    }
    
    // Get module and repo roots
    moduleDir, repoRoot := moduleAndRepoRoots(t)
    
    // Setup
    envName := "e2e-my-feature"
    project := "my-project"
    
    // Cleanup first (idempotent)
    _ = runCilo(moduleDir, "destroy", envName, "--force", "--project", project)
    
    // Create environment
    if err := runCilo(moduleDir, "create", envName, "--from", fromPath, "--project", project); err != nil {
        t.Fatalf("create failed: %v", err)
    }
    
    // Defer cleanup
    defer func() {
        _ = runCilo(moduleDir, "destroy", envName, "--force", "--project", project)
    }()
    
    // Start environment
    if err := runCilo(moduleDir, "up", envName, "--project", project); err != nil {
        t.Fatalf("up failed: %v", err)
    }
    
    // Test your feature here
    // ...
}
```

### Test Data

Use the `examples/` directory for test fixtures:

| Example | Purpose |
|---------|---------|
| `examples/basic` | Simple nginx service |
| `examples/env-render` | Environment variable rendering |
| `examples/shared-services` | Shared Elasticsearch service |
| `examples/ingress-hostnames` | Custom ingress hostnames |
| `examples/custom-dns-suffix` | Custom DNS suffix |

---

## Test Commands Reference

### Using Just (Recommended)

```bash
# Run unit tests
just test

# Run unit tests with verbose output
just test-verbose

# Run E2E tests
just test-e2e

# Run integration tests
just test-integration

# Run all tests
just test-all

# Format and lint before commit
just check
```

### Using Make (Cilo module only)

```bash
cd cilo

# Run unit tests
make test

# Build and test
go test ./...
```

### Manual Go Commands

```bash
# Cilo CLI unit tests
cd cilo && go test ./...

# Server unit tests (requires PostgreSQL)
cd server && go test ./...

# Agent unit tests
cd agent && go test ./...

# E2E tests (cilo module)
cd cilo && CILO_E2E=1 go test -tags e2e ./tests/e2e -v

# E2E tests (root level, requires binary)
export CILO_E2E_ENABLED=true
export CILO_BINARY=./cilo/bin/cilo
go test -tags e2e ./tests/e2e -v

# Specific E2E test
cd cilo && CILO_E2E=1 go test -tags e2e ./tests/e2e -run TestGitSync -v
```

### Test Filtering

```bash
# Run specific test
go test -run TestFullWorkflow ./tests/e2e

# Run tests matching pattern
go test -run "TestParallel.*" ./tests/e2e

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Continuous Integration

When adding tests for CI/CD:

1. **Unit tests** should run on every commit
2. **E2E tests** should run on PRs and main branch (requires Docker)
3. **Integration tests** should run nightly or on demand

**Example CI Configuration:**

```yaml
# .github/workflows/test.yml
jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: cd cilo && go test ./...
      - run: cd server && go test ./...
      - run: cd agent && go test ./...
  
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: cd cilo && make build
      - run: export CILO_E2E=1 && cd cilo && go test -tags e2e ./tests/e2e
```

---

## Troubleshooting

### Common Issues

**"E2E tests disabled"**
```bash
# Solution: Set the environment variable
export CILO_E2E=1
# OR
export CILO_E2E_ENABLED=true
```

**"docker not available"**
```bash
# Solution: Start Docker Desktop (macOS) or Docker daemon (Linux)
docker info
```

**"cilo binary not found"**
```bash
# Solution: Build and set path
cd cilo && make build
export CILO_BINARY=../bin/cilo
```

**"connection refused" during E2E tests**
```bash
# Solution: Run cilo init first
sudo cilo init
```

**PostgreSQL connection errors in server tests**
```bash
# Solution: Start PostgreSQL or skip server tests
docker run -d -p 5432:5432 -e POSTGRES_DB=cilo_test postgres:15
export DATABASE_URL="postgres://localhost/cilo_test?sslmode=disable"
```

---

## Summary

| Test Suite | Command | Time | Requirements |
|------------|---------|------|--------------|
| Unit Tests (cilo) | `cd cilo && go test ./...` | ~1s | Go 1.24+ |
| Unit Tests (server) | `cd server && go test ./...` | ~2s | PostgreSQL |
| Unit Tests (agent) | `cd agent && go test ./...` | ~1s | Go 1.24+ |
| E2E Tests | `CILO_E2E=1 go test -tags e2e ./tests/e2e` | ~2-5min | Docker, cilo init |
| Integration | `./tests/integration/verify-shared-services.sh` | ~30s | Docker |
| All Tests | `just test-all` | ~5min | All above |

For questions or issues with testing, refer to the test files directly or open an issue on GitHub.
