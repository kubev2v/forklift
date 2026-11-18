# MTV-6734: PriorityClass for XCOPY populator pods

## Summary

Add an optional Plan setting for the PriorityClass used by vSphere XCOPY populator pods. When the setting is omitted, Forklift leaves the pod's priority class unset and existing cluster defaulting applies.

## Implementation

- Add `spec.offloadPopulatorPriorityClassName` to the Plan API. The referenced cluster PriorityClass must already exist; Forklift does not create or default it.
- The referenced cluster PriorityClass must exist when the XCOPY pod is created; Forklift does not create or default it.
- Store a configured value in the `priorityClassName` field of the `VSphereXcopyVolumePopulator` spec.
- Apply that spec value to the pod's `priorityClassName` only when creating vSphere XCOPY populator pods.
- If the PriorityClass is missing at pod creation, Kubernetes rejects the pod; the populator controller records a warning event on the PVC and retries. Do not fall back to an unset class.
- Keep preemption or eviction detection, retries beyond existing behavior, other populator types, CDI importer pods, and UI changes out of scope.

## Example

```yaml
spec:
  offloadPopulatorPriorityClassName: xcopy-migration
```

The `xcopy-migration` PriorityClass must be created by the cluster administrator. A higher priority can help the scheduler place XCOPY pods ahead of lower-priority pending pods, but does not prevent kubelet eviction under node pressure. Descheduler policies must also be reviewed because they can remove pods regardless of their PriorityClass.

## Validation

- Verify a configured value reaches an XCOPY pod and an omitted value leaves `priorityClassName` unset.
- Verify other populator kinds ignore this setting.
- Verify pod creation failures remain visible through the existing PVC warning event and reconciliation path.
- Regenerate the Plan and XCOPY populator CRDs, then run the relevant vSphere builder and populator controller tests.

## Assumptions

- The option affects newly created XCOPY pods only; it does not patch existing pods.
- Preemption and eviction cause detection is not part of this change.
