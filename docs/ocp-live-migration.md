# Live Migration

Live migration is when a powered on VM is migrated to a separate host or new location within the same host without
being shut down. Live migration is available for migrating Virtual Machines within namespaces on the same
OpenShift Virtualization provider or between OpenShift Virtualization providers, provided that the source and destination
clusters are compatible, and the feature is enabled on both clusters.

## Pre-requisites

### Enable the feature

Consult the OpenShift Virtualization documentation to make the Live Migration feature available on the source
and destination clusters.

To enable it in Forklift, the `ForkliftController` CR must be modified to add the feature flag:

```yaml
spec:
  feature_ocp_live_migration: true
```

### Cross-cluster Connectivity

The source and destination clusters must share a network to synchronize VM state. If the synchronization addresses advertised
by the synchronization controllers are not reachable from both clusters, the migration will be unable to proceed successfully.

At the start of the migration process, Forklift will synchronize CAs between the OpenShift Virtualization namespaces on the
source and destination clusters.

### VM compatibility

The virtual machines to be migrated must not have any features or conditions that preclude live migration. A VM that is
reporting LiveMigratable or StorageLiveMigratable conditions that are false will not be able to be migrated.

* VMs using shared disks cannot be live migrated.
* Cluster-scoped InstanceTypes and Preferences that are referred to by a VM must already exist on the destination cluster.
  Locally scoped InstanceTypes and Preferences will be copied to the destination if they do not exist, but any InstanceType
  or Preference with the same name that already exists in the destination namespace is assumed to be suitable. Preexisting InstanceTypes
  and Preferences on the destination cluster that are substantially different from those used by the VM on the source may result
  in an unsuccessful migration.
* ConfigMaps and Secrets in the source namespace that are referred to by a VM will be copied to the destination namespace unless
  one with the same name already exists in the destination namespace.
* DataVolumes referred to by the source VM will be recreated in the destination namespace. Name collisions (due to a preexisting
  volume or more than one VM with identically named volumes being moved to the same destination namespace) will result in
  an unsuccessful migration.

## Migration process

### Creating a plan

If the Plan is created via the console UI and feature is available, `Live migration` will be presented as an option in
the migration wizard. If the Plan is created in the CLI, then the field `type: live` must be added to the Plan spec.

### Migration steps

Forklift takes the following steps during a live migration:

1. Synchronize certificates (if necessary)
2. Create prerequisite resources in target namespace
3. Create destination VM
4. Create VirtualMachineInstanceMigration resources
5. Synchronize VM state

### State synchronization

After any required resources have been created in the destination namespace, Forklift will create a target VirtualMachine
with a RunStrategy of `WaitAsReceiver`. Forklift will then create a VirtualMachineInstanceMigration resource first in the
destination namespace and then in the source namespace with a unique migration ID. The synchronization addresses
advertised in the target VirtualMachineInstanceMigration resource's status must be reachable from the source in order
for synchronization to proceed. If the target is not reachable, the migration process may stall indefinitely.

Once the state transfer is complete, the source VirtualMachineInstanceMigration resource will be removed, the
source VM will be powered off, and the target VM will be running. The target VM will also have its RunStrategy changed
to match the original RunStrategy of the source VM.