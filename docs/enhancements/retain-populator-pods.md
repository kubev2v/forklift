---
title: retain-populator-pods
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-05-18
last-updated: 2026-05-18
status: implementable
---

# Retain Populator Pods for Debugging

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

A new `FEATURE_RETAIN_POPULATOR_PODS` feature flag (default `false`) prevents
the migration controller from deleting populator pods during cleanup. When
enabled, populator pods remain on the cluster after migration completes, fails,
or is canceled, allowing engineers to inspect logs, exec into containers, and
examine pod specs for debugging purposes.

## Motivation

Populator-based migrations (oVirt, OpenStack, vSphere copy-offload) create
short-lived populator pods that transfer data into PVCs. When a migration fails
or produces unexpected results, the migration controller's `cleanup()` function
unconditionally deletes these pods before an engineer can investigate. Because
the pods are gone, the only remaining evidence is whatever the controller
logged, which is often insufficient to diagnose issues such as:

- Network connectivity failures between the populator and the source
- Authentication or certificate errors
- Slow or stalled transfers
- Unexpected pod restarts or OOM kills

Without the pod, `kubectl logs`, `kubectl exec`, and `kubectl describe pod` are
unavailable, making root-cause analysis significantly harder.

### Goals

- Allow operators to retain populator pods on demand via a feature flag on the
  ForkliftController CR.
- Log a message when the flag is active so it is clear that pods are being
  intentionally retained.
- Preserve existing behavior (pods are deleted) when the flag is off.

### Non-Goals

- Retaining PVC' (prime/temporary PVCs) created by the populator machinery.
- Per-plan or per-VM granularity for the retention flag.
- Preventing Kubernetes garbage collection when the owning PVC is deleted (see
  Caveats below).

## Proposal

### User Stories

#### Story 1

As a migration administrator, a populator-based migration from oVirt fails and
I need to inspect the populator pod's logs and environment to understand why.
I set `controller_retain_populator_pods: "true"` on the ForkliftController CR
and `deleteVmOnFailMigration: false` on the Plan (to prevent PVC deletion from
cascading to the pod via Kubernetes GC). I re-run the migration, and after it
fails again the populator pod remains so I can run `kubectl logs` and
`kubectl describe pod` against it.

#### Story 2

As a Forklift developer, I am debugging a new populator implementation and need
to iterate quickly. I enable the retain flag so that each test run leaves the
pod around for inspection, then disable it once the issue is resolved.

### Implementation Details

#### Feature Flag

A new constant and struct field in `pkg/settings/features.go`:

```go
FeatureRetainPopulatorPods = "FEATURE_RETAIN_POPULATOR_PODS"
```

```go
RetainPopulatorPods bool
```

Loaded via `getEnvBool(FeatureRetainPopulatorPods, false)`.

#### Controller Guard

In `pkg/controller/plan/kubevirt.go`, the `DeletePopulatorPods` method
short-circuits when the flag is enabled:

```go
func (r *KubeVirt) DeletePopulatorPods(vm *plan.VMStatus) (err error) {
    if Settings.RetainPopulatorPods {
        r.Log.Info("Retaining populator pods (feature flag enabled).", "vm", vm.String())
        return
    }
    list, err := r.getPopulatorPods()
    for _, object := range list {
        err = r.DeleteObject(&object, vm, "Deleted populator pod.", "pod")
    }
    return
}
```

#### Operator Wiring

- **Ansible default** (`operator/roles/forkliftcontroller/defaults/main.yml`):
  `controller_retain_populator_pods: false`
- **Deployment template** (`deployment-controller.yml.j2`): conditionally sets
  `FEATURE_RETAIN_POPULATOR_PODS=true` when the Ansible variable is true.
- **CRD schema** (`forklift.konveyor.io_forkliftcontrollers.yaml`): new string
  property with enum `["true", "false"]`.
- **CSV descriptors** (upstream/downstream): hidden spec descriptor for the new
  field.

#### Caveats

Populator pods are created with an `OwnerReference` pointing to the PVC:

```go
OwnerReferences: []metav1.OwnerReference{
    {
        APIVersion: "v1",
        Kind:       "PersistentVolumeClaim",
        Name:       pvc.Name,
        UID:        pvc.UID,
    },
},
```

If the PVC itself is deleted -- for example, on the failure path when
`DeleteVmOnFailMigration` is enabled (which is the **default**, `true`) and
the controller calls `DeletePopulatedPVCs` and `DeleteDataVolumes` --
Kubernetes garbage collection will delete the populator pod regardless of this
feature flag. The flag only guards the explicit `DeletePopulatorPods` call in
`cleanup()`.

**To retain populator pods on the failure path, you must also set
`deleteVmOnFailMigration: false` on the Plan (or per-VM).** Otherwise the
default behavior (`deleteVmOnFailMigration: true`) will delete the PVCs,
which cascades to the populator pods via Kubernetes GC.

Summary of when the flag is effective:

| Scenario | `deleteVmOnFailMigration` | Populator pods retained? |
|----------|--------------------------|--------------------------|
| Successful migration | N/A | Yes |
| Canceled migration | N/A | Yes |
| Failed migration | `false` | Yes |
| Failed migration | `true` (default) | **No** -- PVC deletion triggers GC |

### Security, Risks, and Mitigations

**Resource accumulation**: Retained pods consume cluster resources (CPU, memory
reservations, IP addresses). The flag defaults to `false` and is intended for
temporary debugging use only. Administrators should disable it after
investigation is complete.

**No privilege escalation**: The flag only prevents deletion of pods that the
controller already created. It does not grant new access or capabilities.

## Design Details

### Test Plan

- Unit test verifying that `DeletePopulatorPods` returns immediately without
  calling `DeleteObject` when `Settings.RetainPopulatorPods` is `true`.
- Unit test verifying that `DeletePopulatorPods` deletes pods normally when the
  flag is `false`.

### Upgrade / Downgrade Strategy

No migration of existing resources is required. The flag defaults to `false`,
preserving the existing behavior where populator pods are always deleted during
cleanup. Upgrading to a version with this feature has no visible effect unless
the flag is explicitly enabled.

## Implementation History

- 2026-05-18 - Enhancement proposed and implemented.

## Drawbacks

- Retained pods accumulate if operators forget to disable the flag after
  debugging.
- The flag cannot prevent garbage collection of pods whose owning PVC is
  deleted, limiting its usefulness on certain failure paths.

## Alternatives

1. **Per-plan annotation**: Allow a per-plan annotation to retain pods. More
   granular but adds API surface and controller complexity.
2. **TTL-based retention**: Retain pods for a configurable duration, then
   auto-delete. Avoids accumulation but adds a timer mechanism.
3. **Remove OwnerReference**: Drop the PVC OwnerReference from populator pods
   so they survive PVC deletion. This would require manual cleanup in all cases
   and risks orphaned pods.
