package migrator

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	azureclient "github.com/kubev2v/forklift/pkg/provider/azure/controller/client"
)

// resolveSnapshotResourceIDs returns the Azure resource IDs to use for VolumeSnapshotContent.
// When cross-region is active, it returns the target-region (copied) snapshot IDs.
func (r *Migrator) resolveSnapshotResourceIDs(vm *planapi.VMStatus, azureClient *azureclient.Client) ([]string, error) {
	if azureClient.IsCrossRegion() {
		xrNames, err := azureClient.GetCrossRegionSnapshotNames(vm.Ref)
		if err != nil {
			r.log.Error(err, "Failed to get cross-region snapshot names", "vm", vm.Name)
			return nil, liberr.Wrap(err)
		}
		if len(xrNames) == 0 {
			return nil, liberr.New("no cross-region snapshots found for VM")
		}
		var ids []string
		for _, xrName := range xrNames {
			resourceID, err := azureClient.GetCrossRegionSnapshotResourceID(xrName)
			if err != nil {
				return nil, liberr.Wrap(err)
			}
			ids = append(ids, resourceID)
		}
		return ids, nil
	}

	snapshots, err := azureClient.GetSnapshotsForVM(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get snapshots for VM", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}
	if len(snapshots) == 0 {
		return nil, liberr.New("no snapshots found for VM")
	}
	var ids []string
	for _, snapshotName := range snapshots {
		resourceID, err := azureClient.GetSnapshotResourceID(snapshotName)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		ids = append(ids, resourceID)
	}
	return ids, nil
}

func (r *Migrator) createSnapshotContent(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating VolumeSnapshotContent resources", "vm", vm.Name)

	azureClient := r.getAzureClient()
	ensurer := r.getEnsurer()

	snapshotResourceIDs, err := r.resolveSnapshotResourceIDs(vm, azureClient)
	if err != nil {
		return false, err
	}

	err = ensurer.EnsureVolumeSnapshotContent(vm, snapshotResourceIDs)
	if err != nil {
		r.log.Error(err, "Failed to create VolumeSnapshotContent", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("VolumeSnapshotContent resources created",
		"vm", vm.Name,
		"count", len(snapshotResourceIDs))

	return true, nil
}

func (r *Migrator) createVolumeSnapshot(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating VolumeSnapshot resources", "vm", vm.Name)

	ensurer := r.getEnsurer()

	err := ensurer.EnsureVolumeSnapshot(vm)
	if err != nil {
		r.log.Error(err, "Failed to create VolumeSnapshot", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("VolumeSnapshot resources created", "vm", vm.Name)
	return true, nil
}
