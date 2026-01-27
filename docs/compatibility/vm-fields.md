# VM-Level Fields Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details the fields available for individual VMs within a Plan's `spec.vms` array. These settings allow per-VM customization that overrides plan-level defaults.

## VM Structure

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Plan
spec:
  vms:
    - id: vm-123                    # Required: VM identifier from inventory
      name: my-source-vm            # Optional: VM name (for display)

      # Optional per-VM settings
      targetName: my-target-vm
      targetPowerState: "on"
      rootDisk: "[datastore1] my-vm/disk-0.vmdk"
      instanceType: u1.medium

      # Template overrides
      pvcNameTemplate: "{{.TargetVmName}}-disk-{{.DiskIndex}}"
      volumeNameTemplate: "vol-{{.VolumeIndex}}"
      networkNameTemplate: "net-{{.NetworkIndex}}"

      # Hooks
      hooks:
        - step: PreHook
          hook:
            name: pre-migration-hook
            namespace: openshift-mtv
        - step: PostHook
          hook:
            name: post-migration-hook
            namespace: openshift-mtv

      # Encryption
      luks:
        name: luks-keys
        namespace: openshift-mtv
      nbdeClevis: false

      # Cleanup
      deleteVmOnFailMigration: false
```

---

## Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | VM identifier from source provider inventory |
| `name` | string | VM name (optional, for display purposes) |

The `id` field is the unique identifier from the source provider's inventory. Use the inventory API or UI to find VM IDs.

---

## Target Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `targetName` | string | Source VM name (DNS1123 normalized) | Custom name for the target VM |
| `targetPowerState` | string | Plan default or `auto` | Power state after migration: `on`, `off`, `auto` |
| `rootDisk` | string | Auto-detected | Primary boot disk identifier |
| `instanceType` | string | - | KubeVirt instance type to apply |

### Target Name

If not specified, the source VM name is automatically normalized to meet Kubernetes DNS-1123 requirements (lowercase alphanumerics and hyphens).

```yaml
vms:
  - id: vm-123
    targetName: prod-web-server-01  # Exact name to use
```

**Note:** If the specified name conflicts with an existing VM, the migration will fail.

### Target Power State

| Value | Behavior |
|-------|----------|
| `on` | Start VM after migration |
| `off` | Leave VM stopped after migration |
| `auto` | Match source VM's power state |

### Root Disk

Specify which disk should be the primary boot disk:

```yaml
vms:
  - id: vm-123
    rootDisk: "[datastore1] my-vm/disk-0.vmdk"  # vSphere format
```

### Instance Type

Apply a KubeVirt instance type to override VM sizing:

```yaml
vms:
  - id: vm-123
    instanceType: u1.medium
```

---

## Naming Templates

Override plan-level templates for specific VMs. See [Template Support Matrix](../template-support-matrix.md) for template syntax.

| Field | Type | Description |
|-------|------|-------------|
| `pvcNameTemplate` | string | Template for PVC names |
| `volumeNameTemplate` | string | Template for volume interface names |
| `networkNameTemplate` | string | Template for network interface names |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `pvcNameTemplate` | Yes | No | No | Yes | No | No | No |
| `volumeNameTemplate` | Yes | No | No | No | No | No | No |
| `networkNameTemplate` | Yes | No | No | No | No | No | No |

### Example

```yaml
vms:
  - id: vm-123
    pvcNameTemplate: "database-{{.DiskIndex}}"
    volumeNameTemplate: "db-vol-{{.VolumeIndex}}"
    networkNameTemplate: "db-net-{{.NetworkIndex}}"
```

---

## Migration Hooks

Hooks allow running custom Ansible playbooks before or after VM migration.

| Field | Type | Description |
|-------|------|-------------|
| `hooks` | []HookRef | List of hook references |
| `hooks[].step` | string | Hook execution point: `PreHook` or `PostHook` |
| `hooks[].hook` | ObjectRef | Reference to Hook CR |

### Hook Steps

| Step | When Executed |
|------|---------------|
| `PreHook` | Before migration starts |
| `PostHook` | After migration completes successfully |

### Support Matrix

All providers support migration hooks.

### Example

```yaml
vms:
  - id: vm-123
    hooks:
      - step: PreHook
        hook:
          name: backup-hook
          namespace: openshift-mtv
      - step: PostHook
        hook:
          name: validation-hook
          namespace: openshift-mtv
```

See [Migration Hooks](../hooks.md) for creating Hook CRs.

---

## Disk Encryption

### LUKS Decryption

For VMs with LUKS-encrypted disks, provide a secret containing decryption keys.

| Field | Type | Description |
|-------|------|-------------|
| `luks` | ObjectRef | Reference to Secret with LUKS keys |

### Support Matrix

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `luks` | Yes | Yes | No | No | No | No | No |

### LUKS Secret Format

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: luks-keys
  namespace: openshift-mtv
type: Opaque
stringData:
  # Key name should match LUKS device UUID or label
  luks-uuid-1234: "passphrase-for-device"
```

### Example

```yaml
vms:
  - id: vm-123
    luks:
      name: luks-keys
      namespace: openshift-mtv
```

### Clevis/NBDE Auto-Unlock

For VMs using Clevis with NBDE (Network-Bound Disk Encryption), enable automatic unlock via TANG server:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `nbdeClevis` | bool | `false` | Attempt Clevis auto-unlock |

```yaml
vms:
  - id: vm-123
    nbdeClevis: true
```

**Requirements:**
- TANG server must be accessible from the target cluster
- VM must be configured with Clevis for NBDE

**Note:** If both `luks` and `nbdeClevis` are set, `nbdeClevis` takes precedence.

---

## Cleanup Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `deleteVmOnFailMigration` | bool | `false` | Delete target VM if migration fails |

This overrides the plan-level setting for individual VMs.

**Note:** If the plan-level `deleteVmOnFailMigration` is `true`, VM-level settings are ignored (plan takes precedence).

### Example

```yaml
vms:
  - id: vm-123
    deleteVmOnFailMigration: true  # Delete this VM on failure
  - id: vm-456
    deleteVmOnFailMigration: false  # Preserve for debugging
```

---

## Complete Field Reference

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| **Identity** | | | | | | | |
| `id` | Req | Req | Req | Req | Req | Req | Req |
| `name` | Opt | Opt | Opt | Opt | Opt | Opt | Opt |
| **Target** | | | | | | | |
| `targetName` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `targetPowerState` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `rootDisk` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `instanceType` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| **Templates** | | | | | | | |
| `pvcNameTemplate` | Yes | - | - | Yes | - | - | - |
| `volumeNameTemplate` | Yes | - | - | - | - | - | - |
| `networkNameTemplate` | Yes | - | - | - | - | - | - |
| **Hooks** | | | | | | | |
| `hooks` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| **Encryption** | | | | | | | |
| `luks` | Yes | Yes | - | - | - | - | - |
| `nbdeClevis` | Yes | Yes | - | - | - | - | - |
| **Cleanup** | | | | | | | |
| `deleteVmOnFailMigration` | Yes | Yes | Yes | Yes | Yes | Yes | Yes |

**Legend:** Req = Required, Yes = Supported, Opt = Optional, - = Not supported
