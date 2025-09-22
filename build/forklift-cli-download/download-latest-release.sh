#!/bin/bash

# Script to download latest kubectl-mtv release artifacts

REPO="yaacov/kubectl-mtv"
ARTIFACTS_DIR="$(dirname "$0")/artifacts"
mkdir -p "${ARTIFACTS_DIR}"

# Get the latest release tag from GitHub API
echo "Getting latest release tag..."
TAG=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep "tag_name" | cut -d '"' -f 4)

if [ -z "$TAG" ]; then
    echo "Could not determine latest tag, exiting..."
    exit 1
fi

echo "Using tag: $TAG"

BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

echo "Downloading from: $BASE_URL"
echo ""

# Simple download function
download_file() {
    local filename="$1"
    local source_name="$2"
    echo "Downloading $filename..."
    if wget -q --show-progress -O "${ARTIFACTS_DIR}/${filename}" "${BASE_URL}/${source_name}"; then
        echo "✓ $filename"
    else
        echo "✗ Failed: $filename"
    fi
    echo ""
}

# Download each file
download_file "kubectl-mtv-linux-amd64.tar.gz" "kubectl-mtv-${TAG}-linux-amd64.tar.gz"
download_file "kubectl-mtv-linux-arm64.tar.gz" "kubectl-mtv-${TAG}-linux-arm64.tar.gz"
download_file "kubectl-mtv-darwin-amd64.tar.gz" "kubectl-mtv-${TAG}-darwin-amd64.tar.gz"
download_file "kubectl-mtv-darwin-arm64.tar.gz" "kubectl-mtv-${TAG}-darwin-arm64.tar.gz"
download_file "kubectl-mtv-windows-amd64.zip" "kubectl-mtv-${TAG}-windows-amd64.zip"
download_file "kubectl-mtv-mcp-servers-linux-amd64.tar.gz" "kubectl-mtv-mcp-servers-linux-amd64.tar.gz"

echo "Download completed!"
ls -la "${ARTIFACTS_DIR}"