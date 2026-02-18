# Cilo Test Suite

This directory contains all tests for the Cilo project, organized by test type.

## Directory Structure

```
test/
├── README.md                 # This file - testing overview
├── e2e/                     # End-to-end tests
│   ├── README.md            # E2E testing guide
│   ├── workflow_test.go     # Complete workflow tests
│   ├── env_render_test.go   # Environment rendering tests
│   ├── shared_services_test.go  # Shared services tests
│   └── sync_test.go         # Git sync tests
├── integration/             # Integration tests
│   ├── README.md            # Integration testing guide
│   ├── test-shared-services.sh    # Full E2E test suite
│   └── verify-shared-services.sh  # Quick verification script
└── testdata/                # Test fixtures and data
    └── .gitkeep             # Placeholder for future fixtures
```

## Test Types

### Unit Tests

Unit tests are co-located with source code in `cilo/internal/*/*_test.go`. These test individual functions and components in isolation.

Run unit tests:
```bash
cd cilo
go test ./...
```

### E2E Tests

E2E tests verify the complete Cilo workflow end-to-end. They require:
- Docker Engine running
- Built cilo binary
- Configured DNS (run `sudo cilo init` once)

Run E2E tests:
```bash
# Build cilo
cd cilo && go build -o cilo

# Run all E2E tests
export CILO_E2E_ENABLED=true
export CILO_BINARY=./cilo/cilo
go test -tags e2e ./test/e2e/...
```

### Integration Tests

Integration tests verify multi-component interactions, particularly the shared services feature.

Quick verification (~30 seconds):
```bash
./test/integration/verify-shared-services.sh
```

Full E2E test suite (~90 seconds):
```bash
export CILO_E2E=1
./test/integration/test-shared-services.sh
```

## Test Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `CILO_E2E_ENABLED` | Enable E2E tests (skips otherwise) | `true` |
| `CILO_BINARY` | Path to cilo binary for E2E tests | `./cilo/cilo` |
| `CILO_E2E` | Alternative flag for some E2E tests | `1` |

## Writing Tests

### Unit Tests

Place unit tests next to the code they test:
```go
// cilo/internal/mypackage/mypackage_test.go
package mypackage

import "testing"

func TestMyFunction(t *testing.T) {
    result := MyFunction()
    if result != expected {
        t.Errorf("MyFunction() = %v, want %v", result, expected)
    }
}
```

### E2E Tests

Place E2E tests in `test/e2e/` with the `e2e` build tag:
```go
// test/e2e/myfeature_test.go
//go:build e2e
// +build e2e

package e2e

import (
    "os"
    "os/exec"
    "testing"
)

func TestMyFeature(t *testing.T) {
    if os.Getenv("CILO_E2E_ENABLED") != "true" {
        t.Skip("E2E tests disabled")
    }
    
    cmd := exec.Command("cilo", "up", "test-env")
    // ... test code
}
```

## License

All test files are licensed under the MIT License. See `LICENSES/MIT.txt` for details.
