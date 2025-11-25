package migrator

import (
	"context"
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	ec2controller "github.com/kubev2v/forklift/pkg/provider/ec2/controller"
	ec2util "github.com/kubev2v/forklift/pkg/provider/ec2/controller/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// getSnapshotStep returns the CreateSnapshots step for a VM.
func (r *Migrator) getSnapshotStep(vm *planapi.VMStatus) (*planapi.Step, error) {
	return ec2util.GetSnapshotStep(vm)
}

// storeSnapshotIDs saves snapshot IDs to step annotations.
func (r *Migrator) storeSnapshotIDs(vm *planapi.VMStatus, volumeIDs, snapshotIDs []string) error {
	return ec2util.StoreSnapshotIDs(vm, volumeIDs, snapshotIDs, r.log)
}

// getSnapshotIDs returns snapshot ID mappings from step annotations.
func (r *Migrator) getSnapshotIDs(vm *planapi.VMStatus) (map[string]string, error) {
	return ec2util.GetSnapshotIDs(vm, r.log)
}

// createSnapshots creates EBS snapshots and stores their IDs.
func (r *Migrator) createSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Creating EBS volume snapshots", "vm", vm.Name)

	if err := r.markSnapshotStepRunning(vm); err != nil {
		return false, err
	}

	if r.snapshotsAlreadyCreated(vm) {
		return true, nil
	}

	volumeIDs, err := r.extractVolumeIDs(vm)
	if err != nil {
		return false, err
	}

	snapshotIDs, err := r.createAndStoreSnapshots(vm, volumeIDs)
	if err != nil {
		return false, err
	}

	r.log.Info("Snapshots created and stored in step annotations",
		"vm", vm.Name,
		"volumeCount", len(volumeIDs),
		"snapshotCount", len(snapshotIDs))

	r.markSnapshotStepComplete(vm)
	return true, nil
}

// waitForSnapshots checks if EBS snapshots have completed and are ready for use.
// Returns true when all snapshots are in the 'completed' state.
func (r *Migrator) waitForSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Checking snapshot status", "vm", vm.Name)

	snapshotMap, err := r.getSnapshotIDs(vm)
	if err != nil {
		r.log.Error(err, "Failed to get snapshot IDs from step annotations", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	r.log.Info("Retrieved snapshot IDs from step annotations",
		"vm", vm.Name,
		"count", len(snapshotMap))

	if len(snapshotMap) == 0 {
		r.log.Error(nil, "No snapshot IDs found in step annotations",
			"vm", vm.Name)
		return false, fmt.Errorf("no snapshot IDs found in step annotations for VM %s", vm.Name)
	}

	snapshotIDs := []string{}
	for volumeID, snapshotID := range snapshotMap {
		if snapshotID != "" {
			snapshotIDs = append(snapshotIDs, snapshotID)
			r.log.V(2).Info("Found snapshot",
				"vm", vm.Name,
				"volumeID", volumeID,
				"snapshotID", snapshotID)
		}
	}

	if len(snapshotIDs) == 0 {
		r.log.Error(nil, "No snapshot IDs found", "vm", vm.Name)
		return false, fmt.Errorf("no snapshot IDs found for VM %s", vm.Name)
	}

	snapshotIDString := strings.Join(snapshotIDs, ",")
	r.log.Info("Calling CheckSnapshotReady",
		"vm", vm.Name,
		"count", len(snapshotIDs),
		"snapshots", snapshotIDString)

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

// getObjectKeys extracts all top-level keys from an unstructured object map.
func getObjectKeys(obj map[string]interface{}) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

// removeSnapshots deletes EBS snapshots after migration completes.
// Returns true when cleanup is finished. Continues on error to ensure best-effort cleanup.
func (r *Migrator) removeSnapshots(vm *planapi.VMStatus) (bool, error) {
	r.log.Info("Removing EBS snapshots", "vm", vm.Name)

	step, err := r.getSnapshotStep(vm)
	if err != nil {
		r.log.Info("No snapshots to remove - CreateSnapshots step not found", "vm", vm.Name)
		return true, nil //nolint:nilerr
	}

	if step.Annotations == nil {
		r.log.Info("No snapshots found in annotations", "vm", vm.Name)
		return true, nil
	}

	var snapshotIDs []string
	for key, value := range step.Annotations {
		if value != "" && strings.HasPrefix(key, "snapshot-") {
			snapshotIDs = append(snapshotIDs, value)
		}
	}

	if len(snapshotIDs) == 0 {
		r.log.Info("No snapshots to remove", "vm", vm.Name)
		return true, nil
	}

	snapshotIDString := strings.Join(snapshotIDs, ",")
	r.log.Info("Removing snapshots", "vm", vm.Name, "snapshots", snapshotIDString)

	_, err = r.adpClient.RemoveSnapshot(vm.Ref, snapshotIDString, nil)
	if err != nil {
		r.log.Error(err, "Failed to remove snapshots", "vm", vm.Name)
		r.log.Info("Continuing despite snapshot cleanup error", "vm", vm.Name)
	} else {
		r.log.Info("Snapshots removed successfully", "vm", vm.Name)
		for key := range step.Annotations {
			if strings.HasPrefix(key, "snapshot-") {
				delete(step.Annotations, key)
			}
		}
	}

	ec2Ensurer := r.getEnsurer()
	err = ec2Ensurer.CleanupPopulatorSecret(context.TODO(), vm)
	if err != nil {
		r.log.Error(err, "Failed to cleanup populator secrets", "vm", vm.Name)
	}

	r.log.Info("Cleanup complete", "vm", vm.Name)
	return true, nil
}

// volumeExtraction holds parsed EC2 block device mappings, separating migratable
// EBS volumes from non-migratable instance store volumes.
type volumeExtraction struct {
	// ebsVolumeIDs are EBS volumes that can be migrated via snapshots
	ebsVolumeIDs []string
	// instanceStoreDev are ephemeral instance store device names (cannot be migrated)
	instanceStoreDev []string
	// skippedCount tracks unparseable block devices
	skippedCount int
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

// snapshotsAlreadyCreated checks whether EBS snapshots already exist for the VM.
// Returns true if snapshots are found, skipping redundant creation.
func (r *Migrator) snapshotsAlreadyCreated(vm *planapi.VMStatus) bool {
	existingSnapshots, err := r.getSnapshotIDs(vm)
	if err == nil && len(existingSnapshots) > 0 {
		r.log.Info("Snapshots already created, skipping creation",
			"vm", vm.Name,
			"snapshotCount", len(existingSnapshots))
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

	extraction := r.parseBlockDevices(vm, awsInstance)

	if err := r.validateVolumeExtraction(vm, extraction); err != nil {
		return nil, err
	}

	r.log.Info("Found EBS volumes", "vm", vm.Name, "count", len(extraction.ebsVolumeIDs))
	return extraction.ebsVolumeIDs, nil
}

// getAWSInstance fetches the EC2 instance object from the provider inventory.
func (r *Migrator) getAWSInstance(vm *planapi.VMStatus) (map[string]interface{}, error) {
	vmObj := &unstructured.Unstructured{}
	vmObj.SetUnstructuredContent(map[string]interface{}{"kind": "Instance"})
	err := r.Source.Inventory.Find(vmObj, vm.Ref)
	if err != nil {
		r.log.Error(err, "Failed to get VM from inventory", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}

	r.log.Info("VM object from inventory",
		"vm", vm.Name,
		"vmID", vm.ID,
		"objectKeys", getObjectKeys(vmObj.Object))

	awsInstanceObj, err := ec2controller.GetAWSObject(vmObj)
	if err != nil {
		r.log.Error(err, "VM inventory missing AWS object data",
			"vm", vm.Name,
			"objectKeys", getObjectKeys(vmObj.Object))
		return nil, err
	}

	return awsInstanceObj, nil
}

// parseBlockDevices extracts EBS volume and instance store information from block device mappings.
// Separates EBS volumes (which can be migrated) from instance store volumes (which cannot).
func (r *Migrator) parseBlockDevices(vm *planapi.VMStatus, awsInstance map[string]interface{}) *volumeExtraction {
	result := &volumeExtraction{
		ebsVolumeIDs:     []string{},
		instanceStoreDev: []string{},
	}

	blockDevices, found, _ := unstructured.NestedSlice(awsInstance, "BlockDeviceMappings")

	if !found || len(blockDevices) == 0 {
		return result
	}

	r.log.Info("Found block device mappings", "vm", vm.Name, "count", len(blockDevices))

	if len(blockDevices) > 0 && r.log.V(2).Enabled() {
		if firstDev, ok := blockDevices[0].(map[string]interface{}); ok {
			r.log.V(2).Info("Sample block device structure",
				"vm", vm.Name,
				"deviceKeys", getObjectKeys(firstDev))
		}
	}

	for _, deviceRaw := range blockDevices {
		device, ok := deviceRaw.(map[string]interface{})
		if !ok {
			result.skippedCount++
			continue
		}

		deviceName, _, _ := unstructured.NestedString(device, "DeviceName")

		if volumeID := r.extractEBSVolumeID(device); volumeID != "" {
			result.ebsVolumeIDs = append(result.ebsVolumeIDs, volumeID)
			r.log.V(1).Info("Found EBS volume",
				"vm", vm.Name,
				"device", deviceName,
				"volumeID", volumeID)
		} else if r.isInstanceStore(device) {
			virtualName, _, _ := unstructured.NestedString(device, "VirtualName")
			r.log.Info("Instance store volume detected",
				"vm", vm.Name,
				"device", deviceName,
				"virtualName", virtualName)
			result.instanceStoreDev = append(result.instanceStoreDev, deviceName)
		} else {
			r.log.V(1).Info("Block device has no EBS mapping",
				"vm", vm.Name,
				"device", deviceName)
			result.skippedCount++
		}
	}

	return result
}

// extractEBSVolumeID extracts EBS volume ID from a device, returns empty string if not EBS
// extractEBSVolumeID retrieves the EBS volume ID from a block device mapping entry.
// Returns an empty string if the device is not an EBS volume.
func (r *Migrator) extractEBSVolumeID(device map[string]interface{}) string {
	if r.isInstanceStore(device) {
		return ""
	}

	ebs, found, _ := unstructured.NestedMap(device, "Ebs")
	if !found {
		return ""
	}

	volumeID, _, _ := unstructured.NestedString(ebs, "VolumeId")
	return volumeID
}

// isInstanceStore determines whether a block device is an instance store volume.
func (r *Migrator) isInstanceStore(device map[string]interface{}) bool {
	virtualName, hasVirtualName, _ := unstructured.NestedString(device, "VirtualName")
	return hasVirtualName && virtualName != ""
}

// validateVolumeExtraction ensures the VM has at least one EBS volume for migration.
// Logs warnings for instance store volumes which cannot be migrated.
func (r *Migrator) validateVolumeExtraction(vm *planapi.VMStatus, extraction *volumeExtraction) error {
	if len(extraction.instanceStoreDev) > 0 {
		r.log.Info("WARNING: VM has instance store volumes that cannot be migrated",
			"vm", vm.Name,
			"instanceStoreDevices", extraction.instanceStoreDev,
			"help", "Instance store data will be lost during migration")
	}

	if extraction.skippedCount > 0 {
		r.log.Info("Some block devices were skipped",
			"vm", vm.Name,
			"skippedCount", extraction.skippedCount)
	}

	if len(extraction.ebsVolumeIDs) == 0 {
		err := fmt.Errorf("no EBS volumes found for VM %s (found %d instance store, %d skipped)",
			vm.Name, len(extraction.instanceStoreDev), extraction.skippedCount)
		r.log.Error(err, "No EBS volumes to snapshot",
			"vm", vm.Name,
			"instanceStoreCount", len(extraction.instanceStoreDev),
			"skippedCount", extraction.skippedCount,
			"help", "VM must have at least one EBS volume for migration")
		return err
	}

	return nil
}

// createAndStoreSnapshots creates EBS snapshots via the adapter client and stores their IDs.
// Validates that the number of returned snapshots matches the number of volumes.
func (r *Migrator) createAndStoreSnapshots(vm *planapi.VMStatus, volumeIDs []string) ([]string, error) {
	snapshotIDs, _, err := r.adpClient.CreateSnapshot(vm.Ref, nil)
	if err != nil {
		r.log.Error(err, "Failed to create snapshots", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}

	snapshotIDList := strings.Split(snapshotIDs, ",")
	if len(snapshotIDList) != len(volumeIDs) {
		err := fmt.Errorf("snapshot count (%d) doesn't match volume count (%d)",
			len(snapshotIDList), len(volumeIDs))
		r.log.Error(err, "Snapshot/volume mismatch", "vm", vm.Name)
		return nil, err
	}

	if err := r.storeSnapshotIDs(vm, volumeIDs, snapshotIDList); err != nil {
		r.log.Error(err, "Failed to store snapshot IDs in step annotations", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}

	return snapshotIDList, nil
}

// markSnapshotStepComplete updates the CreateSnapshots step progress to complete.
func (r *Migrator) markSnapshotStepComplete(vm *planapi.VMStatus) {
	if step, found := vm.FindStep(CreateSnapshots); found {
		step.Progress.Completed = 1
	}
}
