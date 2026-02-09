# Integration Tests

Integration tests for Cilo's shared services feature.

## Quick Verification

Run the quick verification script to test core functionality (~30 seconds):

```bash
./verify-shared-services.sh
```

This script:
- Creates two environments with shared elasticsearch
- Verifies they share the same container
- Tests network connections
- Validates status display
- Tests CLI flags
- Cleans up automatically

## Full E2E Test Suite

Run the comprehensive E2E test suite (~90 seconds):

```bash
export CILO_E2E=1
./test-shared-services.sh
```

This runs Go tests that verify:
- Container sharing across multiple environments
- Reference counting
- Grace period behavior (60 seconds)
- Network attachment/detachment
- Doctor command detection and fixing
- CLI flag functionality

## Prerequisites

- Docker installed and running
- Go 1.21 or later
- Cilo examples directory (`examples/shared-services/`)

## Manual Testing

You can also test manually:

```bash
# Build cilo
cd ../../cilo
go build -o cilo

# Create test environments
./cilo create test1 --from ../examples/shared-services --project test-shared
./cilo up test1 --project test-shared

./cilo create test2 --from ../examples/shared-services --project test-shared
./cilo up test2 --project test-shared

# Verify only one elasticsearch container exists
docker ps | grep elasticsearch

# Clean up
./cilo destroy test1 --force --project test-shared
./cilo destroy test2 --force --project test-shared
```
