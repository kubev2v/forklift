package migrator

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// isVMRunning checks the source VM's power state to decide whether
// incremental pre-snapshots should be taken (only useful while VM is running).
func (r *Migrator) isVMRunning() (bool, error) {
	if r.vm == nil {
		return false, nil
	}
	azureClient := r.getAzureClient()
	deallocated, err := azureClient.IsVMDeallocated(r.vm.Ref)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	return !deallocated, nil
}

func (r *Migrator) createPreSnapshot(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating incremental pre-snapshots (VM running)", "vm", vm.Name)

	azureClient := r.getAzureClient()

	existing, err := azureClient.GetPreSnapshotsForVM(vm.Ref)
	if err == nil && len(existing) > 0 {
		r.log.Info("Pre-snapshots already exist, skipping creation",
			"vm", vm.Name,
			"count", len(existing))
		return true, nil
	}

	names, err := azureClient.CreatePreSnapshots(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to create pre-snapshots", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Pre-snapshots created",
		"vm", vm.Name,
		"count", len(names))

	return true, nil
}

func (r *Migrator) waitForPreSnapshot(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking pre-snapshot provisioning status", "vm", vm.Name)

	azureClient := r.getAzureClient()
	preSnaps, err := azureClient.GetPreSnapshotsForVM(vm.Ref)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if len(preSnaps) == 0 {
		return false, liberr.New("no pre-snapshots found for VM")
	}

	for _, name := range preSnaps {
		ready, err := azureClient.IsSnapshotReady(name)
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if !ready {
			r.log.Info("Pre-snapshot not yet ready", "vm", vm.Name, "snapshot", name)
			return false, nil
		}
	}

	r.log.Info("All pre-snapshots are ready", "vm", vm.Name)
	return true, nil
}

func (r *Migrator) deletePreSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Deleting pre-snapshots (no longer needed after final snapshot)", "vm", vm.Name)

	azureClient := r.getAzureClient()
	err := azureClient.DeletePreSnapshots(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to delete pre-snapshots", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Pre-snapshots deleted", "vm", vm.Name)
	return true, nil
}

func (r *Migrator) deallocateVM(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Deallocating Azure VM", "vm", vm.Name)

	azureClient := r.getAzureClient()
	err := azureClient.DeallocateVM(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to deallocate VM", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("VM deallocation initiated", "vm", vm.Name)
	return true, nil
}

func (r *Migrator) waitForDeallocation(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking VM deallocation status", "vm", vm.Name)

	azureClient := r.getAzureClient()
	ready, err := azureClient.IsVMDeallocated(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to check VM deallocation status", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if ready {
		r.log.Info("VM is deallocated", "vm", vm.Name)
	} else {
		r.log.Info("Waiting for VM deallocation", "vm", vm.Name)
	}

	return ready, nil
}

func (r *Migrator) createSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating Azure managed disk snapshots", "vm", vm.Name)

	azureClient := r.getAzureClient()

	existing, err := azureClient.GetSnapshotsForVM(vm.Ref)
	if err == nil && len(existing) > 0 {
		r.log.Info("Snapshots already exist, skipping creation",
			"vm", vm.Name,
			"count", len(existing))
		return true, nil
	}

	snapshotNames, err := azureClient.CreateDiskSnapshots(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to create snapshots", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Snapshots created",
		"vm", vm.Name,
		"snapshotCount", len(snapshotNames))

	return true, nil
}

func (r *Migrator) waitForSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking snapshot provisioning status", "vm", vm.Name)

	azureClient := r.getAzureClient()
	snapshots, err := azureClient.GetSnapshotsForVM(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get snapshots for VM", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(snapshots) == 0 {
		return false, liberr.New("no snapshots found for VM")
	}

	for _, snapshotName := range snapshots {
		ready, err := azureClient.IsSnapshotReady(snapshotName)
		if err != nil {
			r.log.Error(err, "Failed to check snapshot status",
				"vm", vm.Name,
				"snapshot", snapshotName)
			return false, liberr.Wrap(err)
		}
		if !ready {
			r.log.Info("Snapshot not yet ready",
				"vm", vm.Name,
				"snapshot", snapshotName)
			return false, nil
		}
	}

	r.log.Info("All snapshots are ready", "vm", vm.Name)
	return true, nil
}

func (r *Migrator) copySnapshotsCrossRegion(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Copying snapshots to target region", "vm", vm.Name)

	azureClient := r.getAzureClient()

	xrNames, err := azureClient.CopySnapshotsCrossRegion(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to copy snapshots cross-region", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Cross-region snapshot copies initiated",
		"vm", vm.Name,
		"count", len(xrNames))

	return true, nil
}

func (r *Migrator) waitForCrossRegionSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking cross-region snapshot copy status", "vm", vm.Name)

	azureClient := r.getAzureClient()

	xrNames, err := azureClient.GetCrossRegionSnapshotNames(vm.Ref)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if len(xrNames) == 0 {
		return false, liberr.New("no cross-region snapshots found")
	}

	ready, err := azureClient.AreCrossRegionSnapshotsReady(vm.Ref, xrNames)
	if err != nil {
		r.log.Error(err, "Failed to check cross-region snapshot status", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if ready {
		r.log.Info("All cross-region snapshots are ready", "vm", vm.Name)
	} else {
		r.log.Info("Waiting for cross-region snapshot copies", "vm", vm.Name)
	}

	return ready, nil
}
