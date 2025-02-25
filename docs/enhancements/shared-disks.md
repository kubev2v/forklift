---
title: shared-disks
authors:
  - "@mnecas"
reviewers:
  - "@yaacov"
  - "@mansam"
approvers:
  - "@yaacov"
  - "@mansam"
creation-date: 2025-02-27
last-updated: 2025-02-27
status: implementable
---

# Migrate Shared Disks

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Summary

In the current release of Forklift it is possible to import shared disks using cold migration.
This is possible because the virt-v2v is transferring all disks attached to the VM, but because
the shared disks are attached to multiple VMs the shared disks get migrated multiple times and users right now,
need to delete the extra volumes and reattach the disks.

### Goals

* Forklift will support skipping the shared disks during the disk transfer so they won't be migrated multiple times.

### Non-Goals

* Forklift will not allow creating a large plan which would support first transferring shared disk and then all others.
This would require breaking lot of standard flows in Forklift.

## Proposal

This enhancement document proposes an additional parameter to the plan called `migrateSharedDisks`.
This parameter will allow the users to choose if the plan should migrate shared disks or not.
When the `migrateSharedDisks` will be enabled the Forklift will use the "normal" cold migration flow using virt-v2v and labeling
the shared PVC. 
When the `migrateSharedDisks` will be disabled the Forklift will use the KubeVirt CDI for disk transfer, and it will skip
the shared disks. After the disk transfer it will try to locate the already shared PVCs and attach them to the VMs.
The Forklift will try to automatically find migrated shared PVCs in the namespace and attach them to the VMs.

### User flow
The user will do following steps:
- Turn off all VMs with the attached shared disk on the VMware side.
- Create a plan with a single VM and the `migrateSharedDisks` enabled in the Forklift.
- Start the migration of the first plan and wait for it to finish.
- Create a secondary plan with all other VMs attached and the `migrateSharedDisks` disabled to the same target namespace as first plan.
- Start the migration of the second plan and wait for it to finish.
- Check if all shared disks are attached to all VMs.

### Security, Risks, Mitigations and Limitation

One of the limitation on the VMware side is that the shared disks do not support Change Block Tracking,
which is requirement for the warm migration. So the shared disks can be migrated only with the cold migration.

Another limitation is that the VMs with a shared disk can not be migrated to the separate namespaces as PVCs are
namespace bound and the VMs from another namespaces are not able to access them.

One of the risks is that all VMs with the shared disk must be turned off during the whole migration process.
Otherwise, there could be a risk of disk corruption or Forklift could not acquire lock on the disks and the migration,
would fail in middle of the process.

All these risks/limitation can be solved by adding corresponding validation and either blocking the migration if it's 
critical risk or warning the user about the potential.

Risk that can not be mitigated by validation is guest conversion of shared disks. With the current implementation of 
the feature we don't provide the shared disk during guest conversion, so virt-v2v is not aware of that disk. 
This can lead into issues as the guest conversion will not update the config files such as fstab.
This could be possibly mitigated in followup PR by mounting the shared disk during the guest conversion.
But the virt-v2v-in-place directly writes to the disks and if there would be multiple changes at the same time could 
cause disk corruption. 
This means that we can not support an Operating System on the shared disk, luckily this is not common scenario as most
shared disks are used for storing data.

## Design Details

### Test Plan

The existing tests will need to get expanded to have shared disks with multistage process.
First step to migrate the VMs with the disks and second wihtout.

### Upgrade / Downgrade Strategy

The plans without the `migrateSharedDisks` automatically get the default true.

### Open Questions

## Implementation History

* 02/27/2025 - Enhancement submitted.
