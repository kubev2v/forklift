---
title: multidisk-ova
authors:
  - "@mansam"
reviewers:
  - "@mnecas"
approvers:
  - "@mnecas"
creation-date: 2024-11-26
last-updated: 2024-11-26
status: implementable
---

# Storage Mapping for multi-disk OVA Imports

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Summary

In the current release of Forklift it is possible to import OVA appliances
that have been exported from a vSphere environment.  However, all of
an appliance's disks are mapped to the same storage class, and it is
not possible for the end user to indicate that certain disks should be
mapped differently from others. In order to support more use cases, it
should be possible to select map a destination storage class for each
disk of an OVA appliance individually. Forklift does not normally allow
such fine-grained storage mapping because it becomes untenable with a
large number of VMs, but because an OVA plan is expected to involve only
a small number of VMs it should be acceptable.

### Example StorageMap

```yaml
spec:
  map:
    - destination:
        storageClass: fast
      source:
        id: vm1-disk1
    - destination:
        storageClass: slow
      source:
        id: vm1-disk2
    - destination:
        storageClass: fast
      source:
        id: vm2-disk1
    - destination:
        storageClass: default
      source:
        id: default
  provider:
    destination:
      name: host
      namespace: default
    source:
      name: ova
      namespace: default
```

### Goals

* Forklift will enable end users to individually map each disk of a
  vSphere OVA appliance to a destination storage class of their choice.

### Non-Goals

* Forklift will not allow fine-grained storage mapping for any VM
  providers other than OVA.

## Proposal

The OVA inventory adapter should be modified to surface each disk from
each OVA appliance as its own source storage class. The OVA VM builder
would then be modified to respect these mappings when building the
DataVolumes rather than assuming only a single mapping.

### Security, Risks, and Mitigations

No new security risks are introduced by permitting disks to be mapped
individually.

## Design Details

### Test Plan

Existing tests for the OVA provider should be updated to include
individual disk mappings. No additional tests should be necessary.

### Upgrade / Downgrade Strategy

Permitting OVA disks to be mapped individually does not require any
changes to the update or downgrade path of Forklift itself, although
Plans created with this feature would not be compatible with a downgraded
version of Forklift as the previous version would not recognize the individual
disk mappings.

### Open Questions

Should we continue, to support a default mapping or require each disk to
be specifically mapped?  Allowing a default would preserve the current
ease of use for simple cases, but may make it easier to make mistakes
since plan validation would not be able to ensure that each of the disks
had been mapped as desired.

## Implementation History

* 11/26/2024 - Enhancement submitted.
