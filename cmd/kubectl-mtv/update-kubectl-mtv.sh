#!/bin/bash

set -e

# Change to the kubectl-mtv directory
cd "$(dirname "$0")"

echo "Fetching latest kubectl-mtv release from GitHub..."

# Get the latest release version from GitHub API
LATEST_VERSION=$(curl -s https://api.github.com/repos/yaacov/kubectl-mtv/releases/latest | \
    grep '"tag_name":' | \
    sed -E 's/.*"([^"]+)".*/\1/')

if [[ -z "$LATEST_VERSION" ]]; then
    echo "Error: Failed to fetch latest version from GitHub" >&2
    exit 1
fi

echo "Latest version: ${LATEST_VERSION}"

# Get current version from go.mod
CURRENT_VERSION=$(grep "github.com/yaacov/kubectl-mtv" go.mod | sed -E 's/.*v([0-9]+\.[0-9]+\.[0-9]+).*/v\1/')

if [[ "$CURRENT_VERSION" == "$LATEST_VERSION" ]]; then
    echo "Already up to date! Current version: ${CURRENT_VERSION}"
    exit 0
fi

echo "Updating from ${CURRENT_VERSION} to ${LATEST_VERSION}..."

# Update go.mod with the latest version (portable across macOS and Linux)
sed -i.bak "s|github.com/yaacov/kubectl-mtv v[0-9]*\.[0-9]*\.[0-9]*|github.com/yaacov/kubectl-mtv ${LATEST_VERSION}|" go.mod
rm -f go.mod.bak

echo "Running go mod tidy..."
go mod tidy

echo "Running go mod vendor..."
go mod vendor

echo "Successfully updated kubectl-mtv to ${LATEST_VERSION}!"
echo "Updated files:"
echo "  - go.mod"
echo "  - go.sum"
echo "  - vendor/ directory"
