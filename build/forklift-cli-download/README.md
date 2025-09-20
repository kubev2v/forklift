# Forklift CLI Download Service

This directory contains the build configuration and artifacts for the Forklift CLI download service.

## Overview

The CLI download service provides a simple HTTP server that serves kubectl-mtv CLI binaries and MCP (Model Context Protocol) servers for multiple platforms through OpenShift routes and ConsoleCLIDownload resources.

## Directory Structure

```
build/forklift-cli-download/
├── artifacts/              # CLI binaries and archives
├── Containerfile           # Container build configuration
├── Containerfile-downstream # Downstream container build configuration
├── download-latest-release.sh # Script to download latest releases
└── README.md               # This file
```

## Downloading Latest Release

To download the latest kubectl-mtv release artifacts from GitHub:

```bash
# Simply run the script
./download-latest-release.sh
```

### What Gets Downloaded

The script downloads the following files from the latest kubectl-mtv GitHub release:

- `kubectl-mtv-linux-amd64.tar.gz` - Linux x86_64 binary
- `kubectl-mtv-linux-arm64.tar.gz` - Linux ARM64 binary
- `kubectl-mtv-darwin-amd64.tar.gz` - macOS x86_64 binary
- `kubectl-mtv-darwin-arm64.tar.gz` - macOS ARM64 binary
- `kubectl-mtv-windows-amd64.zip` - Windows x86_64 binary
- `kubectl-mtv-mcp-servers-linux-amd64.tar.gz` - MCP servers for Linux

The script automatically renames the files to remove version numbers, ensuring they match the expected filenames referenced in the ConsoleCLIDownload resources.

## Integration with Operator

The CLI download service is integrated into the Forklift operator as an optional feature:

- **Feature flag**: `feature_cli_download: true` (enabled by default)
- **Deployment**: Only on OpenShift clusters (`not k8s_cluster|bool`)
- **Resources created**:
  - Deployment (`forklift-cli-download`)
  - Service (port 8080)
  - Route (HTTPS with edge termination)
  - ConsoleCLIDownload resources for kubectl-mtv and MCP

## ConsoleCLIDownload Integration

When deployed on OpenShift, the service creates ConsoleCLIDownload resources that appear in the OpenShift console, allowing users to easily download kubectl-mtv CLI tools directly from the web console.

## Environment Variables

The operator uses these environment variables for container image configuration:

- `CLI_DOWNLOAD_IMAGE` - Primary image reference
