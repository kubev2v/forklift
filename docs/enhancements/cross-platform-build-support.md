---
title: cross-platform-build-support
authors:
  - "@yzamir"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2025-10-25
last-updated: 2025-10-25
status: implemented
---

# Cross-Platform Build Support

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [x] User-facing documentation is created

## Summary

The Makefile has been enhanced to support building Forklift container images for multiple architectures (AMD64, ARM64) with automatic architecture tagging and multi-architecture manifest creation. This enables developers to build and deploy Forklift on ARM-based systems such as Apple Silicon Macs and ARM servers, while maintaining compatibility with existing AMD64 infrastructure.

## Motivation

The Makefile was hardcoded to build for `linux/amd64` architecture only, which created issues for:
- Developers using ARM-based machines (Apple Silicon Macs, ARM servers) who could not easily build native images
- Running amd64 containers on ARM machines required emulation, resulting in slow performance
- Testing on non-x86 architectures required manual workarounds
- Deploying to ARM-based Kubernetes clusters was not straightforward

### Goals

- Enable developers to build container images for their native architecture (ARM64, AMD64)
- Support explicit platform selection for cross-platform builds
- Automatically tag images with architecture suffix for easy identification
- Support creation of multi-architecture manifests compatible with both Podman and Docker
- Maintain backward compatibility with existing build workflows

### Non-Goals

- Build multi-arch virt-v2v container image
- Build multi-arch Ovirt populator image
- Downstream multi-arch support
- Support architectures other than AMD64 and ARM64

## Proposal

### Platform Selection

Add `PLATFORM` variable to the Makefile to specify target architecture:

```bash
# Build for AMD64 (default)
make build-controller-image PLATFORM=linux/amd64

# Build for ARM64
make build-controller-image PLATFORM=linux/arm64
```

### Automatic Architecture Tagging

Images are automatically tagged with architecture suffix based on the `PLATFORM` variable:

```makefile
PLATFORM_ARCH ?= $(shell echo $(PLATFORM) | cut -d'/' -f2)
PLATFORM_SUFFIX := -$(PLATFORM_ARCH)
```

Examples:
- `PLATFORM=linux/amd64` → `forklift-controller:devel-amd64`
- `PLATFORM=linux/arm64` → `forklift-controller:devel-arm64`

### Multi-Architecture Manifests

After building and pushing images for multiple architectures, users can create a multi-arch manifest that automatically pulls the correct architecture:

```bash
make push-controller-image-manifest
```

This creates a manifest (e.g., `forklift-controller:devel`) that points to both `-amd64` and `-arm64` images.

### Multi-Architecture Build Workflow

Complete example workflow:

```bash
# Step 1: Build and push AMD64 images
make build-controller-image PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0
make push-controller-image PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0

# Step 2: Build and push ARM64 images
make build-controller-image PLATFORM=linux/arm64 REGISTRY_TAG=v2.11.0
make push-controller-image PLATFORM=linux/arm64 REGISTRY_TAG=v2.11.0

# Step 3: Create and push multi-arch manifest
make push-controller-image-manifest REGISTRY_TAG=v2.11.0
```

Result:
- `forklift-controller:v2.11.0-amd64` (AMD64 image)
- `forklift-controller:v2.11.0-arm64` (ARM64 image)
- `forklift-controller:v2.11.0` (multi-arch manifest pointing to both)

**Hint**: Manifests point to architecture-specific images by their SHA256 hash digest, not by tag. This means that if you rebuild and push an architecture-specific image (e.g., `forklift-controller:v2.6.0-amd64`) with the same tag, the manifest will still point to the old image hash. You must repush the manifest to update it with the new hash. Always remember to run the `push-*-image-manifest` target after pushing updated architecture-specific images.

Bulk operations are also supported:
```bash
make build-all-images PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0
make push-all-images PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0
make push-all-images-manifest REGISTRY_TAG=v2.11.0
```

### Bundle and Index Image Behavior

The operator bundle and index images have two build modes:

#### Default Single-Architecture Mode

By default, bundles reference **platform-specific images** for development workflows:

```bash
# Build single-arch bundle (default targets)
make build-operator-bundle-image PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0
make push-operator-bundle-image REGISTRY_TAG=v2.11.0
# Bundle will reference: forklift-controller:v2.11.0-amd64
```

This is ideal for development where you're building and testing on a single architecture without needing to create multi-arch manifests.

#### Multi-Architecture Mode

For production multi-arch deployments, use the `-multiarch` targets to make the bundle reference **multi-arch manifest names**:

```bash
# Complete multi-arch workflow
# Step 1: Build and push images for both architectures
make build-all-images PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0
make push-all-images PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0

make build-all-images PLATFORM=linux/arm64 REGISTRY_TAG=v2.11.0
make push-all-images PLATFORM=linux/arm64 REGISTRY_TAG=v2.11.0

# Step 2: Create multi-arch manifests
make push-all-images-manifest REGISTRY_TAG=v2.11.0

# Step 3: Build bundle that references multi-arch manifests (using -multiarch targets)
make build-operator-bundle-image-multiarch REGISTRY_TAG=v2.11.0
make push-operator-bundle-image-multiarch REGISTRY_TAG=v2.11.0
# Bundle will reference: forklift-controller:v2.11.0 (no platform suffix)

# Step 4: Build and push the multi-arch index
make build-operator-index-image-multiarch REGISTRY_TAG=v2.11.0
make push-operator-index-image-multiarch REGISTRY_TAG=v2.11.0
```

When deployed, the container runtime automatically pulls the correct architecture from the multi-arch manifests.

#### Available Multi-Arch Targets

- `build-operator-bundle-image-multiarch` - Build bundle referencing multi-arch manifests
- `push-operator-bundle-image-multiarch` - Push multi-arch bundle
- `build-operator-index-image-multiarch` - Build index referencing multi-arch bundle
- `push-operator-index-image-multiarch` - Push multi-arch index
- `deploy-operator-index-multiarch` - Deploy multi-arch index to cluster

#### Deployment Targets

The deployment behavior also differs between single-arch and multi-arch modes:

**Single-arch deployment (default):**
```bash
# Deploy platform-specific index to cluster
make deploy-operator-index PLATFORM=linux/amd64 REGISTRY_TAG=v2.11.0
# Deploys: forklift-operator-index:v2.11.0-amd64
```

**Multi-arch deployment:**
```bash
# Deploy multi-arch index to cluster
make deploy-operator-index-multiarch REGISTRY_TAG=v2.11.0
# Deploys: forklift-operator-index:v2.11.0 (no platform suffix)
```

### Implementation Details/Notes/Constraints

#### Architecture Limitations

##### virt-v2v

**Important**: The `forklift-virt-v2v` component can **only** be built for `linux/amd64` due to:
- **virt-v2v tool**: Only available for AMD64 architecture
- **libvirt**: CGO package only packaged for AMD64

The virt-v2v build will be automatically skipped on non-AMD64 platforms with a notice message.

The virt-v2v manifest target creates a single-architecture manifest with AMD64 only.

##### ovirt-populator

**Important**: The `ovirt-populator` component can **only** be built for `linux/amd64` due to:
- **python3-ovirt-engine-sdk4**: Only available for AMD64 architecture
- **ovirt-imageio-client**: Only available for AMD64 architecture

The ovirt-populator build will be automatically skipped on non-AMD64 platforms with a notice message.

The ovirt-populator manifest target creates a single-architecture manifest with AMD64 only.

#### Available Manifest Targets

Individual targets for each component:
- `push-controller-image-manifest`
- `push-api-image-manifest`
- `push-validation-image-manifest`
- `push-operator-image-manifest`
- `push-virt-v2v-image-manifest` (AMD64 only)
- `push-populator-controller-image-manifest`
- `push-ovirt-populator-image-manifest` (AMD64 only)
- `push-openstack-populator-image-manifest`
- `push-vsphere-xcopy-volume-populator-image-manifest`
- `push-ova-provider-server-image-manifest`
- `push-cli-download-image-manifest`
- `push-forklift-proxy-image-manifest`
- `push-operator-bundle-image-manifest`
- `push-operator-index-image-manifest`

Bulk target:
- `push-all-images-manifest`
