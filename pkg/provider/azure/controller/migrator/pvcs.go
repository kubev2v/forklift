package migrator

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

func (r *Migrator) createPVCs(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating PVCs with VolumeSnapshot dataSource", "vm", vm.Name)

	ensurer := r.getEnsurer()

	err := ensurer.EnsurePVCs(vm)
	if err != nil {
		r.log.Error(err, "Failed to create PVCs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("PVCs created", "vm", vm.Name)
	return true, nil
}

func (r *Migrator) waitForPVCsBound(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking PVC bound status", "vm", vm.Name)

	ensurer := r.getEnsurer()

	ready, err := ensurer.CheckPVCsBound(vm)
	if err != nil {
		r.log.Error(err, "Failed to check PVC bound status", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if ready {
		r.log.Info("All PVCs are bound", "vm", vm.Name)
	} else {
		r.log.Info("Waiting for PVCs to be bound", "vm", vm.Name)
	}

	return ready, nil
}

func (r *Migrator) injectOwnerRefs(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Injecting owner references", "vm", vm.Name)

	ensurer := r.getEnsurer()

	err := ensurer.InjectOwnerReferences(vm)
	if err != nil {
		r.log.Error(err, "Failed to inject owner references", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Owner references injected", "vm", vm.Name)
	return true, nil
}
