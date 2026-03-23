---
title: ovirt-lun-migration
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-03-23
last-updated: 2026-03-23
status: implemented
---

# Migration of oVirt Direct LUN (FC and iSCSI) Block Devices

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Forklift supports migrating oVirt/RHV virtual machines that use direct LUN
disks backed by Fibre Channel (FC) or iSCSI storage. Unlike image-based disks
that are copied from source to destination, LUN disks are **not copied**.
Instead, Forklift detaches the LUN from the source VM and re-attaches it on
the destination by creating static Kubernetes PersistentVolume and
PersistentVolumeClaim objects that reference the same physical LUN. The target
KubeVirt VM then accesses the LUN through SCSI command passthrough using the
`lun` disk device type.

This document describes the source (oVirt), target infrastructure (Kubernetes
PV/PVC, KubeVirt VM), and the Forklift implementation that bridges them.

## Motivation

Many oVirt/RHV environments use direct LUN storage -- Fibre Channel or iSCSI
LUNs mapped directly to VMs rather than stored within an oVirt storage domain.
These LUNs typically reside on shared SAN arrays accessible to both the source
oVirt cluster and the destination OpenShift cluster. Copying the contents of
these LUNs would be wasteful and slow when the same SAN is reachable from both
environments. Instead, Forklift re-attaches the existing LUN to the migrated
VM on the target side, preserving data in place and enabling near-instant
migration of the storage layer.

### Goals

* Migrate oVirt VMs that use direct LUN disks (FC or iSCSI) to OpenShift
  Virtualization without copying disk contents.
* Automatically detect whether a LUN is FC or iSCSI from the oVirt inventory
  and create the correct Kubernetes PV type.
* Ensure the migrated VM can be live-migrated on the target cluster by using
  `ReadWriteMany` access mode and `Block` volume mode.
* Detach the LUN from the source VM after successful migration to avoid
  dual-use data corruption.

### Non-Goals

* Copying LUN disk contents (this is a re-attachment, not a data migration).
* Supporting dynamic provisioning or CSI drivers for LUN disks -- static PVs
  are the correct approach for pre-existing LUNs.
* Multi-path I/O (MPIO) on the target -- only the first logical unit is used.
* SCSI Persistent Reservation on the target VM -- the `reservation` flag is
  not set.

## Background

### oVirt LUN Storage Model

oVirt VMs can have two types of disk storage:

* **Image disks** (`storage_type: image`): Stored within an oVirt storage
  domain (NFS, GlusterFS, or block-based). The data is managed by oVirt and
  transferred via the ImageIO API during migration.

* **LUN disks** (`storage_type: lun`): Directly attached to the VM via iSCSI
  or Fibre Channel. The LUN is not managed by an oVirt storage domain. Instead,
  oVirt stores the connection metadata (target portal, IQN, LUN mapping for
  iSCSI; WWID for FC) in the disk's `lun_storage` property. Direct LUN disks
  **only support raw format** -- qcow2 and other formats are not available
  ([oVirt Direct LUN spec](https://www.ovirt.org/develop/release-management/features/storage/direct-lun.html)).
  The guest OS manages its own filesystem on top of the raw block device.

The two storage types differ in format and capabilities:

| | Image disk | Direct LUN disk |
|---|---|---|
| Storage type | `image` | `lun` |
| Location | Inside an oVirt storage domain | Directly on a SAN LUN (iSCSI or FC) |
| Disk format | `raw` or `cow` (qcow2) | **`raw` only** |
| Identified by | PDIV quartet (Pool, Domain, Image, Volume) | GUID/WWID of the block device |
| Snapshots | Supported (via qcow2 chain) | Not supported |
| Managed by oVirt SD | Yes | No -- oVirt only stores connection metadata |

The oVirt REST API represents a LUN disk as:

```json
{
  "id": "disk-uuid",
  "storage_type": "lun",
  "lun_storage": {
    "logical_units": {
      "logical_unit": [
        {
          "id": "lun-wwid",
          "address": "10.0.0.1",
          "port": "3260",
          "target": "iqn.2024-01.com.example:storage",
          "lun_mapping": 0,
          "size": 10737418240
        }
      ]
    }
  }
}
```

For FC LUNs, the `address`, `port`, and `target` fields are empty; the `id`
field contains the WWID used for FC device identification.

The oVirt engine version must be **>= 4.5.2.1** to expose the `lun_storage`
details required for migration (commit `e7c1f585` in ovirt-engine).

### Kubernetes FC and iSCSI PV Support

Kubernetes natively supports Fibre Channel and iSCSI as in-tree
PersistentVolume types
([docs](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)).

**FC PersistentVolume:**

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: fc-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  fc:
    wwids: ["50060e801049cfd1"]
    readOnly: false
```

FC volumes support identification via `targetWWNs` + `lun` or via `wwids`.
The `wwids` approach is recommended as it is independent of access paths.

**iSCSI PersistentVolume:**

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: iscsi-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  iscsi:
    targetPortal: 10.0.0.1:3260
    iqn: iqn.2024-01.com.example:storage
    lun: 0
    readOnly: false
```

**Documented access modes and volume modes:**

| Volume Plugin | RWO | ROX | RWX | RWOP | Filesystem | Block |
|---|---|---|---|---|---|---|
| FC | Yes | Yes | No | No | Yes | Yes |
| iSCSI | Yes | Yes | No | No | Yes | Yes |

The Kubernetes documentation does not list `ReadWriteMany` (RWX) as a
supported access mode for FC or iSCSI in-tree plugins. However, access modes
in Kubernetes are **advisory** -- they are used for PVC-to-PV matching, not
for enforcing I/O restrictions at the storage layer. A shared SAN LUN is
physically accessible from multiple nodes regardless of the declared access
mode.

**Provisioning models:**

* **Static**: An administrator creates a PV manually with the FC or iSCSI
  connection details. A PVC with `storageClassName: ""` binds to it via label
  selectors or capacity matching. No CSI driver is required.
* **Dynamic**: A CSI driver for the SAN array (e.g., PowerMax, Pure, NetApp
  Trident) can dynamically provision LUNs. This requires a StorageClass
  referencing the CSI driver.

For pre-existing LUNs that already contain data (as in a migration scenario),
**static provisioning** is the correct approach.

### KubeVirt LUN Disk Support

KubeVirt supports four disk device types: `disk`, `lun`, `cdrom`, and `floppy`
([docs](https://kubevirt.io/user-guide/storage/disks_and_volumes/)).

The `lun` type exposes the volume as a **LUN device** to the VM, enabling
**arbitrary SCSI command passthrough**. This differs from `disk`, which
presents an ordinary virtio/SCSI disk without passthrough semantics.

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
spec:
  domain:
    devices:
      disks:
      - name: mypvcdisk
        lun: {}
  volumes:
    - name: mypvcdisk
      persistentVolumeClaim:
        claimName: mypvc
```

The `lun` device type is designed for:

* Workloads that need raw SCSI semantics (e.g., databases, clustered
  filesystems)
* SCSI Persistent Reservation support (via the `reservation: true` flag and
  `PersistentReservation` feature gate)
* Direct block-level access without filesystem virtualization overhead

**PVC requirements for KubeVirt LUN disks:**

| Requirement | Details |
|---|---|
| Volume Mode | `Block` recommended for raw device passthrough; `Filesystem` works if PVC contains a `disk.img` file |
| Access Mode | **RWX required for live migration** -- KubeVirt checks access modes and marks the VMI as non-`LiveMigratable` if volumes are not RWX |
| Volume Source | PVC or DataVolume required; `containerDisk` cannot be used with `lun` |

**Live migration and RWX:** KubeVirt's live migration documentation explicitly
states: *"Virtual machines using a PersistentVolumeClaim (PVC) must have a
shared ReadWriteMany (RWX) access mode to be live migrated."*
([docs](https://kubevirt.io/user-guide/compute/live_migration/)). The
`LiveMigratable` condition is calculated based on the access mode of VM
volumes.

This creates a tension: Kubernetes FC/iSCSI plugins document only RWO and ROX,
but KubeVirt requires RWX for live migration. In practice, since access modes
are advisory and SAN LUNs inherently support multi-node access, setting RWX on
FC/iSCSI PVs works correctly.

## Proposal

### User Stories

#### Story 1

As a migration administrator, I have oVirt VMs with direct iSCSI LUN disks on
a shared SAN that is accessible from both my oVirt cluster and my OpenShift
cluster. I want to migrate these VMs to OpenShift Virtualization without
copying the disk contents, since the same LUN can be directly accessed from the
target environment.

#### Story 2

As a migration administrator, I have oVirt VMs with Fibre Channel LUN disks. I
want Forklift to automatically detect the FC connection details from the oVirt
inventory and create the correct Kubernetes PV so the migrated VM can access
the same LUN on the target cluster.

#### Story 3

As a migration administrator, after migrating a VM with LUN disks, I want the
LUNs to be detached from the source oVirt VM to prevent dual-use data
corruption, while keeping them available in the source environment for rollback
if needed.

#### Story 4

As a migration administrator, before migrating VMs with direct LUN disks, I
want guidance on creating a SAN-level snapshot of each LUN so that I have a
reliable rollback path if the migration fails or the target VM writes
unexpected data to the disk.

### Implementation Details

#### Architecture

```
  oVirt Source                  Forklift                    OpenShift Target
  ┌─────────────┐     ┌────────────────────┐     ┌──────────────────────────┐
  │ VM           │     │ 1. Read oVirt      │     │ Static PV (FC or iSCSI)  │
  │  └─ LUN disk │────>│    inventory       │────>│  └─ wwids / targetPortal │
  │     (FC or   │     │ 2. Create PV+PVC   │     │ PVC (Block, RWX, no SC)  │
  │      iSCSI)  │     │ 3. Wire KubeVirt   │     │ KubeVirt VM              │
  │              │     │    lun device       │     │  └─ lun: {} device       │
  │              │     │ 4. Detach from src  │     │     backed by PVC        │
  └─────────────┘     └────────────────────┘     └──────────────────────────┘
```

#### FC vs iSCSI Detection

The oVirt `LogicalUnit` object carries connection details. Forklift
differentiates FC and iSCSI by checking the `Address` field:

* **iSCSI** (Address is non-empty): Creates a PV with
  `ISCSIPersistentVolumeSource` using `TargetPortal` (address:port), `IQN`
  (target), and `Lun` (mapping number).
* **FC** (Address is empty): Creates a PV with `FCVolumeSource` using `WWIDs`
  populated from the logical unit's `LunID`.

#### PV and PVC Configuration

LUN PVs and PVCs use hard-coded settings that are not influenced by the
StorageMap:

| Property | PV Value | PVC Value |
|---|---|---|
| Volume Mode | `Block` | `Block` |
| Access Mode | `ReadWriteMany` | `ReadWriteMany` |
| StorageClass | (none) | `""` (empty -- disables dynamic provisioning) |
| Capacity | From `LogicalUnit.Size` | From `LogicalUnit.Size` |
| Binding | Label `volume: <vmName>-<attachmentID>` | Selector matching the PV label |

**Why `Block` is the only correct volume mode:** oVirt direct LUN disks are
always raw block devices (qcow2 is not supported for LUNs). The guest OS
manages its own filesystem on top of the raw device. While Kubernetes iSCSI
and FC PVs support both `Filesystem` and `Block` volume modes, using
`Filesystem` would cause Kubernetes to create a new filesystem (ext4/xfs) on
the device at mount time, **destroying the existing guest data**. `Block` mode
passes the raw device through unchanged, which is what KubeVirt's `LunTarget`
requires for SCSI passthrough.

**Why `ReadWriteMany`:** RWX is used intentionally to ensure the migrated VM
is live-migratable on the target cluster (KubeVirt requires RWX for live
migration), even though Kubernetes FC/iSCSI in-tree documentation only lists
RWO and ROX. This works because access modes are advisory and the underlying
SAN storage physically supports multi-node access.

#### StorageMap Behavior

LUN disks are **excluded** from StorageMap validation and usage:

* The `StorageMapped` validator skips disks with `StorageType == "lun"` --
  they do not need a storage domain mapping.
* The `DataVolumes` builder only processes `StorageType == "image"` disks
  through the storage map.
* LUN PVs/PVCs are created by dedicated `LunPersistentVolumes` /
  `LunPersistentVolumeClaims` functions with hard-coded settings.

#### StorageMap Requirement for LUN-Only VMs

A migration plan **always requires** a `spec.map.storage` reference, even when
all VMs in the plan use only LUN disks and no image disks. The plan validation
in `pkg/controller/plan/validation.go` unconditionally checks whether the
StorageMap ref is set:

```go
if !libref.RefSet(&ref) {
    newCnd.Reason = NotSet
    plan.Status.SetCondition(newCnd)  // Critical: StorageRefNotValid
    return
}
```

If the ref is not set, the plan is blocked with a **Critical**
`StorageRefNotValid` condition regardless of disk types.

**Workaround: empty StorageMap.** An empty StorageMap (with `spec.map: []`)
reaches the `Ready` condition as long as it has valid provider references. The
StorageMap controller iterates over `spec.map` entries during validation; with
an empty list, no validation errors are produced, and the `Ready` condition is
set. For LUN-only migrations, users can create a minimal StorageMap:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
metadata:
  name: lun-only-map
  namespace: openshift-mtv
spec:
  provider:
    source:
      name: ovirt-provider
      namespace: openshift-mtv
    destination:
      name: host
      namespace: openshift-mtv
  map: []
```

Once this StorageMap exists and is `Ready`, plans that reference it will pass
the StorageMap validation. The per-VM `StorageMapped` check also passes because
it skips all LUN disks -- when every disk is a LUN, the check returns `ok`
without requiring any map entries.

**Known limitation:** Requiring users to create an empty StorageMap for
LUN-only migrations is unnecessary boilerplate. A future improvement could make
the StorageMap reference optional on the plan when all selected VMs have only
LUN disks. This would require the plan validation to inspect the selected VMs'
disk types before enforcing the StorageMap ref.

#### KubeVirt VM Disk Wiring

LUN disks are wired to the KubeVirt VM using the `LunTarget` device type
(SCSI passthrough), while image disks use the regular `DiskTarget`:

| Disk Type | KubeVirt Device | Serial | Bus |
|---|---|---|---|
| Image | `DiskTarget` | Set to disk ID | Mapped from oVirt interface |
| LUN | `LunTarget` | Not set | Mapped from oVirt interface |

Bus mapping from oVirt interface types: `virtio_scsi` -> `scsi`, `sata`/`ide`
-> `sata`, default -> `virtio`.

#### Post-Migration LUN Detachment

After successful migration, Forklift detaches (but does not delete) LUN disks
from the source oVirt VM via the oVirt API. This prevents dual-use data
corruption while preserving the LUN in the source environment for potential
rollback.

#### Progress Tracking

LUN disks are excluded from migration progress tracking because no data copy
occurs. PVCs with the `lun` annotation are skipped during progress calculation.

### Security, Risks, and Mitigations

**Data corruption risk**: After migration, the LUN is accessible from both the
source and target environments (detached but not deleted from oVirt). If the
LUN is re-attached to the source VM while still in use on the target,
simultaneous writes from both sides will cause data corruption. This is
documented in user-facing documentation as a prerequisite warning.

**Irreversible ownership transfer (no built-in rollback)**: Direct LUN
migration is a re-attachment, not a copy. Once the target KubeVirt VM is
created with a reference to the LUN, the target VM effectively owns the disk.
If the target VM boots -- even briefly -- it may write to the LUN (filesystem
journals, swap, systemd machine-id rotation, etc.), making a clean return to
the source environment uncertain. Unlike image-based disks, oVirt does not
support snapshots for direct LUN disks, so there is no oVirt-side safety net.

*Mitigation -- SAN-level snapshot before migration:* Administrators should use
the storage array's native snapshot or clone capability (e.g., NetApp
Snapshot, Dell/EMC SnapVX) to create a point-in-time copy of the LUN **before** 
starting the migration plan. This provides a rollback path that is independent
of both oVirt and Forklift. If the migration fails or the target VM misbehaves,
the administrator can restore the LUN from the array snapshot and re-attach 
it to the original oVirt VM. Forklift does not automate this step -- it is a
manual prerequisite that should be part of the migration runbook.

**SAN accessibility**: Target OpenShift nodes must have physical connectivity
to the iSCSI targets or FC fabric. This is a network/SAN zoning prerequisite
that cannot be validated by Forklift at plan time.

**Single logical unit**: Only `LogicalUnit[0]` is used per disk attachment.
Multi-path I/O configurations with multiple logical units per LUN are not
explicitly handled. Kubernetes and the node's multipath daemon manage path
redundancy at the host level.

**RWX on non-RWX plugins**: Setting `ReadWriteMany` on FC/iSCSI PVs exceeds
the documented Kubernetes support matrix. This is intentional for live
migration support and works because access modes are advisory. Future
Kubernetes versions that enforce access modes more strictly could break this.

## Design Details

### Validation

| Check | Severity | Description |
|---|---|---|
| oVirt engine >= 4.5.2.1 | Critical | Required for LUN logical unit details in the API |
| LUN logical unit `Size > 0` | Critical | Disks with zero or negative size are invalid |
| Disk `storage_type` is `image` or `lun` | Critical | Other storage types are unsupported (Rego policy) |
| StorageMap covers non-LUN disks | Critical | LUN disks are skipped; image disks must be mapped |

### Key Code Locations

| Component | File | Lines |
|---|---|---|
| oVirt Disk/LUN model | `pkg/controller/provider/model/ovirt/model.go` | 251-277 |
| oVirt REST mapping | `pkg/controller/provider/container/ovirt/resource.go` | 661-732 |
| LUN disk detail fetch | `pkg/controller/provider/container/ovirt/model.go` | 1044-1062 |
| LUN PV/PVC creation | `pkg/controller/plan/adapter/ovirt/builder.go` | 639-751 |
| VM disk device mapping | `pkg/controller/plan/adapter/ovirt/builder.go` | 489-538 |
| StorageMap validation | `pkg/controller/plan/adapter/ovirt/validator.go` | 153-172 |
| DirectStorage version check | `pkg/controller/plan/adapter/ovirt/validator.go` | 174-226 |
| LUN detach from source | `pkg/controller/plan/adapter/ovirt/client.go` | 529-551 |
| createLunDisks orchestration | `pkg/controller/plan/kubevirt.go` | 916-935 |
| Rego validation policy | `validation/policies/io/konveyor/forklift/ovirt/disk_storage_type.rego` | - |
| StorageMap API types | `pkg/apis/forklift/v1beta1/mapping.go` | 100-146 |

### Test Plan

* **Unit tests**: Verify that `LunPersistentVolumes` creates iSCSI PV when
  `Address` is non-empty and FC PV when `Address` is empty.
* **Unit tests**: Verify that `LunPersistentVolumeClaims` creates PVCs with
  `Block` volume mode, `ReadWriteMany` access, and empty StorageClass.
* **Unit tests**: Verify that `StorageMapped` passes for VMs with LUN-only
  disks even when no StorageMap entries exist.
* **Unit tests**: Verify that `DirectStorage` rejects LUN disks when the oVirt
  engine version is below 4.5.2.1.
* **Unit tests**: Verify that `InvalidDiskSizes` catches LUN disks with zero
  or negative logical unit size.
* **Integration tests**: End-to-end migration of an oVirt VM with a direct
  iSCSI LUN disk to OpenShift Virtualization.
* **Integration tests**: End-to-end migration of an oVirt VM with a direct FC
  LUN disk to OpenShift Virtualization.
* **Integration tests**: Verify the migrated VM can be live-migrated on the
  target cluster.

### Upgrade / Downgrade Strategy

This feature requires no schema changes to existing CRDs. LUN detection and
handling is automatic based on the `storage_type` field from the oVirt
inventory. No migration of existing resources is needed.

## Implementation History

* oVirt engine 4.5.2.1 -- Added LUN logical unit details to the disk API
  (prerequisite).
* Forklift initial LUN support -- Implemented static PV/PVC creation for
  FC and iSCSI LUNs with `Block` volume mode and `ReadWriteMany` access.
* MTV 2.10.4 -- Fixed a bug where direct LUN disks failed with
  "Disk has an invalid capacity of 0 bytes" (MTV-4180). The fix validates
  LUN logical unit size instead of disk provisioned size.

## Drawbacks

* **No data copy / no built-in rollback**: Unlike image disks, LUN disks are
  not copied -- the original LUN is re-attached to the target VM. This means
  there is no Forklift-managed backup. Once the target VM boots, writes to the
  LUN (filesystem journals, swap, etc.) may make reverting to the source
  problematic. oVirt does not support snapshots for direct LUN disks, so
  administrators who need a rollback safety net must create a SAN-level
  snapshot (e.g., NetApp Snapshot, Dell/EMC SnapVX, Pure SafeMode) before
  starting the migration. If the SAN is not accessible from the target
  cluster, migration will fail at runtime (the PV will not bind to a node).
  Forklift cannot validate SAN connectivity at plan time.
* **Single logical unit**: Only `LogicalUnit[0]` is used. VMs with multi-path
  LUN configurations may not have all paths represented in the target PV.
* **RWX on non-RWX plugins**: The use of `ReadWriteMany` exceeds documented
  Kubernetes support for FC/iSCSI in-tree plugins, which could be affected by
  future Kubernetes changes.
* **No SCSI Persistent Reservation**: Forklift does not set the KubeVirt
  `reservation: true` flag, so workloads that relied on SCSI PR in oVirt will
  need manual reconfiguration on the target.
* **Empty StorageMap required for LUN-only VMs**: Plans always require a
  StorageMap reference, even when all VM disks are LUNs. Users must create an
  empty StorageMap (`spec.map: []`) with valid provider references as
  boilerplate. A future improvement could make the StorageMap optional when no
  image disks need mapping.

## Alternatives

1. **Copy LUN contents via CDI**: Instead of re-attaching the LUN, copy its
   contents into a new PVC using CDI. This would work without shared SAN access
   but is significantly slower for large disks and requires destination storage
   capacity.
2. **CSI-based dynamic provisioning**: Use a SAN CSI driver to dynamically
   create a new LUN and copy data. This adds complexity (CSI driver
   installation, StorageClass configuration) and is unnecessary when the same
   LUN is reachable from the target.
3. **StorageMap integration for LUNs**: Allow LUN disks to be included in
   StorageMap with configurable access mode and volume mode. This would give
   users more control but adds complexity to an otherwise straightforward
   re-attachment flow.
