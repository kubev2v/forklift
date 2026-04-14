---
title: bridge-vm-concerns-to-plan-readiness
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-04-13
last-updated: 2026-04-13
status: provisional
see-also:
  - "/enhancements/allow-missing-sources.md"
---

# Bridge VM Inventory Concerns to Plan Readiness

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Inventory VM concerns evaluated by Rego policies and plan validation conditions
evaluated by Go code are two disconnected systems. A VM can have Critical
inventory concerns (such as passthrough devices, independent disks, or
insufficient free space for conversion) while the migration plan is marked
`Ready` and will execute -- only to fail at runtime.

This enhancement bridges the gap by making plan validation read each VM's
inventory concerns and block the plan when any concern has `category =
"Critical"`. It also reclassifies inventory concerns that are not true
migration blockers from Critical to Warning.

## Motivation

Users see Critical concerns on VMs in the UI and CLI (`kubectl-mtv`) but are
confused when the migration plan is Ready and starts executing despite those
concerns. The migration then fails at runtime with an error that could have
been caught during validation.

The root cause is an architectural disconnect: Rego policies evaluate VM
compatibility and attach concerns to the inventory, but plan validation in
`validateVM()` performs its own independent checks in Go code without reading
inventory concerns. The two systems have gaps where Critical concerns exist
in the inventory (e.g., independent disks, passthrough devices) but are
invisible to plan validation.

### Goals

* Make plan validation aware of Critical inventory concerns so that a plan
  with VMs that have Critical concerns is not marked `Ready`.
* Reclassify inventory concerns that are not true migration blockers from
  Critical to Warning.

### Non-Goals

* Changing how Rego policies are evaluated or how inventory concerns are
  stored -- the policy agent and inventory model remain unchanged.
* Adding new Rego policies -- existing Critical concerns already cover the
  checks that are missing from plan validation.
* Modifying the `HasBlockerCondition()` logic or the plan readiness gate --
  the mechanism is correct, only its inputs change.

## Proposal

### User Stories

#### Story 1

As a migration administrator, I create a plan that includes a VM with an
independent-mode disk in VMware. Today the plan shows as Ready and I start
the migration, which fails during VDDK data transfer because independent disks
cannot be transferred. After this enhancement, the plan will not be Ready and
will show a condition explaining that the VM has critical inventory concerns.

#### Story 2

As a migration administrator, I create a plan that includes a VM with an RDM
disk. Today the inventory shows a Critical concern on that VM, but the plan is
Ready. This is doubly confusing because Forklift now supports RDM migration
via the `RDMAsLun` option. After this enhancement, the RDM concern is
reclassified to Warning (reflecting that RDM is supported), so the plan
correctly shows as Ready with no misleading Critical flag on the VM.

### Implementation Details

#### Part A: Bridge Critical inventory concerns into plan validation

A new condition type `VMCriticalConcerns` is added to `validateVM()`. During
the per-VM loop, after loading the inventory VM, the code checks all inventory
concerns. Any concern with `category == "Critical"` causes the VM to be added
to the condition's `Items` list. If the list is non-empty after the loop, the
condition is set on `plan.Status` with `CategoryCritical`, which blocks plan
readiness via `HasBlockerCondition()`.

To access concerns generically across provider VM types, a `ConcernHolder`
interface is added:

```go
type ConcernHolder interface {
    GetConcerns() []Concern
}
```

All provider web VM types (vsphere, ovirt, openstack, ovfbase, hyperv)
implement this interface. In `validateVM()`, the inventory VM (returned as
`interface{}`) is type-asserted to `ConcernHolder`.

This replaces the existing OVA-specific concern check (which matched a single
concern by ID). The `ova.source.unsupported` concern has `category = "Warning"`
in Rego, so it is not affected by the generic Critical check.

#### Part B: Reclassify non-blocking concerns to Warning

The following Critical concerns are reclassified to Warning because they do
not actually block migration:

| Concern ID | Provider | Reason |
|---|---|---|
| `vmware.disk.rdm.detected` | VMware | RDM is now supported via `RDMAsLun` at plan and per-VM level |
| `ovirt.vm.status_invalid` | oVirt | Assessment says "may fail"; oVirt PowerState validator is a no-op |
| `openstack.vm.status.invalid` | OpenStack | Assessment says "may fail"; OpenStack PowerState validator is a no-op |

The `vmware.disk.rdm.detected` assessment text is also updated to reflect
current RDM support.

### Security, Risks, and Mitigations

**Risk: Plans that were previously Ready may now be blocked.**
VMs with Critical inventory concerns (e.g., passthrough devices, independent
disks) will now prevent the plan from being Ready. This is the intended
behavior -- these migrations would have failed at runtime anyway. Users who
have been working around runtime failures will need to resolve the concerns
or remove the affected VMs from the plan.

**Risk: Rego policy changes affect plan readiness.**
Any future Rego policy that introduces a new Critical concern will
automatically block plans containing affected VMs. This is by design -- Rego
policies are the domain-specific source of truth for VM compatibility. Policy
authors should be aware that Critical means "blocks migration."

## Design Details

### Architecture

```
                  Rego Policies
                       │
                       ▼
              ┌─────────────────┐
              │  Inventory VM   │
              │  .Concerns[]    │
              │  (Critical,     │
              │   Warning,      │
              │   Information)  │
              └────────┬────────┘
                       │  Part A: bridge
                       ▼
              ┌─────────────────┐
              │  validateVM()   │──── existing Go checks (mappings,
              │                 │     power state, templates, etc.)
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ plan.Status     │
              │ .Conditions[]   │
              │ (CategoryCritical│
              │  blocks Ready)  │
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │ HasBlockerCond? │──► Ready or Not Ready
              └─────────────────┘
```

### Files Changed

| File | Change |
|---|---|
| `pkg/controller/provider/model/base/model.go` | Add `ConcernHolder` interface |
| `pkg/controller/provider/web/*/vm.go` (5 providers) | Add `GetConcerns()` method |
| `pkg/controller/plan/validation.go` | Add `VMCriticalConcerns` const and bridge logic |
| `validation/policies/.../vmware/rdm_disk.rego` | Critical to Warning; update assessment |
| `validation/policies/.../ovirt/vm_status.rego` | Critical to Warning |
| `validation/policies/.../openstack/vm_status.rego` | Critical to Warning |

### Inventory Concerns That Remain Critical

These concerns are true migration blockers and will now correctly block plan
readiness via the bridge:

| Concern ID | Provider | Why it blocks |
|---|---|---|
| `vmware.disk_mode.independent` | VMware | VDDK cannot transfer independent disks |
| `vmware.passthrough_device.detected` | VMware | Passthrough devices cannot be migrated |
| `vmware.datastore.missing` | VMware | No datastore means no disk to copy |
| `vmware.guestDisks.freespace` | VMware | Insufficient free space fails virt-v2v conversion |
| `vmware.disk.capacity.invalid` | VMware | Zero-size disk fails PVC creation |
| `ovirt.disk.illegal_or_locked_status` | oVirt | Illegal/locked disk fails transfer |
| `ovirt.disk.storage_type.unsupported` | oVirt | Unsupported storage type fails transfer |
| `ovirt.disk.illegal_images.detected` | oVirt | Illegal disk images fail transfer |
| `ovirt.disk.capacity.invalid` | oVirt | Zero-size disk fails PVC creation |
| `openstack.disk.status.unsupported` | OpenStack | Unsupported disk status fails transfer |
| `openstack.disk.capacity.invalid` | OpenStack | Zero-size volume fails PVC creation |
| `ova.disk.capacity.invalid` | OVA | Zero-size disk fails PVC creation |
| `hyperv.disk.capacity.invalid` | Hyper-V | Zero-size disk fails PVC creation |

### Test Plan

Unit tests verify:
- A VM with a Critical inventory concern blocks plan readiness.
- A VM with only Warning/Information concerns does not block plan readiness.
- The `VMCriticalConcerns` condition lists affected VMs and concern labels.
- Reclassified concerns (RDM, oVirt status, OpenStack status) no longer block.

### Upgrade / Downgrade Strategy

No migration of existing resources is required. Plans that were previously
Ready but contained VMs with Critical inventory concerns will become Not Ready
after upgrade. This is the correct behavior -- those migrations would have
failed at runtime.

On downgrade, the bridge is removed and behavior reverts to the previous
state where inventory concerns do not affect plan readiness.

## Implementation History

* 2026-04-13 - Enhancement proposed.

## Drawbacks

Plans that previously appeared Ready (but would fail at runtime) will now
show as Not Ready. Users may perceive this as a regression until they
understand that the plan was never truly safe to execute.

## Alternatives

1. **Duplicate Rego logic in Go**: Instead of bridging concerns, replicate
   every Rego check in Go validators. This is the current (implicit) approach
   and leads to drift -- Rego and Go checks diverge over time.

2. **Reclassify all non-blocking concerns to Warning without bridging**: This
   hides real problems from the inventory UI. Concerns like independent disks
   and passthrough devices genuinely block migration and should remain Critical.

3. **Per-concern bridging by ID**: Instead of bridging all Critical concerns,
   explicitly list which concern IDs to check. More granular but fragile --
   every new Rego Critical concern would need a corresponding Go change.
