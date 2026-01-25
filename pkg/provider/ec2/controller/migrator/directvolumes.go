package migrator

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	core "k8s.io/api/core/v1"
)

// getVolumeIDs retrieves created EBS volume IDs from AWS by querying volumes tagged with the VM name.
// Returns a map of originalVolumeID -> newVolumeID.
func (r *Migrator) getVolumeIDs(vm *planapi.VMStatus) (map[string]string, error) {
	ec2Client := r.getEC2Client()
	return ec2Client.GetCreatedVolumesForVM(vm.Ref)
}

// createVolumes creates EBS volumes from snapshots in the target AZ.
// Returns true when all volumes are created. Volumes are tagged in AWS with VM name
// and original volume ID for later retrieval.
func (r *Migrator) createVolumes(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating EBS volumes from snapshots", "vm", vm.Name)

	// Mark step as running
	if step, found := vm.FindStep(DiskTransfer); found {
		if !step.MarkedStarted() {
			step.MarkStarted()
		}
		step.Phase = api.StepRunning
	}

	// Check if volumes already created in AWS (tagged with this VM)
	existingVolumes, err := r.getVolumeIDs(vm)
	if err == nil && len(existingVolumes) > 0 {
		r.log.Info("Volumes already exist in AWS, skipping creation",
			"vm", vm.Name,
			"volumeCount", len(existingVolumes))
		return true, nil
	}

	// Get snapshot IDs from AWS tags
	snapshotMap, err := r.getSnapshotIDs(vm)
	if err != nil {
		r.log.Error(err, "Failed to get snapshot IDs from AWS", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(snapshotMap) == 0 {
		err := fmt.Errorf("no snapshots found in AWS for VM %s", vm.Name)
		r.log.Error(err, "No snapshots to create volumes from", "vm", vm.Name)
		return false, err
	}

	r.log.Info("Creating volumes from snapshots",
		"vm", vm.Name,
		"snapshotCount", len(snapshotMap))

	// Get EC2-specific client for volume operations
	ec2Client := r.getEC2Client()

	// Create volumes from each snapshot (volumes are tagged in AWS)
	volumeCount := 0
	for originalVolumeID, snapshotID := range snapshotMap {
		r.log.Info("Creating volume from snapshot",
			"vm", vm.Name,
			"originalVolumeID", originalVolumeID,
			"snapshotID", snapshotID)

		newVolumeID, err := ec2Client.CreateVolumeFromSnapshot(vm.Ref, originalVolumeID, snapshotID)
		if err != nil {
			r.log.Error(err, "Failed to create volume from snapshot",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID,
				"snapshotID", snapshotID)
			return false, liberr.Wrap(err)
		}

		r.log.Info("Volume created and tagged in AWS",
			"vm", vm.Name,
			"originalVolumeID", originalVolumeID,
			"newVolumeID", newVolumeID)
		volumeCount++
	}

	r.log.Info("All volumes created",
		"vm", vm.Name,
		"volumeCount", volumeCount)

	return true, nil
}

// waitForVolumes checks if all EBS volumes are available.
// Returns true when all volumes reach the 'available' state.
func (r *Migrator) waitForVolumes(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking volume availability", "vm", vm.Name)

	// Get volume mapping from AWS tags
	volumeMapping, err := r.getVolumeIDs(vm)
	if err != nil {
		r.log.Error(err, "Failed to get volume IDs from AWS", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(volumeMapping) == 0 {
		err := fmt.Errorf("no volumes found in AWS for VM %s", vm.Name)
		r.log.Error(err, "No volumes to check", "vm", vm.Name)
		return false, err
	}

	// Collect all new volume IDs
	volumeIDs := make([]string, 0, len(volumeMapping))
	for _, newVolumeID := range volumeMapping {
		volumeIDs = append(volumeIDs, newVolumeID)
	}

	r.log.Info("Checking volume status",
		"vm", vm.Name,
		"volumeCount", len(volumeIDs))

	// Get EC2-specific client for volume operations
	ec2Client := r.getEC2Client()

	// Check if all volumes are ready
	ready, err := ec2Client.CheckVolumesReady(vm.Ref, volumeIDs)
	if err != nil {
		r.log.Error(err, "Failed to check volume status", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if ready {
		r.log.Info("All volumes are available", "vm", vm.Name)
	} else {
		r.log.Info("Waiting for volumes to become available", "vm", vm.Name)
	}

	return ready, nil
}

// createPVsAndPVCs creates PersistentVolumes and PersistentVolumeClaims for the EBS volumes.
// PVs are created with CSI volume source pointing to EBS volumes, pre-bound to PVCs.
// Returns true when all PVCs are bound.
func (r *Migrator) createPVsAndPVCs(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating PVs and PVCs for EBS volumes", "vm", vm.Name)
	ctx := context.TODO()

	ec2Ensurer := r.getEnsurer()
	ec2Builder, ok := r.builder.(*builder.Builder)
	if !ok {
		return false, liberr.New("builder is not an EC2 builder")
	}

	// Get volume mapping from AWS tags
	volumeMapping, err := r.getVolumeIDs(vm)
	if err != nil {
		r.log.Error(err, "Failed to get volume IDs from AWS", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(volumeMapping) == 0 {
		err := fmt.Errorf("no volumes found in AWS for VM %s", vm.Name)
		r.log.Error(err, "No volumes to create PVs/PVCs for", "vm", vm.Name)
		return false, err
	}

	// Get snapshot mapping from AWS tags for volume metadata
	snapshotMap, err := r.getSnapshotIDs(vm)
	if err != nil {
		r.log.Error(err, "Failed to get snapshot IDs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Build PVC specs first (we need PVC names for PV ClaimRef)
	var pvcs []*core.PersistentVolumeClaim
	var pvs []*core.PersistentVolume
	volumeInfos := make(map[string]*builder.VolumeInfo)

	// Get the AWS instance to iterate over BlockDeviceMappings in the correct order.
	// This ensures the disk-index annotation matches the order that mapDisks uses
	// when attaching disks to the VM spec.
	awsInstance, err := inventory.GetAWSInstance(ec2Builder.Source.Inventory, vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get AWS instance from inventory", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	blockDevices, _ := inventory.GetBlockDevices(awsInstance)

	// Iterate over BlockDeviceMappings to preserve disk order from the source VM.
	// The volumeMapping map is only used for lookups, not iteration order.
	// Using the slice index directly preserves the source block device position.
	for i, dev := range blockDevices {
		if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
			continue
		}

		originalVolumeID := *dev.Ebs.VolumeId
		newVolumeID, found := volumeMapping[originalVolumeID]
		if !found {
			r.log.Info("No new volume found for original volume, skipping",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID)
			continue
		}

		snapshotID := snapshotMap[originalVolumeID]

		// Get volume size from inventory
		sizeGiB := ec2Builder.GetVolumeSize(originalVolumeID, snapshotID)

		// Get volume type from inventory for storage class mapping
		volumeType := r.getVolumeType(originalVolumeID)

		volumeInfo := &builder.VolumeInfo{
			EBSVolumeID:      newVolumeID,
			OriginalVolumeID: originalVolumeID,
			SnapshotID:       snapshotID,
			SizeGiB:          sizeGiB,
			VolumeType:       volumeType,
		}
		volumeInfos[originalVolumeID] = volumeInfo

		// Build PVC spec with index matching the BlockDeviceMappings position
		pvc, err := ec2Builder.BuildDirectPVC(vm.Ref, volumeInfo, i)
		if err != nil {
			r.log.Error(err, "Failed to build PVC spec",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID)
			return false, liberr.Wrap(err)
		}
		pvcs = append(pvcs, pvc)
	}

	// Create PVCs first to get their generated names
	pvcNames, err := ec2Ensurer.EnsureDirectPVCs(ctx, vm, pvcs)
	if err != nil {
		r.log.Error(err, "Failed to create PVCs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Build PV specs with ClaimRef pointing to created PVCs
	for originalVolumeID, volumeInfo := range volumeInfos {
		pvcName := pvcNames[originalVolumeID]
		if pvcName == "" {
			r.log.Error(nil, "PVC name not found for volume",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID)
			continue
		}

		pv, err := ec2Builder.BuildPersistentVolume(vm.Ref, volumeInfo, pvcName, r.Plan.Spec.TargetNamespace)
		if err != nil {
			r.log.Error(err, "Failed to build PV spec",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID)
			return false, liberr.Wrap(err)
		}
		pvs = append(pvs, pv)
	}

	// Create PVs
	_, err = ec2Ensurer.EnsurePersistentVolumes(ctx, vm, pvs)
	if err != nil {
		r.log.Error(err, "Failed to create PVs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Check if all PVCs are bound
	allBound, err := ec2Ensurer.CheckDirectPVCsBound(ctx, vm)
	if err != nil {
		r.log.Error(err, "Failed to check PVC binding status", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if allBound {
		r.log.Info("All PVCs are bound", "vm", vm.Name)
		// Update progress
		if step, found := vm.FindStep(DiskTransfer); found {
			step.Progress.Completed = step.Progress.Total
		}
	} else {
		r.log.Info("Waiting for PVCs to be bound", "vm", vm.Name)
	}

	return allBound, nil
}

// getVolumeType retrieves the EBS volume type from inventory.
// Returns "gp3" as default if the volume type cannot be determined.
func (r *Migrator) getVolumeType(volumeID string) string {
	// Default volume type
	defaultType := "gp3"

	// Try to get volume type from inventory via builder
	ec2Builder, ok := r.builder.(*builder.Builder)
	if !ok {
		return defaultType
	}

	volumeType := ec2Builder.GetVolumeType(volumeID)
	if volumeType == "" {
		return defaultType
	}

	return volumeType
}
