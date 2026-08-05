package resolver

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
