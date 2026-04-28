package vsphere

import "github.com/kubev2v/forklift/pkg/controller/provider/model/base"

// DiskCopyMethod indicates how disk data is copied during migration
type DiskCopyMethod string

const (
	CopyMethodVirtV2v     DiskCopyMethod = "virt-v2v"
	CopyMethodCDI         DiskCopyMethod = "cdi-vddk"
	CopyMethodCopyOffload DiskCopyMethod = "copy-offload"
)

// DiskStorageInfo contains storage mapping and copy method for a single disk
type DiskStorageInfo struct {
	DiskFile           string
	DiskKey            int32
	DiskIndex          int
	SourceDatastore    base.Ref
	TargetStorageClass string
	CopyMethod         DiskCopyMethod
}

// VMDiskStorageMap contains disk storage information for a single VM
type VMDiskStorageMap struct {
	VMID         string
	VMName       string
	Disks        map[string]*DiskStorageInfo // Keyed by baseVolume(disk.File)
	DisksByIndex []*DiskStorageInfo
}

// IsCopyOffload checks if a specific disk uses copy-offload
func (m *VMDiskStorageMap) IsCopyOffload(diskFile string) bool {
	if info, found := m.Disks[diskFile]; found {
		return info.CopyMethod == CopyMethodCopyOffload
	}
	return false
}

// GetCopyMethod returns the copy method for a disk
func (m *VMDiskStorageMap) GetCopyMethod(diskFile string) DiskCopyMethod {
	if info, found := m.Disks[diskFile]; found {
		return info.CopyMethod
	}
	return ""
}

// PlanDiskStorageMaps contains disk storage maps for all VMs in a plan
type PlanDiskStorageMaps struct {
	VMs map[string]*VMDiskStorageMap // Keyed by VM ID
}

// GetVMMap retrieves the disk storage map for a specific VM
func (p *PlanDiskStorageMaps) GetVMMap(vmID string) (*VMDiskStorageMap, bool) {
	m, found := p.VMs[vmID]
	return m, found
}
