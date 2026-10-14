---
title: csi-import-copy-offload
authors:
  - "@rgolangh"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-05-14
last-updated: 2026-05-14
status: provisional
see-also:
  - "/docs/enhancements/vsphere-copy-offload-populator.md"
  - "/cmd/vsphere-copy-offload-populator/README.md"
---

# Copy-Offload via CSI Volume Import

Introduce a new copy-offload method that leverages CSI driver volume import capabilities
to migrate VVol and RDM disks. Instead of running a dedicated populator pod that talks
directly to the storage array API, this approach creates a PVC with vendor-specific import
annotations and lets the CSI driver handle the volume provisioning from the existing
source volume. This results in less code, less configuration, safer operations, and
better long-term maintainability for MTV.

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Open Questions

1. Should volume name resolution run in the forklift-controller process, or in a
   lightweight resolver pod? Running in-controller is simpler but adds REST calls to the
   storage array from the controller. A resolver pod adds isolation but reintroduces pod
   overhead (though much lighter than the full populator).
2. Some CSI drivers expose events on the PVC during import (e.g. `ProvisioningSucceeded`,
   `ExternalProvisioning`). Should we surface these as step annotations for observability?
3. For vendors that only support static PV provisioning (Tier 2), should we auto-generate
   PV objects or require the user to pre-create them?

## Summary

Several CSI drivers support importing an existing storage volume into Kubernetes by
annotation on the PVC (e.g. HPE `csi.hpe.com/importVolAsClone`) or via static PV
provisioning with the correct `volumeHandle`. This enhancement adds a new offload plugin
(`CsiImportPluginConfig`) to the StorageMap API, alongside the existing
`VSphereXcopyPluginConfig`. When configured, the forklift controller resolves VM disk
backing to a storage array volume name, then creates a PVC with the appropriate import
annotation. The CSI driver handles the clone/import — no populator pod, no custom CR,
no metrics server.

## Motivation

The current `vsphere-copy-offload-populator` handles volume-to-volume copy for VVol and
RDM disks by deploying a populator pod per disk. The pod resolves the VM disk to a storage
array volume via vendor-specific REST APIs, performs the copy using clone/snapshot
operations, and manages host mapping/unmapping. Progress is tracked via Prometheus metrics
scraped from the pod.

This works but carries significant complexity:

- **Per-vendor REST API integration** — 9 vendor clients today, each with credential
  management, API versioning, and error handling
- **Host mapping/unmapping** — iGroup/host associations, FC/iSCSI adapter discovery, and
  LUN mapping — all operations with blast radius on the storage array
- **Pod lifecycle** — failure modes (OOM, network partitions, eviction) plus RBAC, service
  accounts, and secret management
- **Maintenance burden** — each vendor client must track storage firmware updates and API
  changes

CSI drivers that support volume import already handle all of this through their standard
`CreateVolume` flow. By leveraging them, MTV delegates the heavy lifting to the CSI driver
that is already installed and validated on the target cluster.

### Goals

- Add a CSI import offload plugin to the StorageMap API for VVol and RDM disks
- Eliminate the populator pod for vendors that support CSI volume import
- Extract volume name resolution logic into a shared module usable by both the existing
  populator and the new CSI import path
- Validate StorageClass configuration at plan-creation time to prevent runtime failures
- Phase 1 targets HPE Primera/3PAR/Alletra with `importVolAsClone`

### Non-Goals

- Replacing the existing XCOPY populator — it remains available for VMDK disks and vendors
  without CSI import support
- Supporting warm migration with CSI import (same limitation as NetApp Shift)
- Implementing CSI import for non-vSphere providers

## Proposal

### User Stories

#### Story 1

As a migration administrator with HPE Primera storage backing my vSphere VVol datastores,
I want to migrate VMs to KubeVirt using the HPE CSI driver's import capability so that I
don't need to deploy and manage populator pods, and the migration uses a battle-tested
CSI code path rather than direct array API calls.

#### Story 2

As a platform engineer, I want to configure the StorageMap with a CSI import plugin so
that VVol and RDM disk migrations use the CSI driver for volume provisioning, reducing
the operational surface area and secret management complexity.

### Implementation Details

#### 1. StorageMap API Extension

Add a new plugin config alongside the existing `VSphereXcopyPluginConfig`:

```go
type OffloadPlugin struct {
    VSphereXcopyPluginConfig *VSphereXcopyPluginConfig `json:"vsphereXcopyPluginConfig,omitempty"`
    CsiImportPluginConfig    *CsiImportPluginConfig    `json:"csiImportPluginConfig,omitempty"`
}

type CsiImportPluginConfig struct {
    SecretRef            string               `json:"secretRef"`
    StorageVendorProduct StorageVendorProduct  `json:"storageVendorProduct"`
}
```

The `CsiImportPluginConfig` reuses the same `StorageVendorProduct` enum and secret
structure. The controller still needs array credentials for volume name resolution.
The difference is what happens after resolution: a PVC with import annotations is
created instead of launching a populator pod.

#### 2. Code Path: `PopulatorVolumes` in `vsphere/builder.go`

The method gains a new branch:

```
PopulatorVolumes(vmRef, annotations, secretName)
  │
  ├── For each disk in VM:
  │     │
  │     ├── mapping.OffloadPlugin.VSphereXcopyPluginConfig != nil
  │     │     └── (existing path) Create VSphereXcopyVolumePopulator CR + PVC
  │     │
  │     └── mapping.OffloadPlugin.CsiImportPluginConfig != nil
  │           │
  │           ├── 1. Resolve VM disk to storage array volume name
  │           │     └── vmware.Client.GetVMDiskBacking() → VVol/RDM backing
  │           │     └── vendor StorageApi.ResolvePVToLUN() → array volume name
  │           │
  │           ├── 2. Create PVC with import annotations
  │           │     ├── StorageClass from mapping destination
  │           │     ├── VolumeMode: Block
  │           │     ├── No DataSourceRef (CSI driver handles provisioning)
  │           │     └── Annotations:
  │           │         ├── csi.hpe.com/importVolAsClone: <resolved_volume_name>
  │           │         ├── forklift.konveyor.io/disk-source: <volume_name>
  │           │         └── forklift.konveyor.io/copy-method: csi-import
  │           │
  │           └── 3. Return PVC (no populator CR needed)
  │
  └── Return all PVCs
```

#### 3. Volume Name Resolution — Shared Module

The resolution logic already exists in the populator's vendor implementations (e.g.
`ResolvePVToLUN` in each `StorageApi`). Rather than duplicating it, we extract it into
a shared Go module that both the populator and the controller can import:

- Move the resolution interfaces (`StorageResolver`, `ResolvePVToLUN`) and vendor-specific
  implementations into a shared package (e.g. `pkg/storage/resolver/`)
- The populator imports from the shared package instead of its local copy
- The controller imports the same package for CSI import resolution
- The vendor factory (selecting the right `StorageApi` by `StorageVendorProduct`) is also
  shared

This keeps a single source of truth for volume resolution and avoids drift between the
two code paths.

**Design choice**: Run resolution in the controller process (forklift-controller). This
avoids pod-per-disk overhead. The controller already has access to the secret and can make
a single REST API call per disk.

#### 4. Progress Reporting

| Phase | Signal | Behavior |
|-------|--------|----------|
| Volume resolution | Synchronous in controller | Immediate |
| CSI import/clone | PVC transitions Pending → Bound | Binary: pending or complete |
| Conversion (virt-v2v) | Existing progress model | Unchanged |

This is the same model used by the NetApp Shift integration. We lose per-disk percentage
granularity, but eliminate the populator pod, metrics server, and progress-related failure
modes.

#### 5. Import Mode: Clone vs. Destructive

Default mode is **clone** (`importVolAsClone` for HPE):

1. CSI driver creates a snapshot of the source volume on the array
2. CSI driver promotes/splits the snapshot into an independent volume
3. The new volume is attached to the Kubernetes node
4. The PVC binds to the new PV

The source VM volume is never modified — critical for rollback safety and testing.
Destructive import (`importVolumeName`) could be offered as an opt-in for final cutover
in a future phase.

#### 6. StorageClass Enforcement

The controller validates at plan-creation time that the StorageClass supports CSI import:

- **HPE**: Check that the StorageClass has `allowOverrides` containing `importVolAsClone`
- **Generic**: Optionally check for a well-known annotation
  (e.g. `forklift.konveyor.io/csi-import-capable: "true"`)

#### 7. Validation Rules

| Condition | Category | Message |
|-----------|----------|---------|
| `CsiImportWarmNotSupported` | Critical | CSI import does not support warm migration |
| `CsiImportStorageClassMisconfigured` | Critical | StorageClass does not allow import overrides |
| `CsiImportUnsupportedDiskType` | Warning | Disk is VMDK (not VVol/RDM) — CSI import not applicable |
| `CsiImportAndXcopyConflict` | Critical | Same storage pair has both XCOPY and CSI import configured |

### CSI Import Capabilities by Vendor

#### Tier 1 — PVC Annotation-Based Import (Fully Dynamic)

| Vendor | CSI Driver | Import Annotation | Clone Annotation | Notes |
|--------|-----------|-------------------|------------------|-------|
| HPE Primera/3PAR/Alletra | `csi.hpe.com` | `csi.hpe.com/importVolumeName` | `csi.hpe.com/importVolAsClone` | StorageClass needs `allowOverrides: importVolumeName,importVolAsClone`. Available since CSI driver 1.2.0+. |
| IBM FlashSystem | `block.csi.ibm.com` | Static PV with `volumeHandle: SVC:<volume_UID>` | — | Requires PV+PVC creation. |

#### Tier 2 — Static PV/PVC Provisioning

| Vendor | CSI Driver | volumeHandle Format | Notes |
|--------|-----------|---------------------|-------|
| Pure Storage | `pure-csi` | Volume name (block) or export path (file) | PV annotation `pv.kubernetes.io/provisioned-by: pure-csi` |
| Dell PowerFlex | `csi-vxflexos.dellemc.com` | Volume ID from array | Standard static provisioning |
| Dell PowerStore | `csi-powerstore.dellemc.com` | `<volume-id>/<globalID>/<protocol>` | Supports block (scsi) and file (nfs) |
| Dell PowerMax | `csi-powermax.dellemc.com` | Array-specific volume handle | Standard static provisioning |
| Hitachi Vantara | `hspc.csi.hitachi.com` | LDEV-based handle | Standard static provisioning via HSPC |

### Security, Risks, and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| CSI driver doesn't support import | Low (validated per vendor) | High | Validate StorageClass config at plan time |
| Volume name resolution fails | Medium | Medium | Clear error on step; user can fall back to XCOPY |
| Clone is slow for large volumes | Medium | Low | User sees "Pending" PVC; no timeout pressure |
| CSI driver version too old | Low | High | Document minimum CSI driver versions |
| Controller REST calls to array | Low | Medium | Read-only calls; consider resolver pod for isolation |

The security posture improves compared to the XCOPY populator: the CSI driver already has
the necessary RBAC and storage credentials. The controller only needs read-only access to
the array for volume name resolution, versus the populator which needs read-write access
for cloning, mapping, and unmapping.

## Design Details

### Affected Components

| Component | Change | Scope |
|-----------|--------|-------|
| `forklift-api` (`mapping.go`) | Add `CsiImportPluginConfig` type | Small |
| `vsphere/builder.go` | New branch in `PopulatorVolumes` | Medium |
| `migration.go` | Progress tracking for CSI import PVCs | Small — reuse Shift/PVC-bound pattern |
| `validation.go` | CSI import validation rules | Small |
| Volume resolution (refactor) | Extract `StorageResolver`/`ResolvePVToLUN` into shared `pkg/storage/resolver/` | Medium — shared module, no new logic |
| UI (forklift-console-plugin) | New option in StorageMap offload config | Small |
| Documentation | User guide for CSI import setup | Medium |

### What This Eliminates

Compared to the XCOPY populator path, CSI import removes:

- Populator pod (per disk)
- VSphereXcopyVolumePopulator CR
- Prometheus metrics server
- Host mapping/unmapping and iGroup management
- VIB/SSH fallback path
- Populator service account and RBAC
- Secret merging

### Comparison with Existing Patterns

| Aspect | XCOPY Populator | NetApp Shift | CSI Import (proposed) |
|--------|----------------|-------------|----------------------|
| Copy mechanism | Vendor REST API in populator pod | CSI driver (Trident) reads NFS share | CSI driver imports/clones volume |
| PVC population | DataSourceRef → Populator CR | Direct PVC, CSI driver populates | Direct PVC with import annotation |
| Progress granularity | 0-100% via Prometheus | Binary (Pending → Bound) | Binary (Pending → Bound) |
| Disk types | VVol, RDM, VMDK | NFS-backed VMDK | VVol, RDM |
| Pod overhead | 1 populator pod per disk | None | None |
| Array credentials | Secret mounted in populator pod | Not needed (NFS) | Secret used by controller for name resolution only |

### Test Plan

- Unit tests for `CsiImportPluginConfig` validation and PVC annotation generation
- Unit tests for the shared volume resolution module
- Integration test: create PVC with HPE import annotation against a mock CSI driver
- E2E test: full migration of a VVol-backed VM using HPE CSI driver import on a real
  Primera/3PAR/Alletra cluster
- Regression: existing XCOPY populator path must remain unaffected

### Upgrade / Downgrade Strategy

The `CsiImportPluginConfig` is a new optional field on `OffloadPlugin`. Existing
StorageMap resources are unaffected — they continue using `VSphereXcopyPluginConfig`.
No migration is required. On downgrade, any StorageMap with `CsiImportPluginConfig`
will have the field ignored by the older controller, and those mappings will fall back
to standard (non-offload) migration.

## Implementation History

- 2026-05-14: Initial proposal created (provisional)

## Drawbacks

- **Progress granularity loss** — users lose per-disk copy percentage, seeing only
  "pending" vs "complete". For large volumes this may be perceived as a UX regression.
- **Controller blast radius** — running volume resolution REST calls in the controller
  adds a new failure mode to the controller process.
- **Vendor dependency** — the CSI import feature must be enabled and correctly configured
  in the CSI driver's StorageClass, adding a prerequisite for the user.

## Alternatives

1. **Keep the populator for all vendors** — no code change, but maintenance burden grows
   with each new vendor and firmware version.
2. **Lightweight resolver pod + PVC creation** — run resolution in a short-lived pod
   instead of the controller. Adds pod overhead but isolates the controller from array
   REST calls.
3. **Static PV provisioning only** — generate PV + PVC objects for all vendors instead
   of using annotation-based import. Simpler to implement uniformly but loses the
   elegance of annotation-based import for vendors that support it.

## Infrastructure Needed

- Access to an HPE Primera/3PAR/Alletra cluster with CSI driver 1.2.0+ for E2E testing
- StorageClass configured with `allowOverrides: importVolAsClone`
- vSphere environment with VVol-backed VMs on the HPE array
