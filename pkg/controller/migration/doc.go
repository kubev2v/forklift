/*
The Migration CR represents an execution of the Plan. The migration reconciler
watches the associated Plan and replicates the VM `migration` status to
the Status on the Migration CR. The actual migration is orchestrated by
the Plan controller.
*/
package migration
