package resolver

import (
	"fmt"
	"strings"

	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
)

// DiskType classifies the vSphere backing type for a VM disk.
type DiskType string

const (
	DiskTypeVVol DiskType = "vvol"
	DiskTypeRDM  DiskType = "rdm"
	DiskTypeVMDK DiskType = "vmdk"
)

// DiskBacking contains disk backing information as returned by govmomi.
type DiskBacking struct {
	// VVolID is non-empty when the disk is VVol-backed (govmomi BackingObjectId).
	VVolID string
	// IsRDM is true when the disk is a Raw Device Mapping.
	IsRDM bool
	// DeviceName is the underlying device path or VMDK file name.
	DeviceName string
	// LunUuid is the unique LUN identifier (SCSI 83h / NAA). Used for storage resolution; required for RDM.
	LunUuid string
}

// DetectDiskType returns the DiskType for this backing.
func DetectDiskType(b *DiskBacking) DiskType {
	switch {
	case b.VVolID != "":
		return DiskTypeVVol
	case b.IsRDM:
		return DiskTypeRDM
	default:
		return DiskTypeVMDK
	}
}

// DiskBackingFromDevices finds the disk matching diskFile in a VM's device
// list and returns its backing info (VVol / RDM / VMDK).
func DiskBackingFromDevices(devices []types.BaseVirtualDevice, diskFile string) (*DiskBacking, error) {
	log := klog.Background().WithName("disk-backing")
	normalizedPath := strings.ToLower(diskFile)

	for _, device := range devices {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}

		switch backing := disk.Backing.(type) {
		case *types.VirtualDiskFlatVer2BackingInfo:
			if !strings.Contains(strings.ToLower(backing.FileName), normalizedPath) &&
				!strings.Contains(normalizedPath, strings.ToLower(backing.FileName)) {
				if !diskPathMatches(backing.FileName, diskFile) {
					continue
				}
			}
			if backing.BackingObjectId != "" {
				log.V(2).Info("disk is VVol-backed", "vmdk", diskFile, "backing_object_id", backing.BackingObjectId)
				return &DiskBacking{
					VVolID:     backing.BackingObjectId,
					DeviceName: backing.FileName,
				}, nil
			}
			log.V(2).Info("disk is VMDK-backed", "vmdk", diskFile)
			return &DiskBacking{
				DeviceName: backing.FileName,
			}, nil

		case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
			if !strings.Contains(strings.ToLower(backing.FileName), normalizedPath) &&
				!strings.Contains(normalizedPath, strings.ToLower(backing.FileName)) {
				if !diskPathMatches(backing.FileName, diskFile) {
					continue
				}
			}
			log.V(2).Info("disk is RDM-backed", "vmdk", diskFile, "device", backing.DeviceName, "lunUuid", backing.LunUuid)
			return &DiskBacking{
				IsRDM:      true,
				DeviceName: backing.DeviceName,
				LunUuid:    backing.LunUuid,
			}, nil
		}
	}

	return nil, fmt.Errorf("disk %q not found in device list", diskFile)
}

// FindMatchedDisk returns the *types.VirtualDisk whose backing FileName matches diskFile.
// Uses the same matching logic as DiskBackingFromDevices. Returns nil if not found.
func FindMatchedDisk(devices []types.BaseVirtualDevice, diskFile string) *types.VirtualDisk {
	normalizedPath := strings.ToLower(diskFile)
	for _, device := range devices {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}
		switch backing := disk.Backing.(type) {
		case *types.VirtualDiskFlatVer2BackingInfo:
			if strings.Contains(strings.ToLower(backing.FileName), normalizedPath) ||
				strings.Contains(normalizedPath, strings.ToLower(backing.FileName)) ||
				diskPathMatches(backing.FileName, diskFile) {
				return disk
			}
		case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
			if strings.Contains(strings.ToLower(backing.FileName), normalizedPath) ||
				strings.Contains(normalizedPath, strings.ToLower(backing.FileName)) ||
				diskPathMatches(backing.FileName, diskFile) {
				return disk
			}
		}
	}
	return nil
}

func diskPathMatches(path1, path2 string) bool {
	normalize := func(p string) string {
		p = strings.TrimSpace(p)
		p = strings.ToLower(p)
		p = strings.ReplaceAll(p, "[", "")
		p = strings.ReplaceAll(p, "]", "")
		return p
	}
	return normalize(path1) == normalize(path2)
}
