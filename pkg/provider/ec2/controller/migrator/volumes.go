package migrator

import (
	"context"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// createDataVolumes creates PVCs and Ec2VolumePopulator CRs from EBS snapshots, waits for binding.
// Ensures RBAC (ServiceAccount), per-VM secret with AWS credentials, then creates populator resources.
// Returns true when all PVCs bound. Populator controller creates volumes asynchronously from snapshots.
func (r *Migrator) createDataVolumes(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating PVCs from EBS snapshots", "vm", vm.Name)
	ctx := context.TODO()

	ec2Ensurer := r.getEnsurer()

	// Step 1: Ensure the ServiceAccount exists for populator pods
	// This ServiceAccount is shared across all VMs in the plan
	if err := ec2Ensurer.EnsurePopulatorServiceAccount(ctx, r.Plan.Spec.TargetNamespace); err != nil {
		r.log.Error(err, "Failed to ensure populator service account", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Step 2: Create a VM-specific secret with AWS credentials
	// Each VM gets its own secret to isolate credentials and enable per-VM cleanup
	populatorSecretName, err := ec2Ensurer.EnsurePopulatorSecret(ctx, vm)
	if err != nil {
		r.log.Error(err, "Failed to ensure populator secret", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Step 3: Create Ec2VolumePopulator CRs and PVCs, then check if all are bound
	// This creates all the resources needed for volume population and polls their status
	allBound, err := ec2Ensurer.EnsurePopulatorDataVolumes(ctx, vm, r.builder, populatorSecretName)
	if err != nil {
		r.log.Error(err, "Failed to ensure populator data volumes", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if allBound {
		r.log.Info("All PVCs are bound", "vm", vm.Name)
		return true, nil
	}

	// Still waiting for PVCs to be bound by the populator controller
	r.log.Info("Waiting for PVCs to be bound", "vm", vm.Name)
	return false, nil
}
