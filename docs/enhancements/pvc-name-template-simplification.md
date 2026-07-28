# PVC Name Template Simplification

## Motivation

The PVC naming logic had diverged into two separate code paths in the vSphere builder:

1. `setColdMigrationDefaultPVCName` — used for cold-migration populators and CSI-import PVCs, always applied a template with a hardcoded fallback (`trunc 4` plan/VM names).
2. `setPVCNameFromTemplate` — used for warm-migration DataVolumes and NetApp Shift PVCs, returned a no-op when no template was configured.

This created several problems:
- Inconsistent naming between cold and warm migration PVCs
- Per-provider defaults (`{{.SourcePVCName}}` for OCP, `trunc 4` for vSphere) made behavior unpredictable
- The old `trunc 4` default was overly short and produced cryptic PVC names
- No `.VmId` variable to replicate legacy `planName-vmId` naming
- EC2 provider lacked PVC name template support entirely

## Changes

### Optional Plan Template with Runtime Defaults

`pvcNameTemplate` on the Plan is optional and empty by default (no CRD default, not persisted by the controller).

Resolution order:

1. VM-level `pvcNameTemplate` (if set)
2. Plan-level `pvcNameTemplate` (if set)
3. ForkliftController global:
   - Non-OCP: `controller_pvc_name_template` (`PVC_NAME_TEMPLATE`)
   - OCP: `controller_ocp_pvc_name_template` (`OCP_PVC_NAME_TEMPLATE`)
4. Hardcoded default:
   - Non-OCP: `{{trunc 15 .PlanName}}-{{trunc 15 .TargetVmName}}-disk-{{.DiskIndex}}`
   - OCP: `{{.SourcePVCName}}`

`pvcNameTemplateUseGenerateName` is optional (`*bool`). When unset: `false` for OCP, `true` for other providers. That pairs with the OCP default template so source PVC names are kept as exact Names without a special-case bypass.

The non-OCP global never applies to OCP sources, and the OCP global never applies to non-OCP sources. OCP keeps distinct default naming via a different hardcoded template and UseGenerateName default.

### New `.VmId` Template Variable

A `.VmId` field is now available in all provider template data structs, populated with the source VM's provider identifier. Users can write custom templates to replicate the legacy naming:

```
{{.PlanName}}-{{.VmId}}
```

### Uses `.TargetVmName` (DNS1123-safe)

The default template uses `.TargetVmName` instead of `.VmName` because it is already sanitized for Kubernetes (DNS1123 compliant).

### EC2 Provider Support

The EC2 provider now supports PVC name templates with EC2-specific variables:
- `.VolumeID` — original EBS volume ID
- `.SnapshotID` — snapshot ID used to create the volume

### All Providers Support Templates

PVC name templates are now applied universally across all providers:

- **vSphere**: Template applied in the builder's `DataVolumes()` and `PopulatorVolumes()` methods (provider-specific variables like `.FileName`, `.WinDriveLetter`, `.Shared`).
- **OCP**: Template applied in the builder's `DataVolumes()` method (default `{{.SourcePVCName}}` with UseGenerateName default `false`; OCP-specific variables like `.SourcePVCName`, `.SourcePVCNamespace`).
- **EC2**: Template applied in the builder's `BuildDirectPVC()` method (EC2-specific variables like `.VolumeID`, `.SnapshotID`).
- **HyperV, oVirt (DataVolume path), OVF/OVA**: Template applied centrally in `kubevirt.go` post-hoc after the builder returns DataVolumes. Uses `PVCNameTemplateData` (`.VmName`, `.TargetVmName`, `.PlanName`, `.DiskIndex`, `.VmId`).
- **oVirt (Populator path)**: Template applied in the builder's `persistentVolumeClaimWithSourceRef()` method using the shared `SetPVCNameOnObject` helper.
- **OpenStack (Populator path)**: Template applied in the builder's `persistentVolumeClaimWithSourceRef()` method using the shared `SetPVCNameOnObject` helper.

The centralized approach in `kubevirt.go` checks each returned DataVolume: if the builder hasn't set `Name` or `GenerateName`, the template is applied post-hoc using the slice index as `DiskIndex`.

### Validation

Empty Plan/VM templates are valid. The controller validates the *effective* resolved template (including ForkliftController globals and hardcoded defaults) so a bad global setting fails plans early.

### Length Budget

PVC names are reused in derived resource names (scratch DataVolumes, convert jobs, etc.). The worst case adds 17 characters (`scratch-dv-{pvcName}-xxxxx`). With `UseGenerateName=true` (non-OCP default), the template output should be ≤ 40 characters to keep all derived names under the 63-char DNS1123 limit. The default template's `trunc 15` ensures: 15 + 1 + 15 + 6 + 2 = 39 characters worst case.

## Breaking Changes

- **vSphere**: Default PVC names change from `{4-char-plan}-{4-char-vmname}-disk-{index}-{random}` to `{15-char-plan}-{15-char-targetvmname}-disk-{index}-{random}`
- **vSphere**: Warm migration DataVolume names now also get the default template applied (previously no template was used when none was configured)
- **OCP**: `pvcNameTemplatePreserveSource` is removed. OCP default is `{{.SourcePVCName}}` with `pvcNameTemplateUseGenerateName` defaulting to `false` when unset. To get trunc-15 style names on OCP, set Plan `pvcNameTemplate` (and optionally `pvcNameTemplateUseGenerateName: true`) or `controller_ocp_pvc_name_template` explicitly.
- **HyperV/oVirt/OVF/OVA**: PVC names change from `{planName}-{vmId}-{random}` (hardcoded in `kubevirt.go`) to template-based names `{15-char-plan}-{15-char-targetvmname}-disk-{index}-{random}`
- **oVirt (Populator)**: PVC names change from `{diskAttachmentID}-{random}` to template-based names
- **OpenStack (Populator)**: PVC names change from `{imageID}-{random}` to template-based names
- Existing automations that parse or match PVC names by pattern (e.g., regex, label selectors keyed on name prefix) may break

## Migration Guide

- **Existing in-progress migrations** are not affected (PVCs already created keep their names)
- **New migrations** will use the new naming
- **OCP default behavior** preserves source PVC names via `{{.SourcePVCName}}` with UseGenerateName default `false`
- **Plan field stays empty** unless the user sets a custom template; defaults are applied at runtime
- **Cluster-wide defaults** can be set on ForkliftController via `controller_pvc_name_template` / `controller_ocp_pvc_name_template`
- **To restore old vSphere behavior**, set:
  ```yaml
  pvcNameTemplate: "{{trunc 4 .PlanName}}-{{trunc 4 .TargetVmName}}-disk-{{.DiskIndex}}"
  ```
- **To replicate legacy planName-vmId naming**, use:
  ```yaml
  pvcNameTemplate: "{{.PlanName}}-{{.VmId}}"
  ```
