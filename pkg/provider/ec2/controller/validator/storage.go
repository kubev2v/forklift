package validator

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/mapping"
)

// validateStorage validates EBS volumes exist and detects unsupported instance store volumes.
// Ensures VM has at least one EBS volume (required for migration). Instance store volumes
// are ephemeral and cannot be migrated - logged as warnings but don't fail validation.
// Returns error if no EBS volumes found or no block devices attached.
func (r *Validator) validateStorage(vmRef ref.Ref) (ok bool, err error) {
	awsInstance, err := r.getAWSInstance(vmRef)
	if err != nil {
		r.log.Error(err, "Failed to get AWS instance from inventory", "vm", vmRef.String())
		return false, err
	}

	blockDevices, found := getBlockDevices(awsInstance)
	if !found {
		r.log.Info("VM has no block devices", "vm", vmRef.String())
		return false, fmt.Errorf("VM has no block devices attached - cannot migrate VM without storage")
	}

	ebsVolumeCount := 0
	instanceStoreCount := 0
	var instanceStoreDevices []string

	for _, dev := range blockDevices {
		deviceName := inventory.GetDeviceName(dev)

		// Check if this is an instance store volume (not supported for migration)
		if inventory.IsInstanceStore(dev) {
			instanceStoreCount++
			instanceStoreDevices = append(instanceStoreDevices, deviceName)
			r.log.Info("Instance store volume detected (not supported for migration)",
				"vm", vmRef.String(),
				"device", deviceName,
				"virtualName", inventory.GetVirtualName(dev))
			continue
		}

		// Check if this is an EBS volume
		if inventory.IsEBSVolume(dev) {
			ebsVolumeCount++
		}
	}

	if ebsVolumeCount == 0 {
		r.log.Info("VM has no EBS volumes to migrate",
			"vm", vmRef.String(),
			"totalBlockDevices", len(blockDevices),
			"instanceStoreCount", instanceStoreCount)
		return false, fmt.Errorf("VM has no EBS volumes (only instance store) - cannot migrate")
	}

	if instanceStoreCount > 0 {
		r.log.Info("WARNING: VM has instance store volumes that will not be migrated",
			"vm", vmRef.String(),
			"instanceStoreDevices", instanceStoreDevices,
			"ebsVolumeCount", ebsVolumeCount)
	}

	r.log.Info("Storage validation passed",
		"vm", vmRef.String(),
		"ebsVolumes", ebsVolumeCount,
		"instanceStoreVolumes", instanceStoreCount)

	return true, nil
}

// StorageMapped validates all EBS volume types have storage mappings configured.
// Checks each EBS volume's VolumeType (gp2, gp3, io1, etc.) against storage mapping.
// Returns error listing any unmapped volume types that would block migration.
func (r *Validator) StorageMapped(vmRef ref.Ref) (bool, error) {
	awsInstance, err := r.getAWSInstance(vmRef)
	if err != nil {
		return false, err
	}

	blockDevices, found := getBlockDevices(awsInstance)
	if !found {
		return true, nil
	}

	for _, dev := range blockDevices {
		volumeID := inventory.ExtractEBSVolumeID(dev)
		if volumeID == "" {
			continue
		}

		volumeType := inventory.GetVolumeType(r.Source.Inventory, volumeID)
		if volumeType == "" {
			continue
		}

		if !mapping.HasStorageMapping(r.Map.Storage, volumeType) {
			return false, nil
		}
	}

	return true, nil
}

// UnSupportedDisks returns unsupported disk identifiers.
func (r *Validator) UnSupportedDisks(vmRef ref.Ref) ([]string, error) {
	awsInstance, err := r.getAWSInstance(vmRef)
	if err != nil {
		r.log.V(1).Info("Failed to get AWS instance from inventory", "vm", vmRef.String())
		return nil, err
	}

	blockDevices, found := getBlockDevices(awsInstance)
	if !found {
		r.log.V(1).Info("No block device mappings found in inventory", "vm", vmRef.String())
		return nil, nil
	}

	var unsupported []string
	for _, dev := range blockDevices {
		if inventory.IsInstanceStore(dev) {
			deviceName := inventory.GetDeviceName(dev)
			virtualName := inventory.GetVirtualName(dev)
			unsupported = append(unsupported, fmt.Sprintf("%s (instance store: %s)", deviceName, virtualName))
			r.log.Info("Unsupported instance store volume",
				"vm", vmRef.String(),
				"device", deviceName,
				"virtualName", virtualName)
		}
	}

	if len(unsupported) > 0 {
		r.log.Info("Found unsupported disks",
			"vm", vmRef.String(),
			"count", len(unsupported),
			"devices", unsupported)
	}

	return unsupported, nil
}
