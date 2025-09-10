# CI/CD, Automation, and Tooling

## Build and Development Tools

### Makefile
The project uses a comprehensive Makefile for build automation:

```bash
# Core build commands
make build-controller           # Build the main controller
make build-operator            # Build the operator
make build-validation          # Build validation components

# Image building
make push-controller-image     # Build and push controller image
make push-operator-bundle-image # Build and push operator bundle
make push-operator-index-image  # Build and push operator index

# Deployment
make deploy-operator-index     # Deploy to cluster
make undeploy                  # Remove from cluster

# Development
make generate                  # Generate code and manifests
make manifests                 # Generate CRD manifests
make vendor                    # Update vendor dependencies
```

### Environment Variables

#### Build Configuration
```bash
REGISTRY_ORG=kubev2v          # Registry organization
REGISTRY=quay.io              # Container registry
REGISTRY_TAG=latest           # Image tag
VERSION=99.0.0                # Build version
CONTAINER_CMD=podman          # Container runtime
```

#### Image Configuration
```bash
CONTROLLER_IMAGE=quay.io/kubev2v/forklift-controller:latest
OPERATOR_IMAGE=quay.io/kubev2v/forklift-operator:latest
UI_PLUGIN_IMAGE=quay.io/kubev2v/forklift-console-plugin:latest
VALIDATION_IMAGE=quay.io/kubev2v/forklift-validation:latest
VIRT_V2V_IMAGE=quay.io/kubev2v/forklift-virt-v2v:latest
VDDK_IMAGE=                   # User-provided VDDK image
```

## Code Generation

### API Code Generation
```bash
# Generate API types and client code
make generate

# This runs:
# - controller-gen for CRD generation
# - client-gen for typed clients
# - lister-gen for listers  
# - informer-gen for informers
```

### Operator SDK
```bash
# Generate operator bundles
operator-sdk generate kustomize manifests
operator-sdk generate bundle

# Build operator
operator-sdk build
```

### Custom Resource Definitions
```bash
# Generate CRD manifests
make manifests

# Files generated:
# - operator/config/crd/bases/*.yaml
# - Includes validation schemas
# - OpenAPI specifications
```

## Testing Infrastructure

### Unit Testing
```bash
# Run unit tests
make test

# Run with coverage
make test-coverage

# Run specific package
go test ./pkg/controller/plan/...
```

### Integration Testing
```bash
# Run integration tests
make test-integration

# Test with real cluster
make test-e2e

# Test specific scenarios
make test-vmware
make test-ovirt
```

### OPA Policy Testing
```bash
# Test validation policies
make test-validation

# Run OPA tests
opa test validation/policies/

# Specific policy testing
opa test validation/policies/io/konveyor/forklift/vmware/
```

## Container Management

### Image Building
```bash
# Build all images
make images

# Build specific components
make controller-image
make operator-image
make validation-image

# Multi-arch builds
make images ARCH=amd64,arm64
```

### Image Scanning
```bash
# Security scanning (if configured)
make scan-images

# Vulnerability assessment
make security-check
```

### Registry Operations
```bash
# Push to registry
make push-images

# Tag for release
make tag-release VERSION=v2.9.0

# Promote images
make promote-images FROM_TAG=latest TO_TAG=stable
```

## Development Environment

### Local Development Setup
```bash
# Setup development environment
make dev-setup

# Install dependencies
make install-deps

# Setup git hooks
make install-hooks
```

### Cluster Deployment
```bash
# Deploy to development cluster
make deploy-dev

# Deploy with custom configuration
make deploy-dev NAMESPACE=forklift-dev

# Cleanup
make cleanup-dev
```

### Hot Reloading
```bash
# Restart controller with new changes
make restart-controller

# Update operator configuration
make update-operator
```

## Quality Assurance

### Linting
```bash
# Run Go linting
make lint

# Run with specific linters
golangci-lint run

# Fix auto-fixable issues
golangci-lint run --fix
```

### Code Formatting
```bash
# Format Go code
make fmt

# Imports organization
make imports

# Verify formatting
make verify-fmt
```

### Static Analysis
```bash
# Run static analysis
make vet

# Security analysis
make security-scan

# Dependency analysis
make deps-check
```

## Continuous Integration

### GitHub Actions (if applicable)
```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Setup Go
        uses: actions/setup-go@v2
      - name: Run tests
        run: make test
      - name: Build images
        run: make images
```

### Pre-commit Hooks
```bash
# Install pre-commit hooks
make install-hooks

# Hooks typically run:
# - go fmt
# - go vet  
# - golangci-lint
# - unit tests
# - commit message validation
```

### Release Automation
```bash
# Create release
make release VERSION=v2.9.0

# Build release artifacts
make release-artifacts

# Publish release
make publish-release
```

## Debugging and Troubleshooting

### Local Debugging
```bash
# Run controller locally
make run-controller

# Debug mode with verbose logging
make debug-controller LOG_LEVEL=debug

# Profile performance
make profile-controller
```

### Cluster Debugging
```bash
# Get controller logs
kubectl logs -n konveyor-forklift deployment/forklift-controller

# Get operator logs
kubectl logs -n konveyor-forklift deployment/forklift-operator

# Debug validation issues
kubectl get plans -o yaml
kubectl describe plan my-plan
```

### Must-Gather
```bash
# Collect debug information
make must-gather

# This collects:
# - Controller logs
# - Resource manifests
# - Cluster information
# - Migration status
```

## Monitoring and Metrics

### Prometheus Metrics
```bash
# Metrics endpoint (if exposed)
curl http://controller:8080/metrics

# Key metrics:
# - forklift_migrations_total
# - forklift_migration_duration_seconds
# - forklift_validation_errors_total
```

### Health Checks
```bash
# Controller health
curl http://controller:8081/healthz

# Readiness probe
curl http://controller:8081/readyz
```

## Documentation Generation

### API Documentation
```bash
# Generate API docs
make docs-api

# Generate from OpenAPI specs
make docs-openapi
```

### Code Documentation
```bash
# Generate Go docs
make docs-go

# Serve documentation locally
make docs-serve
```

## Development Workflow Integration

### IDE Setup (VS Code)
```json
// .vscode/settings.json
{
    "go.lintTool": "golangci-lint",
    "go.buildOnSave": "package",
    "go.testOnSave": true,
    "go.formatTool": "goimports"
}
```

### Git Configuration
```bash
# Setup git hooks for the project
make setup-git

# This configures:
# - Pre-commit hooks for formatting
# - Commit message templates
# - Branch naming conventions
```

### Dependency Management
```bash
# Update dependencies
make deps-update

# Verify dependencies
make deps-verify

# Prune unused dependencies
make deps-tidy
```

This tooling infrastructure ensures consistent builds, automated testing, and reliable deployments while supporting efficient development workflows.
