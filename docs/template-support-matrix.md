# Template Support Matrix

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 9, 2026 |
| **Applies To** | Forklift v2.7+ |
| **Maintainer** | Forklift Team |
| **Update Policy** | Update when template features are added or provider support changes |

This document provides an overview of template support across different source providers in Forklift. Templates allow users to customize naming conventions for PVCs, volumes, and network interfaces during VM migration.

---

## Template Syntax

Forklift templates use **Go's `text/template` syntax** with additional helper functions curated from the Sprig library.

### Go Template Basics

For complete Go template syntax, see the official documentation:
- [Go text/template package](https://pkg.go.dev/text/template)

**Basic syntax:**
```yaml
# Variable substitution
"{{.VariableName}}"

# Conditionals
"{{if .Condition}}value{{else}}other{{end}}"

# Comparisons
"{{if eq .A .B}}equal{{end}}"

# Pipelines (chaining functions)
"{{.Name | lower | trunc 10}}"
```

### Available Functions

Forklift provides a curated set of template functions. For the authoritative list, see [templateutil](../pkg/templateutil/README.md).

---

## Overview

Forklift supports three types of naming templates and a naming behavior modifier:

1. **PVCNameTemplate** - Controls the naming of PersistentVolumeClaims created during migration
2. **VolumeNameTemplate** - Controls the naming of volume interfaces in the target VM
3. **NetworkNameTemplate** - Controls the naming of network interfaces in the target VM
4. **PVCNameTemplateUseGenerateName** - Controls whether the PVC name is used as a `generateName` prefix (with random suffix) or as the exact name

Templates can be specified at two levels:
- **Plan level** - Applies to all VMs in the migration plan
- **VM level** - Overrides the plan-level template for a specific VM (except PVCNameTemplateUseGenerateName which is plan-level only)

## Support Matrix

| Feature | VMware (vSphere) | OpenShift | oVirt | OpenStack | OVA |
|---------|------------------|-----------|-------|-----------|-----|
| **PVCNameTemplate** | Full | Full | No | No | No |
| **PVCNameTemplateUseGenerateName** | Full | Ignored | No | No | No |
| **VolumeNameTemplate** | Full | No | No | No | No |
| **NetworkNameTemplate** | Full | No | No | No | No |

### Legend

- **Full** - Feature is fully supported
- **No** - Feature is not supported
- **Ignored** - Field exists but is ignored by the provider

---

## Template Data Structures and Defaults

This section documents the data structures passed to templates and default behaviors for providers with template support.

### VMware (vSphere)

VMware has full support for all template features.

#### PVCNameTemplate

**Data Structure:** `VSpherePVCNameTemplateData`

| Variable | Type | Description | K8s Compliant |
|----------|------|-------------|---------------|
| `.VmName` | string | Name of the VM in the source cluster (original source name) | No, may need `lower` |
| `.TargetVmName` | string | Final VM name in the target cluster (DNS1123 normalized) | Yes |
| `.PlanName` | string | Name of the migration plan | Yes |
| `.DiskIndex` | int | Initial volume index of the disk (0-based) | Yes |
| `.WinDriveLetter` | string | Windows drive letter (lowercase, e.g., "c"). Requires guest agent | Yes |
| `.RootDiskIndex` | int | Index of the root/boot disk | Yes |
| `.Shared` | bool | `true` if the volume is shared by multiple VMs | Yes |
| `.FileName` | string | Name of the vmdk file in the source provider (includes .vmdk suffix) | No, may need `lower` |

**Default Template:** `{{trunc 4 .PlanName}}-{{trunc 4 .VmName}}-disk-{{.DiskIndex}}`

**Examples:**
```yaml
# Truncated plan and target VM names with disk index (recommended for uniqueness)
# Note: .PlanName and .TargetVmName are already valid k8s names, no need to lowercase
pvcNameTemplate: "{{trunc 10 .PlanName}}-{{trunc 20 .TargetVmName}}-{{.DiskIndex}}"

# Conditional naming for root vs data disks
pvcNameTemplate: "{{trunc 15 .TargetVmName}}-{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data-{{.DiskIndex}}{{end}}"

# Using source VM name (may need lowercase as source names aren't k8s compliant)
pvcNameTemplate: "{{if .Shared}}shared-{{end}}{{trunc 30 .VmName | lower}}-{{.DiskIndex}}"
```

#### PVCNameTemplateUseGenerateName

**Type:** bool

**Default:** `true`

| Value | Behavior |
|-------|----------|
| `true` | Template output is used as `generateName` prefix, Kubernetes adds a random suffix (e.g., "my-vm-disk-0-" becomes "my-vm-disk-0-abc12") |
| `false` | Template output is used as the exact PVC name. Warning: may cause conflicts if names are not unique |

#### VolumeNameTemplate

**Data Structure:** `VolumeNameTemplateData`

| Variable | Type | Description | K8s Compliant |
|----------|------|-------------|---------------|
| `.PVCName` | string | Name of the PVC mounted to the VM using this volume | Yes |
| `.VolumeIndex` | int | Sequential index of the volume interface (0-based) | Yes |

**Default Template:** `vol-{{.VolumeIndex}}`

**Examples:**
```yaml
volumeNameTemplate: "disk-{{.VolumeIndex}}"
volumeNameTemplate: "pvc-{{.PVCName}}"
```

#### NetworkNameTemplate

**Data Structure:** `NetworkNameTemplateData`

Variables refer to the **target/destination** network from the network mapping, not the source network.

| Variable | Type | Description | K8s Compliant |
|----------|------|-------------|---------------|
| `.NetworkName` | string | Name of the target Multus NetworkAttachmentDefinition (empty for pod network) | Yes |
| `.NetworkNamespace` | string | Namespace of the target NetworkAttachmentDefinition | Yes |
| `.NetworkType` | string | Type of target network: "Multus" or "Pod" | Yes |
| `.NetworkIndex` | int | Sequential index of the network interface (0-based) | Yes |

**Default Template:** `net-{{.NetworkIndex}}`

**Examples:**
```yaml
# Simple indexed naming
networkNameTemplate: "net-{{.NetworkIndex}}"

# Conditional based on network type
networkNameTemplate: '{{if eq .NetworkType "Pod"}}pod-net{{else}}{{.NetworkName}}-{{.NetworkIndex}}{{end}}'

# Include namespace for clarity
networkNameTemplate: '{{if eq .NetworkType "Multus"}}{{trunc 10 .NetworkNamespace}}-{{trunc 15 .NetworkName}}{{else}}pod{{end}}'
```

---

### OpenShift

OpenShift supports PVC naming templates only.

#### PVCNameTemplate

**Data Structure:** `OCPPVCNameTemplateData`

| Variable | Type | Description | K8s Compliant |
|----------|------|-------------|---------------|
| `.VmName` | string | Name of the VM in the source OpenShift cluster | Yes |
| `.TargetVmName` | string | Final VM name in the target cluster | Yes |
| `.PlanName` | string | Name of the migration plan | Yes |
| `.DiskIndex` | int | Index of the disk (0-based) | Yes |
| `.SourcePVCName` | string | Original name of the PVC in the source cluster | Yes |
| `.SourcePVCNamespace` | string | Namespace of the PVC in the source cluster | Yes |

**Default Template:** `{{.SourcePVCName}}` (preserves original PVC name)

**Examples:**
```yaml
# Prefix with target VM name
pvcNameTemplate: "{{.TargetVmName}}-{{.SourcePVCName}}"

# Add migration prefix
pvcNameTemplate: "migrated-{{.SourcePVCName}}-{{.DiskIndex}}"

# Use target VM name with disk index
pvcNameTemplate: "{{.TargetVmName}}-disk-{{.DiskIndex}}"
```

#### PVCNameTemplateUseGenerateName

**Status:** Ignored - OpenShift always uses the template output as the exact PVC name.

#### VolumeNameTemplate / NetworkNameTemplate

**Status:** Not supported - names are preserved from the source VM configuration.
