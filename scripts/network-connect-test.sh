#!/bin/bash
# Phase 0 Spike: Validate Docker network connect behavior
# Tests that a container can be connected to multiple networks and resolved by service name

set -e

echo "🔬 Starting Docker networking spike test..."
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    docker rm -f spike_test_container 2>/dev/null || true
    docker network rm spike_network_1 spike_network_2 2>/dev/null || true
    echo "✅ Cleanup complete"
}

# Set up cleanup trap
trap cleanup EXIT

# Step 1: Create two test networks
echo "1️⃣  Creating test networks..."
docker network create --subnet 172.30.1.0/24 spike_network_1
docker network create --subnet 172.30.2.0/24 spike_network_2
echo "   ✓ Networks created"
echo ""

# Step 2: Start an Elasticsearch container on network 1
echo "2️⃣  Starting Elasticsearch container on spike_network_1..."
docker run -d \
    --name spike_test_container \
    --network spike_network_1 \
    -e "discovery.type=single-node" \
    -e "xpack.security.enabled=false" \
    docker.elastic.co/elasticsearch/elasticsearch:8.11.0

# Wait for container to be ready
echo "   ⏳ Waiting for container to be ready..."
for i in {1..30}; do
    if docker exec spike_test_container curl -s http://localhost:9200 >/dev/null 2>&1; then
        echo "   ✓ Container started and healthy"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "   ✗ Container failed to become healthy"
        exit 1
    fi
    sleep 1
done
echo ""

# Step 3: Connect it to network 2 WITH alias
echo "3️⃣  Connecting container to spike_network_2 with alias 'elasticsearch'..."
docker network connect --alias elasticsearch spike_network_2 spike_test_container
echo "   ✓ Network connected with alias"
echo ""

# Step 4: Verify DNS resolution from network 1
echo "4️⃣  Testing DNS resolution from spike_network_1..."
docker run --rm --network spike_network_1 alpine sh -c "
    apk add --no-cache curl > /dev/null 2>&1
    echo '   Testing by container name: spike_test_container'
    if curl -s -o /dev/null -w '%{http_code}' http://spike_test_container:9200 | grep -q 200; then
        echo '   ✓ Container name resolution works'
    else
        echo '   ✗ Container name resolution failed'
        exit 1
    fi
"
echo ""

# Step 5: Verify DNS resolution from network 2 (via alias)
echo "5️⃣  Testing DNS resolution from spike_network_2..."
docker run --rm --network spike_network_2 alpine sh -c "
    apk add --no-cache curl > /dev/null 2>&1
    echo '   Testing by alias: elasticsearch'
    if curl -s -o /dev/null -w '%{http_code}' http://elasticsearch:9200 | grep -q 200; then
        echo '   ✓ Alias resolution works'
    else
        echo '   ✗ Alias resolution failed'
        exit 1
    fi
"
echo ""

# Step 6: Test disconnection
echo "6️⃣  Testing disconnection from spike_network_2..."
docker network disconnect spike_network_2 spike_test_container
echo "   ✓ Network disconnected"
echo ""

# Step 7: Verify alias is no longer reachable
echo "7️⃣  Verifying alias is no longer reachable..."
if docker run --rm --network spike_network_2 alpine sh -c "
    apk add --no-cache curl > /dev/null 2>&1
    curl -s -o /dev/null -w '%{http_code}' --connect-timeout 2 http://elasticsearch:9200
" 2>/dev/null | grep -q 200; then
    echo "   ✗ Alias still reachable (should have failed)"
    exit 1
else
    echo "   ✓ Alias no longer reachable (expected)"
fi
echo ""

# Step 8: Get container IPs from both networks
echo "8️⃣  Testing IP address retrieval..."
docker network connect spike_network_2 spike_test_container
IP_NET1=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{if eq .NetworkID "'$(docker network inspect -f '{{.Id}}' spike_network_1)'"}}{{.IPAddress}}{{end}}{{end}}' spike_test_container)
IP_NET2=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{if eq .NetworkID "'$(docker network inspect -f '{{.Id}}' spike_network_2)'"}}{{.IPAddress}}{{end}}{{end}}' spike_test_container)
echo "   Network 1 IP: $IP_NET1"
echo "   Network 2 IP: $IP_NET2"
if [ -n "$IP_NET1" ] && [ -n "$IP_NET2" ]; then
    echo "   ✓ Successfully retrieved IPs from both networks"
else
    echo "   ✗ Failed to retrieve IPs"
    exit 1
fi
echo ""

echo "✅ All spike tests passed!"
echo ""
echo "📋 Summary:"
echo "   • docker network connect --alias works reliably"
echo "   • Containers can resolve shared services by alias"
echo "   • Disconnection cleanly removes DNS resolution"
echo "   • Multiple network IPs can be retrieved"
echo ""
echo "✨ Architecture decision validated: Option B (direct attachment) is viable"

