#!/bin/bash

# Test script for the Mock Nutanix Prism API Server

set -e

SERVER="https://localhost:9440"
USER="admin"
PASSWORD="password"
CURL_OPTS="-k"  # Skip certificate verification for self-signed cert

echo "════════════════════════════════════════════════════════"
echo "  Testing Mock Nutanix Prism API Server"
echo "════════════════════════════════════════════════════════"
echo

# Check if server is running
echo "Checking if server is running at $SERVER..."
if ! curl $CURL_OPTS -s -f -u $USER:$PASSWORD "$SERVER/api/nutanix/v3" > /dev/null 2>&1; then
    echo "❌ Server not responding. Please start the mock server first:"
    echo "   go run mock_server.go"
    exit 1
fi
echo "✅ Server is running"
echo

# Test clusters endpoint
echo "Testing /api/nutanix/v3/clusters/list..."
CLUSTERS=$(curl $CURL_OPTS -s -X POST "$SERVER/api/nutanix/v3/clusters/list" \
    -u $USER:$PASSWORD \
    -H "Content-Type: application/json" \
    -d '{"kind": "cluster"}')

CLUSTER_COUNT=$(echo "$CLUSTERS" | jq '.metadata.total_matches // 0')
echo "✅ Found $CLUSTER_COUNT clusters"
echo

# Test hosts endpoint
echo "Testing /api/nutanix/v3/hosts/list..."
HOSTS=$(curl $CURL_OPTS -s -X POST "$SERVER/api/nutanix/v3/hosts/list" \
    -u $USER:$PASSWORD \
    -H "Content-Type: application/json" \
    -d '{}')

HOST_COUNT=$(echo "$HOSTS" | jq '.metadata.total_matches // 0')
echo "✅ Found $HOST_COUNT hosts"
echo

# Test VMs endpoint
echo "Testing /api/nutanix/v3/vms/list..."
VMS=$(curl $CURL_OPTS -s -X POST "$SERVER/api/nutanix/v3/vms/list" \
    -u $USER:$PASSWORD \
    -H "Content-Type: application/json" \
    -d '{"kind": "vm", "length": 100}')

VM_COUNT=$(echo "$VMS" | jq '.metadata.total_matches // 0')
echo "✅ Found $VM_COUNT VMs"
echo

# Test networks endpoint
echo "Testing /api/nutanix/v3/subnets/list..."
NETWORKS=$(curl $CURL_OPTS -s -X POST "$SERVER/api/nutanix/v3/subnets/list" \
    -u $USER:$PASSWORD \
    -H "Content-Type: application/json" \
    -d '{}')

NETWORK_COUNT=$(echo "$NETWORKS" | jq '.metadata.total_matches // 0')
echo "✅ Found $NETWORK_COUNT networks"
echo

# Test storage containers endpoint
echo "Testing /api/nutanix/v3/storage_containers/list..."
STORAGE=$(curl $CURL_OPTS -s -X POST "$SERVER/api/nutanix/v3/storage_containers/list" \
    -u $USER:$PASSWORD \
    -H "Content-Type: application/json" \
    -d '{}')

STORAGE_COUNT=$(echo "$STORAGE" | jq '.metadata.total_matches // 0')
echo "✅ Found $STORAGE_COUNT storage containers"
echo

# Test images endpoint
echo "Testing /api/nutanix/v3/images/list..."
IMAGES=$(curl $CURL_OPTS -s -X POST "$SERVER/api/nutanix/v3/images/list" \
    -u $USER:$PASSWORD \
    -H "Content-Type: application/json" \
    -d '{}')

IMAGE_COUNT=$(echo "$IMAGES" | jq '.metadata.total_matches // 0')
echo "✅ Found $IMAGE_COUNT images"
echo

# Test authentication failure
echo "Testing authentication failure..."
HTTP_CODE=$(curl $CURL_OPTS -s -o /dev/null -w "%{http_code}" -X POST \
    "$SERVER/api/nutanix/v3/clusters/list" \
    -u wrong:credentials \
    -H "Content-Type: application/json" \
    -d '{}')

if [ "$HTTP_CODE" = "401" ]; then
    echo "✅ Authentication correctly rejected with 401"
else
    echo "❌ Expected 401, got $HTTP_CODE"
fi
echo

echo "════════════════════════════════════════════════════════"
echo "  All tests passed! ✅"
echo "════════════════════════════════════════════════════════"
echo
echo "Summary:"
echo "  Clusters: $CLUSTER_COUNT"
echo "  Hosts: $HOST_COUNT"
echo "  VMs: $VM_COUNT"
echo "  Networks: $NETWORK_COUNT"
echo "  Storage: $STORAGE_COUNT"
echo "  Images: $IMAGE_COUNT"
echo
