# Plan Specification Fields Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details all fields available in the Plan CR specification and their support across providers.

## Plan CR Structure

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Plan
metadata:
  name: my-migration-plan
  namespace: openshift-mtv
spec:
  # Provider references
  provider:
    source:
      name: vsphere-provider
      namespace: openshift-mtv
    destination:
      name: host  # or remote OpenShift provider

  # Resource mappings
  map:
    network:
      name: network-map
      namespace: openshift-mtv
    storage:
      name: storage-map
      namespace: openshift-mtv

  # Target configuration
  targetNamespace: migrated-vms

  # VMs to migrate
  vms:
    - id: vm-123
      name: my-vm

  # Migration options (see sections below)
```

---

## Basic Configuration

### Provider References

| Field | Required | Description |
|-------|----------|-------------|
| `provider.source` | Yes | Reference to source provider |
| `provider.destination` | Yes | Reference to destination provider (use `host` for same cluster) |

### Mappings

| Field | Required | Description |
|-------|----------|-------------|
| `map.network` | Yes | Reference to NetworkMap CR |
| `map.storage` | Yes | Reference to StorageMap CR |

### Target Configuration

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `targetNamespace` | Yes | - | Namespace where VMs will be created |
| `description` | No | - | Human-readable plan description |

---

## Migration Type and Behavior

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `cold` | Migration type: `cold`, `warm`, `live`, `conversion` |
| `warm` | bool | `false` | **Deprecated**: Use `type: warm` instead |
| `archived` | bool | `false` | Archive plan after completion |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `type: cold` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `type: warm` | Yes | Yes* | No | No | No | No | No |
| `type: live` | No | No | No | Yes** | No | No | No |
| `type: conversion` | Yes | No | No | No | No | No | No |

*oVirt warm migration requires `FEATURE_OVIRT_WARM_MIGRATION` feature gate
**OpenShift live migration requires `FEATURE_OCP_LIVE_MIGRATION` feature gate and KubeVirt `DecentralizedLiveMigration` on both clusters

---

## Target VM Configuration

### Labels and Scheduling

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `targetLabels` | map | - | Labels applied to target VMs |
| `targetNodeSelector` | map | - | Node selector for target VMs |
| `targetAffinity` | Affinity | - | Affinity rules for target VMs |
| `targetPowerState` | string | `auto` | Target VM power state: `on`, `off`, `auto` |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `targetLabels` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetNodeSelector` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetAffinity` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetPowerState` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

### Example

```yaml
spec:
  targetLabels:
    environment: production
    migrated-from: vsphere
  targetNodeSelector:
    node-role.kubernetes.io/worker: ""
  targetPowerState: "on"
```

---

## Convertor Pod Configuration

Settings for the virt-v2v conversion pods.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `convertorLabels` | map | - | Labels for conversion pods |
| `convertorNodeSelector` | map | - | Node selector for conversion pods |
| `convertorAffinity` | Affinity | - | Affinity rules for conversion pods |
| `conversionTempStorageClass` | string | - | Storage class for conversion scratch space |
| `conversionTempStorageSize` | string | - | Size of temporary storage PVC |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `convertorLabels` | Yes | No | No | No | Yes | Yes | Yes |
| `convertorNodeSelector` | Yes | No | No | No | Yes | Yes | Yes |
| `convertorAffinity` | Yes | No | No | No | Yes | Yes | Yes |
| `conversionTempStorageClass` | Yes | No | No | No | Yes | Yes | Yes |
| `conversionTempStorageSize` | Yes | No | No | No | Yes | Yes | Yes |

**Note:** Convertor settings only apply to providers that require guest conversion.

### Example

```yaml
spec:
  convertorLabels:
    workload-type: migration
  convertorNodeSelector:
    topology.kubernetes.io/zone: us-east-1a
  conversionTempStorageClass: fast-ssd
  conversionTempStorageSize: 100Gi
```

---

## Naming Templates

Templates for customizing resource names. See [Template Support Matrix](../template-support-matrix.md) for detailed template syntax.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `pvcNameTemplate` | string | Provider-specific | Template for PVC names |
| `pvcNameTemplateUseGenerateName` | bool | `true` | Use `generateName` instead of exact name |
| `volumeNameTemplate` | string | `vol-{{.VolumeIndex}}` | Template for volume interface names |
| `networkNameTemplate` | string | `net-{{.NetworkIndex}}` | Template for network interface names |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `pvcNameTemplate` | Yes | No | No | Yes | No | No | No |
| `pvcNameTemplateUseGenerateName` | Yes | No | No | Ignored | No | No | No |
| `volumeNameTemplate` | Yes | No | No | No | No | No | No |
| `networkNameTemplate` | Yes | No | No | No | No | No | No |

### Default Templates

| Provider | PVC Name Default |
|----------|------------------|
| vSphere | `{{trunc 4 .PlanName}}-{{trunc 4 .VmName}}-disk-{{.DiskIndex}}` |
| OpenShift | `{{.SourcePVCName}}` (preserves original) |

---

## Guest Conversion Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skipGuestConversion` | bool | `false` | Skip virt-v2v (raw copy mode) |
| `useCompatibilityMode` | bool | `true` | Use SATA/E1000E when skipping conversion |
| `installLegacyDrivers` | *bool | `nil` | Install legacy Windows drivers (auto-detect if nil) |
| `deleteGuestConversionPod` | bool | `false` | Delete conversion pod after success |
| `customizationScripts` | ObjectRef | - | ConfigMap with custom scripts |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `skipGuestConversion` | Yes | No | No | No | No | Yes | No |
| `useCompatibilityMode` | Yes | No | No | No | No | Yes | No |
| `installLegacyDrivers` | Yes | No | No | No | Yes | Yes | Yes |
| `deleteGuestConversionPod` | Yes | No | No | No | Yes | Yes | Yes |
| `customizationScripts` | Yes | No | No | No | Yes | Yes | Yes |

---

## Storage and Network Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `migrateSharedDisks` | bool | `true` | Migrate disks shared between VMs |
| `preserveStaticIPs` | bool | `true` | Preserve VM static IP configuration |
| `preserveClusterCPUModel` | bool | `false` | Preserve oVirt cluster CPU model |
| `transferNetwork` | ObjectRef | - | Network for disk transfer traffic |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `migrateSharedDisks` | Yes | Yes | No | No | No | No | No |
| `preserveStaticIPs` | Yes | No | No | No | No | No | No |
| `preserveClusterCPUModel` | No | Yes | No | No | No | No | No |
| `transferNetwork` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

---

## Provider-Specific Options

### EC2-Specific

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skipZoneNodeSelector` | bool | `false` | Skip automatic zone-based node selector |

EC2 migrations automatically add a node selector based on the target AZ. Set `skipZoneNodeSelector: true` to disable this behavior.

---

## Warm Migration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `runPreflightInspection` | bool | `true` | Run inspection before disk transfer |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `runPreflightInspection` | Yes* | No | No | No | No | No | No |

*Only applies to warm migrations from VMware

---

## Cleanup Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `deleteVmOnFailMigration` | bool | `false` | Delete target VM if migration fails |

### Support Matrix

All providers support `deleteVmOnFailMigration`.

---

## Complete Field Reference

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| **Basic** | | | | | | | |
| `targetNamespace` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `description` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `archived` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| **Migration Type** | | | | | | | |
| `type` | cold/warm/conversion | cold/warm* | cold | cold/live** | cold | cold | cold |
| **Target VM** | | | | | | | |
| `targetLabels` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetNodeSelector` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetAffinity` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetPowerState` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| **Convertor** | | | | | | | |
| `convertorLabels` | Yes | - | - | - | Yes | Yes | Yes |
| `convertorNodeSelector` | Yes | - | - | - | Yes | Yes | Yes |
| `convertorAffinity` | Yes | - | - | - | Yes | Yes | Yes |
| `conversionTempStorageClass` | Yes | - | - | - | Yes | Yes | Yes |
| `conversionTempStorageSize` | Yes | - | - | - | Yes | Yes | Yes |
| **Templates** | | | | | | | |
| `pvcNameTemplate` | Yes | - | - | Yes | - | - | - |
| `pvcNameTemplateUseGenerateName` | Yes | - | - | Ignored | - | - | - |
| `volumeNameTemplate` | Yes | - | - | - | - | - | - |
| `networkNameTemplate` | Yes | - | - | - | - | - | - |
| **Conversion** | | | | | | | |
| `skipGuestConversion` | Yes | - | - | - | - | - | - |
| `useCompatibilityMode` | Yes | - | - | - | - | - | - |
| `installLegacyDrivers` | Yes | - | - | - | Yes | Yes | Yes |
| `deleteGuestConversionPod` | Yes | - | - | - | Yes | Yes | Yes |
| `customizationScripts` | Yes | - | - | - | Yes | Yes | Yes |
| **Storage/Network** | | | | | | | |
| `migrateSharedDisks` | Yes | Yes | - | - | - | - | - |
| `preserveStaticIPs` | Yes | - | - | - | - | - | - |
| `preserveClusterCPUModel` | - | Yes | - | - | - | - | - |
| `transferNetwork` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| **Provider-Specific** | | | | | | | |
| `skipZoneNodeSelector` | - | - | - | - | - | Yes | - |
| `runPreflightInspection` | Yes* | - | - | - | - | - | - |
| **Cleanup** | | | | | | | |
| `deleteVmOnFailMigration` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

**Legend:** Yes = Supported, - = Not applicable/supported, * = Conditional
