---
title: kubevirt-live-migration
authors:
  - "@mansam"
reviewers:
  - "@mnecas"
approvers:
  - "@mnecas"
creation-date: 2025-02-10
last-updated: 2025-02-13
status: implementable
---

# KubeVirt Live Migration

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Implement a pipeline for orchestrating Live Migration between Kubernetes clusters.
This pipeline represents a new migration type (cold, warm, live) and the first
that has an entirely provider-specific implementation. The cluster admin will be
responsible for establishing connectivity between the source and target clusters.
KubeVirt will be responsible for the migration mechanics, including storage migration.
Forklift will only need to create resources on the source and target clusters and
wait for migration to complete.

## Motivation

A [KubeVirt enhancement](https://github.com/kubevirt/enhancements/pull/6) has been opened for live migrating VMs between clusters. 
Migrating manually is possible but impractical, as it requires the user to create the destination VM object and any volumes,
secrets, configmaps or other resources that the source VM will require and requires the user to manage every step of the process.
These are things that Forklift already specializes in,  so it's a natural fit to do the orchestration in Forklift to
simplify the migration process for users. The motivation to do this orchestration with Forklift is that it already does the hard work of building
the inventory of resources on the source, mapping source resources to the destination, and managing the migration pipeline.

### Goals

* Orchestrate live migration of a KubeVirt VM from one cluster to another via a "live" Plan.
    * Ensure necessary shared resources are accessible on the destination, including instance types, VM preferences,
      secrets, and configmaps which may be mounted by multiple VMs.

### Non-Goals

* Automatically establish intercluster connectivity
* Migrate resources unrelated to VMs that may be necessary for application availability
  after migration (services, routes, etc)
* Implement live migration for providers other than KubeVirt.

## Proposal

### User Stories

#### Story 1

As a cluster admin, I want to migrate a VM from one cluster to another to rebalance workloads
without downtime.

### Implementation Overview

Forklift was designed with an assumption that the migration process is approximately
the same for each source hypervisor. This assumption lead to a design where the providers
all share the same two (cold, warm) migration pipelines with provider-specific implementations
of pipeline steps. It has become clear over time that this assumption has not held. A substantial
amount of provider-specific branching has been added to the pipelines over time, as well as branching
within the shared steps to deal with storage- or provider-specific idiosyncrasies.

KubeVirt live migration requires a workflow that is so different from cold and warm migration that it
is not reasonable to repurpose the existing pipelines for live migration; a new pipeline needs to
be implemented. Moreover, the live migration pipeline is entirely provider specific. Even if it
were possible to implement live migration for another source hypervisor, it would be so different
in requirements that the pipeline implemented for the KubeVirt provider would not be usable. Due to these considerations
it is necessary to design and implement a flow for using provider-specific migration pipelines.

### Migration Prerequisites

##### Connectivity

The source and target clusters need to be connected such that KubeVirt can communicate cluster-to-cluster
to transfer state. Submariner is one option for this. In any case, configuring connectivity is an administrator
responsibility outside the scope of Forklift.

#### VirtualMachineInstanceTypes and VirtualMachinePreferences

Validation should check whether the target cluster has `VirtualMachineInstanceTypes` and `VirtualMachinePreferences`
that match those used by the VMs on the source cluster. This can be done by looking for resources with
the same name as those referenced by the source VMs, and then comparing the contents to see if they are
identical. If the referenced resources are not present or do not match, appropriate warnings should be raised.
Whether this should be a hard stop on the migration could be configured at the provider level.

##### Proposed Migration Pipeline

            {Name: Started},
            {Name: PreHook, All: HasPreHook},
            {Name: EnsureResources}, // preferences and instance types
            {Name: CreateEmptyDataVolumes},
            {Name: CreateStandbyVM},
            {Name: CreateServiceExports},
            {Name: CreateVirtualMachineInstanceMigrations},
            {Name: WaitForStateTransfer},
            {Name: PostHook, All: HasPostHook},
            {Name: Completed}

* **EnsureResources**: Any secrets or configmaps that are mounted by the VM on the source need to be
  duplicated to the target namespace. Multiple VMs could rely on the same configmap or secret, so Forklift
  will allow this step to pass if secrets or configmaps with the correct names (and Forklift labels) already
  exist.
* **CreateEmptyDataVolumes**: KubeVirt is going to handle storage migration, so all that is necessary
  for Forklift to do is create blank target DataVolumes.
* **CreateStandbyVM**: The target VM needs to be created mounting the blank disks and any secrets or configmaps.
  It needs to be created in a halted state, as KubeVirt will be responsible for starting it once the synchronization
  is complete.
* **CreateServiceExports**: If necessary, Forklift will create ServiceExports in the KubeVirt namespace on the
  target cluster to expose the synchronization endpoints to the source.
* **CreateVirtualMachineInstanceMigrations**: A VirtualMachineInstanceMigration needs to be created in both the source and target namespaces.
  Once the target VMIM is reconciled and ready, it will present a migration endpoint to use for the state transfer. This must be provided in
  the source VMIM spec. If Submariner is in use,
* **WaitForStateTransfer**: Once the source VMIM is created, KubeVirt will handle the state transfer and
  Forklift only needs to wait for the destination VM to report ready. KubeVirt will handle shutdown of the
  source VM.

### CR Changes

The current implementation of the Plan CR has a boolean to indicate a warm migration, so the CR
needs to be extended to support other migration types. An optional string field must be added to accept a migration
type, that if populated takes precedence over the boolean flag.

### Provider adapter changes

The provider adapter interface needs to be expanded to handle provider-specific migration paths.
A new "Migrator" component would be responsible for indicating whether the provider supports a given
migration path and whether it provides its own implementation of any portions of the migration path.

A draft of the new component interface might look something like this:

```go

type Migrator interface {
    Init() error
    Status(plan.VM) *plan.VMStatus
    Reset(*plan.VMStatus, []*plan.Step)
    Pipeline(plan.VM) ([]*plan.Step, error)
    ExecutePhase(*plan.VMStatus) (bool, error)
    Step(*plan.VMStatus) string
    Next(status *plan.VMStatus) (next string)
}
```

The migration runner in `plan/migration.go` would be updated to defer to the provider
implementation if available. (Integration would be at the points where the itinerary
is selected, the pipeline is generated, and where individual phases are executed.)

Approaching it in this way allows the provider adapter to take responsibility for
portions of the migration flow (or the entire flow) without requiring a full reimplementation
of the migration flows for each provider all at once.

### Security, Risks, and Mitigations

Forklift will require new access to read and create VirtualMachineInstanceMigration and ServiceExport
instances on the source and target clusters.
Otherwise, the usual security risks apply for cluster to cluster migrations: Forklift
has significant access to secrets and other resources on both clusters, and we need to ensure
that the user deploying the migration plan has the appropriate rights in the source and target
namespaces.

## Design Details

### Test Plan

Unit tests will be written to ensure that the Migrator component logic
behaves correctly and that the migration runner defers to provider specific
implementations correctly.

Integration tests need to be written to ensure that the KubeVirt live migration path
succeeds.

### Upgrade / Downgrade Strategy

This enhancement requires an operator change to deploy a revised Plan CR and
updated controller image. Existing plans are compatible with the updated controller;
plans created using the new migration type field will appear to the old version of
the controller as though they were cold migrations. No special handling is required
to upgrade or downgrade since the changes are purely additive.

## Implementation History

* **February 13, 2025**: Enhancement submitted.