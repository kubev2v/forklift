package vsphere

import (
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
)

// StorageMapper provides disk copy method information for vSphere VMs.
type StorageMapper struct {
	*plancontext.Context
	maps *PlanDiskStorageMaps
}

// IsCopyOffload checks if a specific disk uses copy-offload method.
// Lazily builds disk storage maps on first call and caches them.
func (r *StorageMapper) IsCopyOffload(diskFile string, vmID string) bool {
	// Lazy build maps on first call
	if r.maps == nil {
		builder := &Builder{Context: r.Context}
		maps, err := builder.buildPlanDiskStorageMaps()
		if err != nil {
			return false
		}
		r.maps = maps
	}

	if r.maps == nil {
		return false
	}

	vmMap, found := r.maps.GetVMMap(vmID)
	if !found {
		return false
	}

	return vmMap.IsCopyOffload(diskFile)
}

// IsPVCCopyOffload checks if a specific PVC uses copy-offload by extracting disk metadata.
func (r *StorageMapper) IsPVCCopyOffload(pvc *core.PersistentVolumeClaim) bool {
	// Get disk source from standard annotation
	diskSource, ok := pvc.Annotations[planbase.AnnDiskSource]
	if !ok {
		// Fallback: check legacy copy-offload annotation for backwards compatibility
		diskSource, ok = pvc.Annotations["copy-offload"]
		if !ok {
			return false
		}
	}

	// Get VM ID from PVC labels
	vmID, ok := pvc.Labels["vmID"]
	if !ok {
		return false
	}

	return r.IsCopyOffload(diskSource, vmID)
}

// IsAnyPVCCopyOffload checks if any PVC in the list uses copy-offload.
func (r *StorageMapper) IsAnyPVCCopyOffload(pvcs []*core.PersistentVolumeClaim) bool {
	for _, pvc := range pvcs {
		if r.IsPVCCopyOffload(pvc) {
			return true
		}
	}
	return false
}
