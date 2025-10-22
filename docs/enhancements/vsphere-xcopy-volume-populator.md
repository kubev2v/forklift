---
title: vsphere-xcopy-volume-populator
authors:
  - "@rgolangh"
reviewers:
  - "@mnecas"
approvers:
  - "@mnecas"
creation-date: 18/05/2025 
last-updated: 18/05/2025 
status: implemented
see-also:
  - "/cmd/vsphere-xcopy-volume-populator/README.md"
---

vSphere XCOPY Volume-Populator
This document describes the vSphere XCOPY Volume-Populator, a specific implementation of a storage 'copy-offload'
feature within the Forklift project, designed for migrating Virtual Machine Disk (VMDK) data from VMware vSphere
environments to KubeVirt Persistent Volume Claims (PVCs) using the storage vendor's XCOPY capability, exposed by vSphere.
The Forklift project is a Toolkit for migrating VMs from various sources, including VMware and vSphere, to KubeVirt
It includes components for volume population.
The vSphere XCOPY Volume-Populator is a populator implementation located under cmd/vsphere-xcopy-volume-populator

Release Signoff Checklist
- [x] Enhancement is implementable
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created # This document and the README serve this purpose

## Open Questions
Based on the identified limitations:
1. TBD 

## Summary
The vsphere-xcopy-volume-populator provides a mechanism to perform storage-level data copy offload when migrating
vSphere Virtual Machine Disks (VMDKs) to KubeVirt Persistent Volume Claims (PVCs) using Forklift.
When a specific feature flag (forklift controller `spec.feature_copy_offload`) is enabled, and the storage
configuration allows for offloading, the Forklift controller will provision a PVC and associate it with a custom
`VSphereXcopyVolumePopulator` resource via the `dataSourceRef` field. A dedicated populator controller then
orchestrates a containerised CLI program that uses storage and vSphere APIs to instruct the ESXi host to perform an
XCOPY operation using vmkfstools, transferring data directly between storage LUNs . This approach is intended to be
more efficient than traditional host-based copying. The mechanism requires the ESXi host to have VAAI and accelerations
enabled and needs a helper VIB (vmkfstools-wrapper) installed on relevant ESXi hosts.

## Motivation
Migrating large volumes of data from vSphere VMs can be time-consuming and consume significant host resources.
The motivation for this feature is to efficiently copy data from source VMDKs to target PVCs during migration.
By leveraging storage array capabilities via XCOPY, the data transfer is accelerated and offloaded from the host to the
array, reducing migration downtime and impact on compute resources, thereby facilitating the migration of virtual machines at scale.

### Goals
- To implement a copy-offload mechanism for vSphere VMDKs during migration to KubeVirt PVCs.
- To utilise the XCOPY primitive available through vSphere/ESXi and capable storage arrays for efficient data transfer.
- To integrate this capability into the Forklift migration workflow using the Kubernetes volume populator framework.
- To support storage configurations where source VMDKs and target PVCs reside on LUNs from the same storage array endpoint.
- To enable this feature via a configurable feature_copy_offload flag and StorageMap configuration.

### Non-Goals
- Support for migrating VMs with multiple disks in a single XCOPY operation (currently limited to single disk).
- Support for storage configurations where the source VMDK and target PVC are not located on LUNs from the same storage array endpoint.
- Implementing copy offload for sources other than vSphere using this specific populator (this populator is vsphere-xcopy specific).

## Proposal
The proposal outlines the implementation of a `VSphereXcopyVolumePopulator` resource and a corresponding populator
controller. This populator is activated when the feature_copy_offload flag is enabled and the `StorageMap` configuration
indicates that a source datastore and destination storage class pair supports offload.
When a VM disk migration is planned for offload:
1. The Forklift controller creates the target `PVC` and a `VSphereXcopyVolumePopulator` custom resource.
2. The PVC's `dataSourceRef` is set to reference the created `VSphereXcopyVolumePopulator` resource.
3. The volume populator controller for `VSphereXcopyVolumePopulator` instances picks up the new resource.
4. A container running the `vsphere-xcopy-volume-populator` CLI program is orchestrated by the volume-populator
5. This program uses a configurable storage API to map the target PVC back to the specific ESXi host it resides on.
6. It then uses the vSphere API to invoke functions on that ESXi host.
7. These functions trigger the ESXi host to perform a vmkfstools clone operation, leveraging XCOPY
   (provided VAAI is enabled) to copy the data directly from the source VMDK's LUN to the target PVC's LUN.
8. This relies on a pre-installed vmkfstools-wrapper VIB on the ESXi host, which provides an API interface to the
   vmkfstools commands.

### User Stories
- Story: As a migration administrator, I want to migrate my large vSphere VMs using Forklift as quickly and efficiently
  as possible, leveraging my enterprise storage array's capabilities. I expect Forklift to utilise storage offload
  features like XCOPY when my vSphere and storage environment supports it.
- Story: As a storage administrator, I want VM migration traffic to minimise impact on my vSphere hosts' CPU and
  network resources. I prefer data transfer to happen directly between LUNs on my storage array using XCOPY when
  migrating VMs from datastores on that array to volumes used by KubeVirt PVCs on the same array.

### Implementation Details/Notes/Constraints
- The populator uses a configurable storage API (`storageVendorProduct`) to interact with the underlying storage array,
  which is specified in the `StorageMap`. Specific storage vendors such as, Hitachi Vantara , NetApp's ontap, Dell PowerFlex
  HP's Primera/3par/Aletra, require implementing vendor-specific logic within the populator.
- A Kubernetes Secret containing storage provider credentials must be referenced in the StorageMap.
- The vmkfstools-wrapper VIB must be installed on all ESXi hosts that are connected to the datastores holding the
  migratable VMs. This VIB wraps vmkfstools commands to allow API interaction.
- The mechanism for matching source VMDK datastores with target PVC storage classes to determine copy offload support
  relies on heuristics that identify if the source VMDK LUN and target PVC LUN would be created on the same storage
  system endpoint. This matching is configured in the `StorageMap`.

### Security, Risks, and Mitigations
- Risk: Exposure of storage provider credentials.
  Mitigation: Credentials are stored in Kubernetes Secrets, which should be managed securely according to standard Kubernetes practices.
- Risk: Potential for misuse or unintended access if the vSphere API or ESXi vmkfstools-wrapper are compromised.
  Mitigation: The populator runs in a containerised environment, providing some level of isolation.
  The vmkfstools-wrapper acts as a proxy, potentially limiting direct CLI access, although its specific security model
  isn't detailed in the sources. Access to the Kubernetes API (to create the populator resource and secrets) and
  vSphere API should be appropriately restricted.
- Risk: Data security during transit.
  Mitigation: The XCOPY operation is performed by the storage array itself, typically within the storage network,
  reducing exposure compared to host-based copy over less secure networks. However, specific security guarantees depend
  on the storage array's implementation and configuration.

Security review processes would involve examining the implementation of the populator, the handling of secrets, and the
interaction points with vSphere/ESXi and storage APIs.

## Design Details
The core design involves custom Kubernetes resources (`VSphereXcopyVolumePopulator`, `StorageMap`), a dedicated controller,
a containerised CLI populator, and dependencies on vSphere/ESXi capabilities (`VAAI`, `vmkfstools`, `vmkfstools-wrapper`)
and storage array configuration.

### Test Plan
- Not detailed in sources. A comprehensive test plan would need to cover:
- Unit tests for the populator logic and storage vendor implementations.
- Integration tests verifying the interaction between the Forklift controller, the populator controller, the populator
  container, vSphere API, and a ESXi host with the VIB installed.
- End-to-end migration tests using the XCOPY populator with various supported storage configurations and VM sizes.
- Testing error handling and retry mechanisms for vSphere/ESXi SOAP errors.
- Testing with different supported storage vendors (e.g., Hitachi Vantara, NetApp Ontap).
- Testing the `feature_copy_offload` flag and `StorageMap` matching logic.

### Upgrade / Downgrade Strategy
- Not detailed in sources. An upgrade/downgrade strategy would need to consider:
- Compatibility of the new populator controller and resource definition with existing Forklift installations.
- Handling of in-progress migrations using the XCOPY populator during Forklift upgrades/downgrades.
- The requirement for the vmkfstools-wrapper VIB on ESXi hosts means that upgrading/downgrading the populator may require corresponding updates or verification steps on the vSphere infrastructure.

## Implementation History
- N/A

## Drawbacks
- Dependency on vSphere/ESXi Configuration: Requires VAAI/accelerations to be enabled on ESXi.
- Dependency on ESXi VIB Installation: Requires the vmkffstools-wrapper VIB to be installed manually or via automation
  (e.g., Ansible) on relevant ESXi hosts.
- Storage Array Dependency: Only works with storage arrays that support vSphere XCOPY (VAAI).
- Configuration Complexity: Requires specific configuration in the `StorageMap` to correctly identify source
  datastore/target storage class pairs that support offload. Requires secrets for storage vendor credentials.
- Current Limitations: Currently supports only single-disk VMs. Requires the source VMDK and target PVC LUNs to be on
  the same storage array endpoint.

## Alternatives
- Standard Cold Migration via virt-v2v: For cold migrations, Forklift uses virt-v2v. virt-v2v can handle disk conversion and
  potentially different copy mechanisms, but it is not specifically designed for storage offload like XCOPY.
- Warm Migration (CBT): Forklift supports warm migration using Change Block Tracking. This is a different mechanism
  focused on minimising downtime by transferring only changed blocks after an initial full copy (which itself might use
  standard copy or, potentially, XCOPY if configured).

## Infrastructure Needed
- A vSphere environment with ESXi hosts that support VAAI and have accelerations enabled.
- Relevant ESXi hosts require the vmkfstools-wrapper VIB installed.
- A storage array supporting vSphere VAAI/XCOPY, where the source VMDK datastore and target PVC volumes can coexist on
  LUNs from the same storage array endpoint.
- Storage provider-specific Go code implementations within the populator for the specific storage array product being used.
- Kubernetes Secrets containing credentials for the storage provider.
- Forklift operator and the necessary populator controller and populator container images deployed on the Kubernetes cluster.
