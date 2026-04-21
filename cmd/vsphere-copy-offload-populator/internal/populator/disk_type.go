package populator

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"k8s.io/klog/v2"
)

// DiskType represents the type of disk backing in vSphere
type DiskType string

const (
	// DiskTypeVVol represents a Virtual Volume backed disk
	DiskTypeVVol DiskType = "vvol"
	// DiskTypeRDM represents a Raw Device Mapping disk
	DiskTypeRDM DiskType = "rdm"
	// DiskTypeVMDK represents a traditional VMDK on datastore (default)
	DiskTypeVMDK DiskType = "vmdk"
)

// PopulatorSettings controls which optimized methods are disabled
// All methods are enabled by default unless explicitly disabled
// VMDK/Xcopy cannot be disabled as it's the default fallback
type populatorSettings struct {
	// VVolDisabled disables VVol optimization when disk is VVol-backed
	VVolDisabled bool
	// RDMDisabled disables RDM optimization when disk is RDM-backed
	RDMDisabled bool
	// Note: VMDK cannot be disabled as it's the default fallback
}

func detectDiskType(backing *vmware.DiskBacking) DiskType {
	log := klog.Background().WithName("copy-offload").WithName("disk-type")

	switch {
	case backing.VVolId != "":
		log.Info("detected VVol disk", "vvolId", backing.VVolId)
		return DiskTypeVVol
	case backing.IsRDM:
		log.Info("detected RDM disk", "device", backing.DeviceName)
		return DiskTypeRDM
	default:
		log.Info("detected VMDK disk")
		return DiskTypeVMDK
	}
}
