---
title: storagemap-offload-plugin-order-independence
authors:
  - "@amitosw15"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-08-05
last-updated: 2026-08-05
status: implementable
see-also:
  - "/enhancements/vsphere-copy-offload-populator.md"
---

# StorageMap Offload Plugin Order Independence

Fixes [MTV-6322](https://issues.redhat.com/browse/MTV-6322): when `spec.map` in a
`StorageMap` lists the same source datastore in more than one entry (e.g. one
entry configured with `csiVolumeImport`, another with `vsphereXcopyConfig`),
which plugin actually governs a disk on that datastore currently depends on
the *order* of the entries in the list, rather than on the disk itself.

## Implementation Status

**Phase 1 implemented** (this PR): a narrower slice of the proposal below —
unifying RDM/VVol resolution between `CsiImportPVCs` and `PopulatorVolumes`
via a shared, kind-aware vendor search (`resolvePassthroughDisksByVendor` /
`findStorageMapEntriesForVendor` in `rdm_storage.go`). This fixes the
reported MTV-6322 order-dependency for RDM/VVol disks without changing
`buildDatastoreMap`'s value type, since it was traced and confirmed that
plain VMDK disks never depend on `buildDatastoreMap`'s last-write-wins in
practice (see Proposal §3 below, and the code comments at the relevant call
sites for the full trace).

**Not yet implemented / deferred**, still tracked by this doc:
- The `buildDatastoreMap` multi-value redesign and unifying `DataVolumes`/
  `NetAppShiftPVCs` behind the same resolver (§1-3 below).
- Proactive `StorageMap` validation for ambiguous configs (§4 below).
- Hardening `disambiguateRDMByNAA`'s tie-break, which today silently keeps
  the first candidate on an exact NAA-prefix-length tie instead of erroring.
- A live "ask the array" `MatchesDevice`-style tie-breaker as the fallback
  when that tie occurs, instead of a bare error. This is a real, better idea
  than erroring — found in `amitosw15/forklift`'s archived
  `archive/matches-device-unused` branch, commit
  `565ab90ffac029f6570c8cdfa3b823b804ceb7ab` ("MTV-5780 | Archive:
  MatchesDevice-based same-array adapter selection"): a per-vendor
  `storage.ArrayIdentifier.MatchesDevice(deviceName string) (bool, error)`,
  fast-rejecting by NAA prefix and falling through to a live array query,
  implemented for all 9 xcopy vendors in the populator binary. That commit's
  message explains why it was archived rather than merged: "per-LUN
  `MatchesDevice` calls do not scale (one REST call to the destination array
  per LUN on the host)" and "the answer depended on the source datastore's
  backing device, which breaks down for RDM disks" — it was superseded in
  its original context (ESXi host-adapter selection for XCOPY) by listing
  the destination array's own SAN target ports once instead. Reusing it here
  as a *last-resort-only* fallback (invoked only when the cheap NAA-prefix
  comparison already ties, not once per LUN per host) likely sidesteps the
  scaling objection, but that must be explicitly re-justified when this
  follow-up is implemented, not assumed away.

## Open Questions

1. Should ambiguous/unresolvable `StorageMap` configs be rejected at
   admission time (webhook), at reconcile time (status condition), or both?
   This doc assumes reconcile-time validation (consistent with how the rest
   of `StorageMap` validation works today), but a webhook would catch it
   earlier. Still open — not addressed by phase 1.
2. ~~`findStorageMapEntriesForVendor` today only inspects
   `VSphereXcopyPluginConfig`. Extending it to also match
   `CsiVolumeImport.StorageVendorProduct` is required for this fix — need a
   reviewer to confirm there's no existing reason it was scoped to xcopy
   only.~~ Resolved by phase 1: extended to take a `PassthroughOffloadKind`
   selector (`OffloadKindXcopy` / `OffloadKindCsi`); no existing reason found
   for the original xcopy-only scoping — it simply predated CSI import.

## Summary

`StorageMap.spec.map` is a list of `StoragePair` entries, each optionally
carrying an `OffloadPlugin` (either `vsphereXcopyConfig` or
`csiVolumeImport`, mutually exclusive in practice). Four `Builder` methods in
`pkg/controller/plan/adapter/vsphere/builder.go` — `DataVolumes`,
`CsiImportPVCs`, `NetAppShiftPVCs`, and `PopulatorVolumes` — each need to
resolve, per source disk, which `StoragePair` entry applies. Three of the
four collapse `spec.map` into a `map[string]*api.StoragePair` keyed only by
source datastore ID (`buildDatastoreMap`), silently overwriting earlier
entries that share a datastore ID with later ones. The fourth
(`PopulatorVolumes`) instead walks the raw, un-deduplicated list and
performs its own RDM/NAA vendor disambiguation. When a StorageMap
legitimately (or accidentally) lists the same source datastore twice with
different offload plugins, these two resolution strategies disagree about
which plugin applies to a given disk, and the outcome flips depending on
list order.

This proposal unifies resolution behind a single function used by all four
call sites, disambiguating by **disk type and storage vendor identity**
instead of **list position**, so behavior no longer depends on the order
entries appear in `spec.map`.

## Motivation

VMs being migrated can have a mix of VMDK disks and RDM/VVol disks. CSI
volume import can only handle RDM/VVol disks (it clones the source array
volume directly via the CSI driver, which has no notion of a VMDK file); it
cannot import a VMDK. vSphere XCOPY, by contrast, can handle both. This
means a single source datastore can legitimately need more than one
`StoragePair` entry: one telling the controller how to CSI-import the RDMs
on it, another telling it how to XCOPY-populate the VMDKs (or RDMs from a
different array that happens to be exposed through the same vSphere
datastore object). Today, only `PopulatorVolumes` was written to expect
multiple entries per datastore; the other three call sites assume one entry
per datastore ID and quietly pick whichever entry happened to be listed
last.

### Goals

- Make the outcome of a migration independent of the order of entries in
  `StorageMap.spec.map`.
- Support datastores that legitimately map to more than one offload plugin
  (mixed VMDK/RDM disks, or RDMs from different arrays sharing a vSphere
  datastore), consistently across all four builder methods.
- Reuse the existing NAA-based vendor disambiguation logic
  (`pkg/controller/plan/adapter/vsphere/rdm_storage.go`) rather than
  inventing a second mechanism.
- Surface genuinely unresolvable configurations as clear errors instead of
  silently picking one of the candidates.

### Non-Goals

- Changing the `OffloadPlugin` CRD shape (still exactly one of
  `vsphereXcopyConfig` / `csiVolumeImport` per `StoragePair`).
- Supporting providers other than vSphere (the bug and this fix are scoped
  to `pkg/controller/plan/adapter/vsphere`).
- Building a general multi-value StorageMap UI/UX; this is a controller-side
  correctness fix.

## Proposal

### Implementation Details/Notes/Constraints

**1. `buildDatastoreMap` keeps all candidates for a datastore ID, not just
the last one.**

```go
// today: map[string]*api.StoragePair, last entry for a given ds.ID wins
func (r *Builder) buildDatastoreMap() (map[string][]*api.StoragePair, error) {
    dsMap := make(map[string][]*api.StoragePair)
    for i := range dsMapIn {
        mapped := &dsMapIn[i]
        ds, err := ... // resolve mapped.Source, as today
        dsMap[ds.ID] = append(dsMap[ds.ID], mapped)
    }
    return dsMap, nil
}
```

**2. A single shared per-disk resolver replaces the direct map lookups and
the bespoke RDM pre-pass in `PopulatorVolumes`.**

```go
// resolveStorageMapping picks the StoragePair that governs a given disk,
// disambiguating when more than one entry targets the disk's source datastore.
func (r *Builder) resolveStorageMapping(
    disk model.Disk,
    candidates []*api.StoragePair,
    naaPrefixes []naaVendorEntry,
) (*api.StoragePair, error)
```

Resolution logic:

- **0 candidates** — disk isn't mapped; caller skips it (unchanged today).
- **1 candidate** — return it directly. This is the fast path and covers the
  overwhelming majority of real `StorageMap`s, which list a datastore once.
  No behavior change for this case.
- **>1 candidates** (only possible when the same source datastore
  intentionally or accidentally appears more than once in `spec.map`):
  - **VMDK disk** (`!disk.RDM`): CSI import can never apply, so drop any
    candidate that is CSI-only (`CsiVolumeImport != nil &&
    VSphereXcopyPluginConfig == nil`). If exactly one candidate survives,
    use it. If more than one remains (e.g. two xcopy entries for the same
    datastore), the config is genuinely ambiguous — return an error rather
    than guessing.
  - **RDM disk**: extract the vendor from the RDM's NAA identifier
    (`vendorFromNAA`, already implemented), then filter candidates whose
    configured `StorageVendorProduct` matches — checking **both**
    `CsiVolumeImport.StorageVendorProduct` and
    `VSphereXcopyPluginConfig.StorageVendorProduct`
    (`findStorageMapEntriesForVendor` currently only checks the latter and
    needs extending). If still ambiguous, fall back to the existing
    `disambiguateRDMByNAA` array-serial matching, which already works
    against either plugin type since it only resolves `candidate.Source`.

**3. Update all four call sites** (`DataVolumes`, `CsiImportPVCs`,
`NetAppShiftPVCs`, `PopulatorVolumes`) to call `resolveStorageMapping` per
disk instead of a direct `dsMap[disk.Datastore.ID]` lookup or (for
`PopulatorVolumes`) its own RDM pre-pass. This deletes the divergent,
`PopulatorVolumes`-only NAA matching block (builder.go, current pre-pass
around the `rdmMapped` construction) in favor of one path all four methods
share.

**4. Add proactive `StorageMap` validation** so ambiguous configs are caught
at `StorageMap` reconcile time instead of surfacing as a migration failure.
Group `spec.map` entries by source ID; for any datastore listed more than
once, verify the entries are actually disambiguable (each RDM-capable entry
has a distinct storage vendor; at most one non-CSI entry exists as the VMDK
fallback). If not, set `StorageMap` `Ready=False` with a message identifying
the conflicting entries.

### User Stories

#### Story 1

As a migration administrator, I map a datastore that holds both VMDK system
disks and RDM data disks backed by a storage array. I configure two entries
for that datastore in my `StorageMap` — one with `vsphereXcopyConfig` for
the VMDKs, one with `csiVolumeImport` for the RDMs — and the migration
succeeds regardless of which entry I listed first.

#### Story 2

As a migration administrator, I accidentally list the same datastore twice
with conflicting, non-disambiguable offload configs (e.g. two xcopy entries
with different vendors that can't be told apart from the VMDK side). Instead
of a migration silently doing the wrong thing, my `StorageMap` shows
`Ready=False` with a message telling me which entries conflict.

### Security, Risks, and Mitigations

No new external inputs or trust boundaries are introduced; this changes
internal resolution logic only. The main risk is behavioral regression for
existing single-entry-per-datastore StorageMaps — mitigated by the resolver
short-circuiting to the current behavior whenever there's exactly one
candidate (the common case).

## Design Details

### Test Plan

- Unit tests for `resolveStorageMapping` covering: single candidate (no
  change), multiple candidates with a VMDK disk (CSI-only candidates
  dropped), multiple candidates with an RDM disk disambiguated by vendor,
  multiple candidates with an RDM disk disambiguated by NAA array-serial
  match, and the unresolvable case (error returned).
- Regression test reproducing the exact MTV-6322 scenario (same source
  datastore listed twice, once `csiVolumeImport` first / once
  `vsphereXcopyConfig` first) asserting identical outcome for both orders.
- Unit tests for the extended `findStorageMapEntriesForVendor` covering CSI
  entries.
- Reconcile-level test for the new `StorageMap` ambiguous-config validation.

### Upgrade / Downgrade Strategy

No CRD or on-disk format changes. Existing `StorageMap`s with one entry per
datastore are unaffected. `StorageMap`s that happen to rely on today's
last-write-wins behavior (undocumented and unsupported) may see their
resolved plugin change after upgrade if the config would now be reclassified
as ambiguous. No downgrade concerns beyond reverting the controller image.

## Implementation History

- 2026-08-05: Initial proposal, drafted while investigating MTV-6322.
- 2026-08-05: Phase 1 landed — RDM/VVol resolution unified between
  `CsiImportPVCs` and `PopulatorVolumes` via shared, kind-aware vendor
  search. See Implementation Status above for what remains.

## Drawbacks

- Adds a small amount of resolution complexity (disk-type + vendor matching)
  to a path that most StorageMaps never exercise, since most datastores are
  listed exactly once.
- The proactive validation piece requires deciding where StorageMap
  validation currently lives and extending it, which is additional surface
  area beyond the minimal builder-side fix.

## Alternatives

- **Reject duplicate source datastore IDs in `spec.map` outright.** Simpler,
  but forecloses the legitimate mixed VMDK/RDM-on-one-datastore case that
  motivated allowing more than one entry per datastore in the first place.
- **Keep last-write-wins but make it explicit/documented.** Rejected: still
  order-dependent, just now expected to be memorized by users, which is the
  exact fragility the original bug report objects to.
- **Fix only `buildDatastoreMap`'s callers to match `PopulatorVolumes`'s
  existing RDM logic, without unifying into one function.** Rejected as a
  short-term patch: three near-duplicate implementations of NAA
  disambiguation are exactly how the current inconsistency happened in the
  first place.
