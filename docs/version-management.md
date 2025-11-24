# Version Management in Forklift

This document describes the unified version management strategy for the Forklift project.

## Overview

Previously, version information was scattered across multiple files with unclear purposes. The new centralized approach provides:

- **Single source of truth** for version constants
- **Clear documentation** of what each version represents
- **Consistent usage** across all components
- **Build-time flexibility** for main application versions

## Version Types

### 1. Application Version (`ForkliftVersion`)
- **Purpose**: Main Forklift application version
- **Set by**: ldflags during build process
- **Example**: `2.10.0`
- **Usage**: Overall product versioning, releases, compatibility

### 2. Component Versions (Constants)
- **Purpose**: Version specific components within Forklift
- **Set by**: Constants in code (changed when component changes)
- **Examples**:
  - `VibPackageVersion`: ESXi VIB package version
  - `APIVersion`: Forklift API schema version

### 3. Build Information
- **Purpose**: Build-time metadata for debugging and support
- **Set by**: ldflags during build process
- **Examples**: git commit, build date, Go version, platform

## Usage

### Importing
```go
import "github.com/kubev2v/forklift/pkg/version"
```

### Accessing Versions
```go
// Main application version
fmt.Println("Forklift version:", version.ForkliftVersion)

// Component versions
fmt.Println("VIB version:", version.VibPackageVersion)

// Complete version info
version.PrintVersionInfo()

// Programmatic access
versionInfo := version.GetVersionInfo()
buildInfo := version.GetBuildInfo()
```

### Build Integration
```bash
# Example ldflags for build
go build -ldflags "
  -X github.com/kubev2v/forklift/pkg/version.ForkliftVersion=2.10.0
  -X github.com/kubev2v/forklift/pkg/version.GitCommit=abc123
  -X github.com/kubev2v/forklift/pkg/version.BuildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
"
```

## Migration Guide

### Before (Multiple scattered versions)
```go
// In various files:
var VibVersion = "x.x.x"             // cmd/.../vib.go
var version = "unknown"              // cmd/.../main.go
```

### After (Centralized)
```go
// In pkg/version/version.go:
const VibPackageVersion = "1.0.0"
var ForkliftVersion = "unknown"      // Set by ldflags
```

## Best Practices

### When to Update Versions

1. **VibPackageVersion**: Increment when the VIB package content/behavior changes  
2. **APIVersion**: Update when API schema changes incompatibly
3. **ForkliftVersion**: Set automatically by build/release process

**Note**: The secure vmkfstools wrapper script no longer uses versioning - it is always uploaded fresh to ensure the latest version is used.

### Version Format
- Use [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH)
- Component versions should be independent of application version
- Use consistent format across all version strings

### Compatibility
- Existing build scripts continue to work via compatibility variables
- Gradual migration approach minimizes disruption
- Old version access patterns are redirected to centralized system

## Examples

### Version Output
```
Forklift Version Information:
============================
Version:      2.10.0
Git Commit:   abc123def
Build Date:   2024-01-15T10:30:00Z
Go Version:   go1.21.0
Platform:     linux/amd64

Component Versions:
-------------------
Forklift        : 2.10.0 (Main application version)
VIBPackage      : 1.0.0 (ESXi VIB package version)
API             : v1beta1 (Forklift API schema version)
```

### Integration in Applications
```go
// In main functions:
if showVersion {
    version.PrintVersionInfo()
    os.Exit(0)
}

// In logging:
klog.Infof("Starting Forklift %s (build %s)", 
    version.ForkliftVersion, version.GitCommit)
```

## Migration Status

- ✅ Created centralized version package
- ✅ Removed SecureScriptVersion (always upload script)
- ✅ Updated VIB version handling  
- ✅ Updated vsphere-copy-offload-populator
- ✅ Updated validation.go references
- 🔄 Integration with build scripts (in progress)
- ⏳ Migration of other components (future)