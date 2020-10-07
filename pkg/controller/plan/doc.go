/*
The Plan CR represents a planned migration of VMs.
The plan defines the source and destination providers; the resource
mapping and a list of VMs to be migrated.  The plan controller
watches Migration CRs. Each Migration CR represents a separate and
ordered execution of the plan.  During plan execution, all validations
are suspended. The plan Status.Migration contains a snapshot of the
specification (except secrets) which is used during the execution.

Each plan execution:

1. Update the Status.Migration snapshot.
2. Ensure the plan CR namespace exists on the destination.
3. Ensure the CNV Secret exists and configured correctly on the destination.
4. Ensure the CNV ResourceMapping CR exists and configured correctly on the destination.
5. Create a CNV Import CR for each incomplete VM.
6. Requeue the reconcile until all of the VMs have either succeeded or failed.
7. A VM has completed successfully when it reaches the `Complete` phase without an error.

Each plan execution is idempotent. Subsequent migrations will only affect
incomplete or failed VM migrations.
*/
package plan
