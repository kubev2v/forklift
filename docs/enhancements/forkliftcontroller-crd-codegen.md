---
title: forkliftcontroller-crd-codegen
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-06-28
last-updated: 2026-06-28
status: implementable
see-also:
  - "https://redhat.atlassian.net/browse/MTV-5481"
  - "https://redhat.atlassian.net/browse/MTV-5809"
  - "https://redhat.atlassian.net/browse/MTV-5808"
---

# Generate ForkliftController CRD Programmatically

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Replace the hand-crafted `ForkliftController` CRD YAML with a Go struct
(`ForkliftControllerSpec`) annotated with kubebuilder markers. The CRD is then
auto-generated via `controller-gen`, the same toolchain that already generates
the other 13 CRDs in this project. This eliminates manual YAML maintenance,
removes the need for a Python sync-validation script, and adds strict schema
validation that rejects invalid field values at admission time.

## Motivation

The `ForkliftController` CRD is the only CRD in the project that is maintained
by hand. The operator uses the Ansible SDK layout, and since the Ansible
operator framework does not generate CRDs from Go types, the schema was written
and maintained as raw YAML. Over time this has led to:

1. **Manual sync burden** -- every new field requires edits to the CRD YAML,
   `defaults/main.yml`, and `specDescriptors` in the CSV, with a Python script
   to verify they stay in sync.
2. **Missing validation** -- fields like `controller_max_vm_inflight` accept
   negative numbers and arbitrary strings (MTV-5809, MTV-5808) because the
   hand-written schema used `x-kubernetes-int-or-string: true` without
   constraints.
3. **Default drift** -- documented defaults in CRD descriptions disagree with
   the actual Ansible defaults (e.g. controller CPU limit documented as 500m
   but defaulted to 2 cores).
4. **No type safety** -- field names are untyped strings flowing between YAML
   layers with no compile-time checks.

### Goals

- Auto-generate the ForkliftController CRD from Go types via `controller-gen`.
- Add strict schema validation (enums, integer minimums) so the API server
  rejects invalid values at admission.
- Remove `additionalProperties: true` to harden the API -- unknown fields are
  rejected.
- Eliminate the `hack/validate_forklift_controller_crd.py` script and the
  `validate-forklift-controller-crd` Makefile target.
- Maintain full backward compatibility with existing ForkliftController CR
  instances.
- Make the CRD schema the single source of truth for default values via
  `+kubebuilder:default` markers.
- Remove redundant static defaults from `defaults/main.yml`, retaining only
  computed names, image FQIN lookups, and internal Ansible state.

### Non-Goals

- Replacing the Ansible reconciler with Go -- the operator continues to use
  Ansible roles for reconciliation.
- Restructuring the flat spec into nested sub-structs (can be a follow-up).

## Proposal

### Implementation Details

A new file `pkg/apis/forklift/v1beta1/forkliftcontroller.go` defines:

- `ForkliftControllerSpec` -- a flat struct with ~100 fields matching the
  existing CR schema, using `json:"snake_case,omitempty"` tags for wire
  compatibility.
- `ForkliftControllerStatus` -- preserves unknown fields
  (`+kubebuilder:validation:XPreserveUnknownFields`) since status is managed
  by the Ansible operator.
- `ForkliftController` / `ForkliftControllerList` -- root types registered
  with `SchemeBuilder`.

Key validation markers:

| Pattern | Marker |
|---------|--------|
| Boolean-string feature gates | `+kubebuilder:validation:Enum="true";"false"` |
| IntOrString numeric fields | `*intstr.IntOrString` (generates `x-kubernetes-int-or-string: true`) |
| Integer fields with minimum | `*int32` + `+kubebuilder:validation:Minimum=1` |
| Pull policy | `+kubebuilder:validation:Enum=Always;IfNotPresent;Never` |

The existing `make manifests` command (`controller-gen crd paths="./pkg/apis/..."`)
automatically picks up the new types and regenerates the CRD YAML in place.

### Defaults Strategy

All fields with static literal defaults receive `+kubebuilder:default` markers.
The API server applies these at admission time, eliminating `defaults/main.yml`
as the defaulting layer for user-facing fields.

`defaults/main.yml` retains only:

- **Computed names** -- Jinja2 templates deriving service/deployment/secret names
  from `app_name` (e.g. `{{ app_name }}-controller`).
- **Image FQINs** -- resolved from `RELATED_IMAGE_*` env vars injected by OLM
  per release. These cannot be CRD-defaulted because the correct value is not
  known at schema time.
- **Internal state** -- variables like `validation_state: absent` that control
  Ansible role logic.
- **Infrastructure paths** -- `profiler_volume_path`, `inventory_volume_path`.

Fields that do NOT receive CRD defaults:

| Category | Reason |
|----------|--------|
| Container image FQINs (18 fields) | Auto-set by operator from release payload at reconcile time |
| User-supplied optional fields (aap_url, transfer_network, extra_args, etc.) | Empty/nil means "disabled" -- a default would force a value |
| olm_managed | Informational, set externally by OLM subscription |

### User Stories

#### Story 1: Developer adds a new ForkliftController field

A developer adds a new field to `ForkliftControllerSpec` with appropriate
markers and JSON tag, then runs `make manifests`. The CRD is regenerated
automatically. No need to manually edit YAML or run validation scripts.

#### Story 2: User sets invalid concurrency limit

A user attempts to set `controller_max_vm_inflight: -3`. The API server rejects
the request because the field is typed as `intstr.IntOrString` and the value
does not match the schema. Previously this was silently accepted.

### Security, Risks, and Mitigations

**Risk: Breaking existing CRs on upgrade.**
Mitigation: JSON tags use the exact same snake_case field names as the existing
schema. All fields are optional (`omitempty`). Existing CRs pass validation
without modification.

**Risk: Ansible variable injection breaks.**
Mitigation: The Ansible operator SDK passes CR spec fields as extra-vars using
the JSON field names. Since the JSON tags match exactly, no change in behavior.

## Design Details

### Test Plan

1. `make generate && make manifests` succeeds without errors.
2. `go build ./pkg/apis/...` compiles cleanly.
3. Existing ForkliftController sample CR (`operator/config/samples/`) validates
   against the new CRD schema.
4. Invalid values (negative integers, unknown fields) are rejected by the API
   server in an envtest or cluster test.

### Upgrade / Downgrade Strategy

**Upgrade:** The new CRD is a strict subset of the previous schema (same field
names, same types, added validation). Existing CRs that were valid before remain
valid. CRs with previously-accepted invalid values (negative numbers, unknown
fields) will fail validation on next update -- this is intentional.

**CRD defaults and upgrades:** Once CRD defaults are set, the API server
persists them into the stored CR. If a future release changes a default value,
existing CRs retain the old default (it was baked in at creation time). To pick
up new defaults, users must explicitly delete the field from their CR so the new
default is re-applied. This is standard Kubernetes CRD defaulting behavior and
matches how other operators (e.g. KubeVirt, OLM) handle evolving defaults.

**Downgrade:** Rolling back the CRD to the previous hand-crafted version
re-enables `additionalProperties: true` and removes validation constraints.
No data loss occurs.

## Implementation History

- 2026-06-28: Initial implementation (MTV-5481).
- 2026-06-28: Add CRD-level defaults, remove static entries from defaults/main.yml.

## Drawbacks

- The flat struct with ~100 fields is large. A follow-up could introduce nested
  sub-structs (e.g. `FeatureGates`, `ResourceLimits`) but this requires a
  versioned API migration.
- CRD-level validation may surface errors for CRs that previously had invalid
  but tolerated values.

## Alternatives

1. **Keep hand-crafted YAML with better tooling** -- e.g. a Go program that
   reads defaults/main.yml and generates the CRD. Rejected because it
   duplicates what controller-gen already does.
2. **Use a JSON Schema generator** -- e.g. generate from a YAML spec file.
   Rejected because Go types provide compile-time safety and integrate with
   the existing controller-gen pipeline.
