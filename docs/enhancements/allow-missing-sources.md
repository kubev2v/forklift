---
title: allow-missing-sources
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-03-22
last-updated: 2026-03-25
status: implemented
---

# Permissive Source Validation in Network and Storage Mappings

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Source references not found in the provider inventory produce a `Warn`
condition instead of a `Critical` one in both `NetworkMap` and `StorageMap`
custom resources. This allows a single shared mapping to remain `Ready` even
when some of its source entries are temporarily or permanently unavailable,
enabling multiple migration plans that each use only a subset of the mapped
sources to proceed independently.

Plan-level validation independently verifies that every VM selected for
migration has its networks and storage present in the provider inventory
(via `Status.Refs`), so missing sources are still caught at migration time.

## Motivation

Organizations managing large virtualization environments commonly have dozens
of networks and datastores. Rather than creating a dedicated mapping for every
migration plan, administrators prefer to define a single NetworkMap and a single
StorageMap that cover all known sources. Individual plans each reference the
same shared mappings but only migrate VMs that use a subset of those sources.

If every source listed in a mapping had to exist in the provider inventory for
the mapping to be marked `Ready`, even one temporarily unavailable source --
because the provider inventory has not yet synced it, or because a datastore
was decommissioned that no current plan needs -- would block the entire mapping
with a `Critical` condition. This would cascade to every plan that references
the mapping, preventing migrations that have nothing to do with the missing
source.

### Goals

* Allow a single NetworkMap or StorageMap to be shared across many migration
  plans without requiring every mapped source to be present in inventory at all
  times.
* Report source `NotFound` validation errors as warnings so the mapping stays
  `Ready` and referencing plans are not blocked.
* Rely on plan-level per-VM validation to catch any missing source that a
  migration actually needs.

### Non-Goals

* Relaxing validation for destination references -- the destination cluster must
  be fully configured before migration.
* Relaxing validation for configuration errors (`NotSet`, `Ambiguous`) -- these
  indicate broken mapping entries, not temporarily missing sources.
* Changing plan-level per-VM validation -- plans independently verify that each
  selected VM has its networks and storage covered by the mapping's resolved
  references (`Status.Refs`).

## Proposal

### User Stories

#### Story 1

As a migration administrator, I create one NetworkMap that covers all 20 source
port groups in my vSphere environment and one StorageMap that covers all 8
datastores. I then create several migration plans, each targeting a different
set of VMs. Some port groups and datastores are used by only one plan. I want
all plans to proceed even if a source that no running plan needs has been
removed from vSphere or is not yet synced into the Forklift inventory.

#### Story 2

As a migration administrator, I am rolling out migrations incrementally. I
pre-configure a mapping with all sources I will eventually need. Early plans
only use a fraction of those sources; the rest will be added to the provider
later. I do not want to wait until every source exists in inventory before I can
start my first migration.

### Implementation Details

The `validateSource` functions in the network and storage map controllers set
`Category: Warn` instead of `Category: Critical` for source refs that produce
a `NotFound` error. Since `Warn` conditions do not block the `Ready` state
(`HasBlockerCondition` only checks for `Critical` and `Error`), the mapping
remains usable.

The following validations remain `Critical`:

| Validation | Reason |
|---|---|
| Source ref with no ID or Name (`NotSet`) | Configuration error -- the entry is incomplete |
| Source ref matching multiple inventory objects (`Ambiguous`) | Configuration error -- the ref must be unique |
| Destination network/storage class not found | Destination must be configured before migration |

Plan-level validation independently verifies that every VM selected for
migration has its networks and storage covered by the mapping's resolved
references in `Status.Refs` (`NetworksMapped` / `StorageMapped`). Only sources
that are successfully resolved against the provider inventory are added to
`Status.Refs`. A VM that needs a source missing from inventory will still
produce a `Critical` condition on the plan.

### Security, Risks, and Mitigations

Permissive mapping validation does not bypass any plan-level safety checks.
The only change is the severity of a mapping condition, not whether the
condition is reported -- missing sources are still visible as warnings in the
mapping status.

## Design Details

### Controller Changes

In `pkg/controller/map/network/validation.go` and
`pkg/controller/map/storage/validation.go`, the `validateSource` method uses
`Warn` category for `NotFound` sources:

```go
mp.Status.SetCondition(libcnd.Condition{
    Type:     SourceStorageNotValid,
    Status:   True,
    Reason:   NotFound,
    Category: Warn,
    Message:  "Source storage not found.",
    Items:    notValid,
})
```

### Examples

#### Shared NetworkMap

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: NetworkMap
metadata:
  name: shared-network-map
  namespace: openshift-mtv
spec:
  provider:
    source:
      name: vsphere-provider
      namespace: openshift-mtv
    destination:
      name: host
      namespace: openshift-mtv
  map:
    - source:
        id: dvportgroup-10
      destination:
        type: pod
    - source:
        id: dvportgroup-20
      destination:
        type: multus
        namespace: openshift-mtv
        name: vlan20-nad
    - source:
        id: dvportgroup-30
      destination:
        type: multus
        namespace: openshift-mtv
        name: vlan30-nad
```

If `dvportgroup-30` is not yet present in the provider inventory, the mapping
will show a warning condition but will still be `Ready`. A plan that only
migrates VMs on `dvportgroup-10` and `dvportgroup-20` will proceed without
issue.

#### Shared StorageMap

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: StorageMap
metadata:
  name: shared-storage-map
  namespace: openshift-mtv
spec:
  provider:
    source:
      name: vsphere-provider
      namespace: openshift-mtv
    destination:
      name: host
      namespace: openshift-mtv
  map:
    - source:
        id: datastore-100
      destination:
        storageClass: ocs-storagecluster-ceph-rbd
    - source:
        id: datastore-200
      destination:
        storageClass: ocs-storagecluster-ceph-rbd
    - source:
        id: datastore-300
      destination:
        storageClass: nfs-csi
```

If `datastore-300` has been removed from vSphere, the mapping stays `Ready`.
Plans migrating VMs that only reside on `datastore-100` or `datastore-200` are
unaffected.

### Test Plan

Unit tests verify:
- `NotFound` sources produce `Warn` and the mapping remains `Ready`.
- `NotSet` and `Ambiguous` remain `Critical` and block `Ready`.

### Upgrade / Downgrade Strategy

No migration of existing resources is required. Plan-level validation continues
to enforce that all sources needed by selected VMs exist in inventory, so no
migration safety is lost.

## Implementation History

* 2026-03-25 - Enhancement implemented.

## Drawbacks

Users who do not review mapping warnings may not notice that sources have
disappeared from their environment. This is mitigated by the fact that warnings
are still reported in the mapping status and that plan-level validation catches
any missing source a migration actually needs.

## Alternatives

1. **Per-entry opt-out**: Allow each mapping entry to be marked as optional.
   More granular but adds complexity to the API.
2. **Plan-level override**: Put the flag on the Plan instead of the mapping.
   This would require plan validation to ignore mapping `Ready` state, which
   couples plan and mapping validation more tightly.
