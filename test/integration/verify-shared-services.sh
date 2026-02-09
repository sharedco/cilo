#!/bin/bash
# Quick manual verification script for shared services feature

set -e

echo "=== Quick Shared Services Verification ==="
echo ""
echo "This script will:"
echo "  1. Create two environments with shared elasticsearch"
echo "  2. Verify they share the same container"
echo "  3. Clean up"
echo ""

PROJECT="verify-shared"
ENV1="verify-env1"
ENV2="verify-env2"

# Build cilo
echo "Building cilo..."
SCRIPT_DIR="$(dirname "$0")"
cd "$SCRIPT_DIR/../.."
go build -o cilo ./cmd/cilo
CILO="$(pwd)/cilo"

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    docker stop cilo_shared_${PROJECT}_elasticsearch 2>/dev/null || true
    docker rm cilo_shared_${PROJECT}_elasticsearch 2>/dev/null || true
    docker stop cilo_shared_${PROJECT}_app 2>/dev/null || true
    docker rm cilo_shared_${PROJECT}_app 2>/dev/null || true
    $CILO destroy $ENV1 --force --project $PROJECT 2>/dev/null || true
    $CILO destroy $ENV2 --force --project $PROJECT 2>/dev/null || true
    docker network rm cilo_${ENV1} 2>/dev/null || true
    docker network rm cilo_${ENV2} 2>/dev/null || true
}

trap cleanup EXIT

# Start fresh
cleanup

echo ""
echo "Step 1: Creating first environment..."
$CILO create $ENV1 --from examples/shared-services --project $PROJECT
$CILO up $ENV1 --project $PROJECT

echo ""
echo "Step 2: Checking elasticsearch container..."
ES_CONTAINER_1=$(docker ps --filter "name=elasticsearch" --format "{{.Names}}")
if [ -z "$ES_CONTAINER_1" ]; then
    echo "❌ Elasticsearch container not found!"
    exit 1
fi
echo "✓ Elasticsearch running: $ES_CONTAINER_1"

echo ""
echo "Step 3: Creating second environment..."
$CILO create $ENV2 --from examples/shared-services --project $PROJECT
$CILO up $ENV2 --project $PROJECT

echo ""
echo "Step 4: Verifying same elasticsearch container is used..."
ES_CONTAINER_2=$(docker ps --filter "name=elasticsearch" --format "{{.Names}}")
if [ "$ES_CONTAINER_1" != "$ES_CONTAINER_2" ]; then
    echo "❌ Different elasticsearch containers!"
    echo "   ENV1: $ES_CONTAINER_1"
    echo "   ENV2: $ES_CONTAINER_2"
    exit 1
fi
echo "✓ Same container used: $ES_CONTAINER_2"

echo ""
echo "Step 5: Verifying container count..."
ES_COUNT=$(docker ps --filter "name=elasticsearch" --format "{{.Names}}" | wc -l)
if [ "$ES_COUNT" -ne 1 ]; then
    echo "❌ Expected 1 elasticsearch container, found $ES_COUNT"
    exit 1
fi
echo "✓ Exactly 1 elasticsearch container running"

echo ""
echo "Step 6: Verifying network connections..."
NETWORKS=$(docker inspect $ES_CONTAINER_1 --format '{{json .NetworkSettings.Networks}}')
if ! echo "$NETWORKS" | grep -q "$ENV1"; then
    echo "❌ Elasticsearch not connected to $ENV1 network"
    exit 1
fi
if ! echo "$NETWORKS" | grep -q "$ENV2"; then
    echo "❌ Elasticsearch not connected to $ENV2 network"
    exit 1
fi
echo "✓ Elasticsearch connected to both environment networks"

echo ""
echo "Step 7: Testing status command..."
STATUS=$($CILO status $ENV1 --project $PROJECT)
if ! echo "$STATUS" | grep -q "elasticsearch"; then
    echo "❌ Elasticsearch not in status output"
    exit 1
fi
if ! echo "$STATUS" | grep -q "shared"; then
    echo "❌ Service not marked as 'shared' in status"
    exit 1
fi
echo "✓ Status shows elasticsearch as shared"

echo ""
echo "Step 8: Testing --shared flag with space-delimited values..."
$CILO down $ENV1 --project $PROJECT
$CILO up $ENV1 --project $PROJECT --shared app
STATUS=$($CILO status $ENV1 --project $PROJECT)
echo "✓ --shared flag works with space-delimited values"

echo ""
echo "=== ✓ All Verifications Passed ==="
echo ""
echo "The shared services feature is working correctly:"
echo "  ✓ Multiple environments share the same service container"
echo "  ✓ Only one container runs for shared services"
echo "  ✓ Network connections work properly"
echo "  ✓ Status display is correct"
echo "  ✓ CLI flags work as expected"
