package inventory

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/web"
)

// IsInstanceStore determines whether a block device is an instance store volume.
// Instance store volumes are ephemeral and cannot be migrated.
func IsInstanceStore(device model.InstanceBlockDeviceMapping) bool {
	return device.VirtualName != nil && *device.VirtualName != ""
}

// IsEBSVolume determines whether a block device is an EBS volume.
func IsEBSVolume(device model.InstanceBlockDeviceMapping) bool {
	return !IsInstanceStore(device) && device.Ebs != nil && device.Ebs.VolumeId != nil
}

// ExtractEBSVolumeID retrieves the EBS volume ID from a block device mapping entry.
// Returns an empty string if the device is not an EBS volume.
func ExtractEBSVolumeID(device model.InstanceBlockDeviceMapping) string {
	if !IsEBSVolume(device) {
		return ""
	}
	return *device.Ebs.VolumeId
}

// GetDeviceName returns the device name or empty string if nil.
func GetDeviceName(device model.InstanceBlockDeviceMapping) string {
	if device.DeviceName != nil {
		return *device.DeviceName
	}
	return ""
}

// GetVirtualName returns the virtual name (for instance store) or empty string if nil.
func GetVirtualName(device model.InstanceBlockDeviceMapping) string {
	if device.VirtualName != nil {
		return *device.VirtualName
	}
	return ""
}

// GetVolume fetches an EBS volume from the provider inventory.
func GetVolume(inv Inventory, volumeID string) (*web.Volume, error) {
	volume := &web.Volume{}
	volumeRef := ref.Ref{ID: volumeID}
	err := inv.Find(volume, volumeRef)
	if err != nil {
		return nil, err
	}
	return volume, nil
}

// GetVolumeType returns the EBS volume type (gp2, gp3, io1, etc.) from inventory.
// Returns empty string if the volume cannot be found or has no type.
func GetVolumeType(inv Inventory, volumeID string) string {
	if volumeID == "" {
		return ""
	}

	volume, err := GetVolume(inv, volumeID)
	if err != nil {
		return ""
	}

	if volume.Object == nil {
		return ""
	}

	return string(volume.Object.VolumeType)
}

// GetVolumeSize returns the EBS volume size in GiB from inventory.
// Returns 0 if the volume cannot be found or has no size.
func GetVolumeSize(inv Inventory, volumeID string) int64 {
	if volumeID == "" {
		return 0
	}

	volume, err := GetVolume(inv, volumeID)
	if err != nil {
		return 0
	}

	if volume.Object == nil || volume.Object.Size == nil {
		return 0
	}

	return int64(*volume.Object.Size)
}

// BlockDeviceStats holds statistics about block device parsing results.
type BlockDeviceStats struct {
	EBSVolumeIDs     []string // List of EBS volume IDs
	InstanceStoreDev []string // List of instance store device names
	SkippedCount     int      // Count of devices that were neither EBS nor instance store
}

// ParseBlockDevices extracts EBS volume and instance store information from block device mappings.
// Separates EBS volumes (which can be migrated) from instance store volumes (which cannot).
func ParseBlockDevices(instance *model.InstanceDetails) *BlockDeviceStats {
	stats := &BlockDeviceStats{
		EBSVolumeIDs:     []string{},
		InstanceStoreDev: []string{},
	}

	devices, found := GetBlockDevices(instance)
	if !found {
		return stats
	}

	for _, device := range devices {
		deviceName := GetDeviceName(device)

		if volumeID := ExtractEBSVolumeID(device); volumeID != "" {
			stats.EBSVolumeIDs = append(stats.EBSVolumeIDs, volumeID)
		} else if IsInstanceStore(device) {
			stats.InstanceStoreDev = append(stats.InstanceStoreDev, deviceName)
		} else {
			stats.SkippedCount++
		}
	}

	return stats
}
