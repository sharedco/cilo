#!/bin/bash
# Test runner for shared services E2E tests

set -e

echo "=== Shared Services E2E Test Suite ==="
echo ""

# Check prerequisites
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed or not in PATH"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo "❌ Docker daemon is not running"
    exit 1
fi

echo "✓ Docker is available"
echo ""

# Build cilo first
echo "Building cilo..."
SCRIPT_DIR="$(dirname "$0")"
cd "$SCRIPT_DIR/../.."
go build -o cilo ./cmd/cilo
echo "✓ Build successful"
echo ""

# Run E2E tests
echo "Running shared services E2E tests..."
echo "(This will take ~90 seconds due to grace period testing)"
echo ""

export CILO_E2E=1
export CILO_BINARY="$(pwd)/cilo"

# Run all shared services tests
go test -tags e2e -v ./test/e2e -run TestSharedServices -timeout 10m

echo ""
echo "=== Test Suite Complete ==="
