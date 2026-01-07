package migrator

import (
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// getSnapshotIDs retrieves snapshot IDs from AWS by querying snapshots tagged with the VM name.
// Returns a map of volumeID -> snapshotID.
func (r *Migrator) getSnapshotIDs(vm *planapi.VMStatus) (map[string]string, error) {
	ec2Client := r.getEC2Client()
	return ec2Client.Client.GetSnapshotsForVM(vm.Ref)
}

// createSnapshots creates EBS snapshots for all volumes attached to the VM.
// Snapshots are tagged with VM name and volume ID in AWS for later retrieval.
func (r *Migrator) createSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating EBS volume snapshots", "vm", vm.Name)

	if err := r.markSnapshotStepRunning(vm); err != nil {
		return false, err
	}

	// Check if snapshots already exist in AWS (tagged with this VM)
	if r.snapshotsAlreadyCreated(vm) {
		return true, nil
	}

	// Validate VM has EBS volumes
	if _, err := r.extractVolumeIDs(vm); err != nil {
		return false, err
	}

	// Create snapshots via adapter client (snapshots are tagged in AWS)
	snapshotIDString, _, err := r.adpClient.CreateSnapshot(vm.Ref, nil)
	if err != nil {
		r.log.Error(err, "Failed to create snapshots", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	snapshotIDs := strings.Split(snapshotIDString, ",")
	r.log.Info("Snapshots created and tagged in AWS",
		"vm", vm.Name,
		"snapshotCount", len(snapshotIDs))

	r.markSnapshotStepComplete(vm)
	return true, nil
}

// waitForSnapshots checks if EBS snapshots have completed and are ready for use.
// Returns true when all snapshots are in the 'completed' state.
func (r *Migrator) waitForSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking snapshot status", "vm", vm.Name)

	// Get snapshot IDs from AWS tags
	ec2Client := r.getEC2Client()
	snapshotIDString, err := ec2Client.Client.GetSnapshotIDsForVM(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get snapshot IDs from AWS", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if snapshotIDString == "" {
		r.log.Error(nil, "No snapshots found in AWS for VM", "vm", vm.Name)
		return false, fmt.Errorf("no snapshots found in AWS for VM %s", vm.Name)
	}

	snapshotIDs := strings.Split(snapshotIDString, ",")
	r.log.Info("Retrieved snapshot IDs from AWS tags",
		"vm", vm.Name,
		"count", len(snapshotIDs))

	precopy := planapi.Precopy{
		Snapshot: snapshotIDString,
	}

	ready, _, err := r.adpClient.CheckSnapshotReady(vm.Ref, precopy, nil)
	if err != nil {
		r.log.Error(err, "Failed to check snapshot status", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("CheckSnapshotReady completed", "vm", vm.Name, "ready", ready)

	if ready {
		r.log.Info("All snapshots ready, advancing to next phase", "vm", vm.Name)
		if step, found := vm.FindStep(CreateSnapshots); found {
			step.Progress.Completed = 2
		}
		return true, nil
	}

	r.log.Info("Snapshots not yet ready", "vm", vm.Name)
	return false, nil
}

// shareSnapshots shares EBS snapshots with the target AWS account for cross-account migration.
// This phase is only executed when cross-account mode is enabled.
// Returns true when all snapshots have been shared successfully.
func (r *Migrator) shareSnapshots(vm *planapi.VMStatus) (bool, error) {
	ec2Client := r.getEC2Client()

	// Skip if not cross-account mode
	if !ec2Client.Client.IsCrossAccount() {
		r.log.Info("Same-account mode, skipping snapshot sharing", "vm", vm.Name)
		return true, nil
	}

	r.log.Info("Sharing snapshots with target account", "vm", vm.Name)

	// Get target account ID
	targetAccountID, err := ec2Client.Client.GetTargetAccountID()
	if err != nil {
		r.log.Error(err, "Failed to get target account ID", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Target account ID retrieved",
		"vm", vm.Name,
		"targetAccountID", targetAccountID)

	// Get snapshot IDs from AWS tags
	snapshotMap, err := ec2Client.Client.GetSnapshotsForVM(vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get snapshots from AWS", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(snapshotMap) == 0 {
		err := fmt.Errorf("no snapshots found for VM %s", vm.Name)
		r.log.Error(err, "No snapshots to share", "vm", vm.Name)
		return false, err
	}

	// Share each snapshot with target account
	for volumeID, snapshotID := range snapshotMap {
		r.log.Info("Sharing snapshot",
			"vm", vm.Name,
			"snapshotID", snapshotID,
			"volumeID", volumeID,
			"targetAccountID", targetAccountID)

		err := ec2Client.Client.ShareSnapshot(snapshotID, targetAccountID)
		if err != nil {
			r.log.Error(err, "Failed to share snapshot",
				"vm", vm.Name,
				"snapshotID", snapshotID)
			return false, liberr.Wrap(err)
		}

		r.log.Info("Snapshot shared successfully",
			"vm", vm.Name,
			"snapshotID", snapshotID)
	}

	r.log.Info("All snapshots shared with target account",
		"vm", vm.Name,
		"targetAccountID", targetAccountID,
		"snapshotCount", len(snapshotMap))

	return true, nil
}

// removeSnapshots deletes EBS snapshots and created volumes after migration completes or fails.
// Returns true when cleanup is finished. Continues on error to ensure best-effort cleanup.
func (r *Migrator) removeSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Removing EBS snapshots and created volumes", "vm", vm.Name)

	// Clean up snapshots from AWS
	r.cleanupSnapshots(vm)

	// Clean up created EBS volumes if migration failed
	// On success, volumes are now backing PVCs and should not be deleted
	if vm.Error != nil {
		r.cleanupCreatedVolumes(vm)
	}

	r.log.Info("Cleanup complete", "vm", vm.Name)
	return true, nil
}

// cleanupSnapshots removes EBS snapshots from AWS by querying snapshots tagged with the VM name.
func (r *Migrator) cleanupSnapshots(vm *planapi.VMStatus) {
	ec2Client := r.getEC2Client()

	// Get snapshot IDs from AWS tags
	snapshotIDString, err := ec2Client.Client.GetSnapshotIDsForVM(vm.Ref)
	if err != nil {
		r.log.Info("Failed to query snapshots from AWS", "vm", vm.Name, "error", err)
		return
	}

	if snapshotIDString == "" {
		r.log.Info("No snapshots found in AWS to remove", "vm", vm.Name)
		return
	}

	r.log.Info("Removing snapshots", "vm", vm.Name, "snapshots", snapshotIDString)

	_, err = r.adpClient.RemoveSnapshot(vm.Ref, snapshotIDString, nil)
	if err != nil {
		r.log.Error(err, "Failed to remove snapshots", "vm", vm.Name)
		r.log.Info("Continuing despite snapshot cleanup error", "vm", vm.Name)
	} else {
		r.log.Info("Snapshots removed successfully", "vm", vm.Name)
	}
}

// cleanupCreatedVolumes removes EBS volumes that were created from snapshots during migration.
// This is called when migration fails to avoid leaving orphaned volumes in AWS.
func (r *Migrator) cleanupCreatedVolumes(vm *planapi.VMStatus) {
	// Get created volumes from AWS tags
	volumeMapping, err := r.getVolumeIDs(vm)
	if err != nil {
		r.log.V(1).Info("No volumes to clean up", "vm", vm.Name, "error", err)
		return
	}

	if len(volumeMapping) == 0 {
		r.log.Info("No created volumes found in AWS to remove", "vm", vm.Name)
		return
	}

	// Collect all created volume IDs
	volumeIDs := make([]string, 0, len(volumeMapping))
	for _, newVolumeID := range volumeMapping {
		volumeIDs = append(volumeIDs, newVolumeID)
	}

	r.log.Info("Removing created EBS volumes due to migration failure",
		"vm", vm.Name,
		"volumeCount", len(volumeIDs))

	ec2Client := r.getEC2Client()
	err = ec2Client.Client.RemoveVolumes(vm.Ref, volumeIDs)
	if err != nil {
		r.log.Error(err, "Failed to remove created volumes", "vm", vm.Name)
		r.log.Info("Continuing despite volume cleanup error", "vm", vm.Name)
	} else {
		r.log.Info("Created volumes removed successfully", "vm", vm.Name)
	}
}

// markSnapshotStepRunning updates the CreateSnapshots pipeline step to running status.
func (r *Migrator) markSnapshotStepRunning(vm *planapi.VMStatus) error {
	if step, found := vm.FindStep(CreateSnapshots); found {
		if !step.MarkedStarted() {
			step.MarkStarted()
		}
		step.Phase = api.StepRunning
	}
	return nil
}

// snapshotsAlreadyCreated checks whether EBS snapshots already exist for the VM in AWS.
// Returns true if snapshots are found (tagged with VM name), skipping redundant creation.
func (r *Migrator) snapshotsAlreadyCreated(vm *planapi.VMStatus) bool {
	ec2Client := r.getEC2Client()
	snapshotMap, err := ec2Client.Client.GetSnapshotsForVM(vm.Ref)
	if err == nil && len(snapshotMap) > 0 {
		r.log.Info("Snapshots already exist in AWS, skipping creation",
			"vm", vm.Name,
			"snapshotCount", len(snapshotMap))
		if step, found := vm.FindStep(CreateSnapshots); found {
			step.Progress.Completed = 1
		}
		return true
	}
	return false
}

// extractVolumeIDs retrieves all EBS volume IDs attached to the EC2 instance.
// Validates that at least one EBS volume exists and logs any instance store volumes found.
func (r *Migrator) extractVolumeIDs(vm *planapi.VMStatus) ([]string, error) {
	awsInstance, err := r.getAWSInstance(vm)
	if err != nil {
		return nil, err
	}

	stats := inventory.ParseBlockDevices(awsInstance)

	if err := r.validateVolumeExtraction(vm, stats); err != nil {
		return nil, err
	}

	r.log.Info("Found EBS volumes", "vm", vm.Name, "count", len(stats.EBSVolumeIDs))
	return stats.EBSVolumeIDs, nil
}

// getAWSInstance fetches the EC2 instance object from the provider inventory.
func (r *Migrator) getAWSInstance(vm *planapi.VMStatus) (*model.InstanceDetails, error) {
	instance, err := inventory.GetAWSInstance(r.Source.Inventory, vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get VM from inventory", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}
	return instance, nil
}

// validateVolumeExtraction ensures the VM has at least one EBS volume for migration.
// Logs warnings for instance store volumes which cannot be migrated.
func (r *Migrator) validateVolumeExtraction(vm *planapi.VMStatus, stats *inventory.BlockDeviceStats) error {
	if len(stats.InstanceStoreDev) > 0 {
		r.log.Info("WARNING: VM has instance store volumes that cannot be migrated",
			"vm", vm.Name,
			"instanceStoreDevices", stats.InstanceStoreDev,
			"help", "Instance store data will be lost during migration")
	}

	if stats.SkippedCount > 0 {
		r.log.Info("Some block devices were skipped",
			"vm", vm.Name,
			"skippedCount", stats.SkippedCount)
	}

	if len(stats.EBSVolumeIDs) == 0 {
		err := fmt.Errorf("no EBS volumes found for VM %s (found %d instance store, %d skipped)",
			vm.Name, len(stats.InstanceStoreDev), stats.SkippedCount)
		r.log.Error(err, "No EBS volumes to snapshot",
			"vm", vm.Name,
			"instanceStoreCount", len(stats.InstanceStoreDev),
			"skippedCount", stats.SkippedCount,
			"help", "VM must have at least one EBS volume for migration")
		return err
	}

	return nil
}

// markSnapshotStepComplete updates the CreateSnapshots step progress to complete.
func (r *Migrator) markSnapshotStepComplete(vm *planapi.VMStatus) {
	if step, found := vm.FindStep(CreateSnapshots); found {
		step.Progress.Completed = 1
	}
}
