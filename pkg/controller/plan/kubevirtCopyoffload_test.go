package plan

import (
	"testing"

	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	vsphere "github.com/kubev2v/forklift/pkg/controller/plan/adapter/vsphere"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockStorageMapper implements StorageMapper interface for testing
type mockStorageMapper struct {
	maps *vsphere.PlanDiskStorageMaps
}

func (m *mockStorageMapper) IsCopyOffload(diskFile string, vmID string) bool {
	if m.maps == nil {
		return false
	}
	vmMap, found := m.maps.GetVMMap(vmID)
	if !found {
		return false
	}
	return vmMap.IsCopyOffload(diskFile)
}

func (m *mockStorageMapper) IsPVCCopyOffload(pvc *core.PersistentVolumeClaim) bool {
	diskSource, ok := pvc.Annotations[planbase.AnnDiskSource]
	if !ok {
		diskSource, ok = pvc.Annotations["copy-offload"]
		if !ok {
			return false
		}
	}
	vmID, ok := pvc.Labels["vmID"]
	if !ok {
		return false
	}
	return m.IsCopyOffload(diskSource, vmID)
}

func (m *mockStorageMapper) IsAnyPVCCopyOffload(pvcs []*core.PersistentVolumeClaim) bool {
	for _, pvc := range pvcs {
		if m.IsPVCCopyOffload(pvc) {
			return true
		}
	}
	return false
}

func TestKubeVirt_IsCopyOffload(t *testing.T) {
	// Setup disk storage maps
	diskStorageMaps := &vsphere.PlanDiskStorageMaps{
		VMs: map[string]*vsphere.VMDiskStorageMap{
			"vm-1": {
				VMID:   "vm-1",
				VMName: "test-vm",
				Disks: map[string]*vsphere.DiskStorageInfo{
					"[ds1] vm/disk0.vmdk": {
						DiskFile:   "[ds1] vm/disk0.vmdk",
						CopyMethod: vsphere.CopyMethodCopyOffload,
					},
					"[ds1] vm/disk1.vmdk": {
						DiskFile:   "[ds1] vm/disk1.vmdk",
						CopyMethod: vsphere.CopyMethodCDI,
					},
				},
			},
			"vm-2": {
				VMID:   "vm-2",
				VMName: "vm2",
				Disks: map[string]*vsphere.DiskStorageInfo{
					"[ds2] vm/disk0.vmdk": {
						DiskFile:   "[ds2] vm/disk0.vmdk",
						CopyMethod: vsphere.CopyMethodVirtV2v,
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		pvc  *core.PersistentVolumeClaim
		want bool
	}{
		{
			name: "PVC with AnnDiskSource matching copy-offload disk returns true",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-1",
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[ds1] vm/disk0.vmdk",
					},
				},
			},
			want: true,
		},
		{
			name: "PVC with AnnDiskSource matching CDI disk returns false",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-1",
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[ds1] vm/disk1.vmdk",
					},
				},
			},
			want: false,
		},
		{
			name: "PVC with AnnDiskSource matching virt-v2v disk returns false",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-2",
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[ds2] vm/disk0.vmdk",
					},
				},
			},
			want: false,
		},
		{
			name: "PVC without AnnDiskSource returns false",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-1",
					},
					Annotations: map[string]string{},
				},
			},
			want: false,
		},
		{
			name: "PVC with legacy copy-offload annotation returns true (backwards compat)",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-1",
					},
					Annotations: map[string]string{
						"copy-offload": "[ds1] vm/disk0.vmdk",
					},
				},
			},
			want: true,
		},
		{
			name: "PVC without vmID label returns false",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels:    map[string]string{},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[ds1] vm/disk0.vmdk",
					},
				},
			},
			want: false,
		},
		{
			name: "PVC with non-existent VM returns false",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-nonexistent",
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[ds1] vm/disk0.vmdk",
					},
				},
			},
			want: false,
		},
		{
			name: "PVC with non-existent disk in VM returns false",
			pvc: &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-pvc",
					Namespace: "test",
					Labels: map[string]string{
						"vmID": "vm-1",
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[ds1] vm/nonexistent.vmdk",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := &KubeVirt{
				StorageMapper: &mockStorageMapper{maps: diskStorageMaps},
			}
			got := kv.StorageMapper.IsPVCCopyOffload(tt.pvc)
			if got != tt.want {
				t.Errorf("StorageMapper.IsPVCCopyOffload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKubeVirt_IsCopyOffload_NilMaps(t *testing.T) {
	// Test with nil storage mapper
	kv := &KubeVirt{
		StorageMapper: nil,
	}

	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Labels: map[string]string{
				"vmID": "vm-1",
			},
			Annotations: map[string]string{
				planbase.AnnDiskSource: "[ds1] vm/disk0.vmdk",
			},
		},
	}

	got := false
	if kv.StorageMapper != nil {
		got = kv.StorageMapper.IsPVCCopyOffload(pvc)
	}
	if got != false {
		t.Errorf("StorageMapper.IsPVCCopyOffload() with nil maps = %v, want false", got)
	}
}

func TestKubeVirt_IsCopyOffloadAny(t *testing.T) {
	diskStorageMaps := &vsphere.PlanDiskStorageMaps{
		VMs: map[string]*vsphere.VMDiskStorageMap{
			"vm-1": {
				VMID: "vm-1",
				Disks: map[string]*vsphere.DiskStorageInfo{
					"[ds1] disk0.vmdk": {
						CopyMethod: vsphere.CopyMethodCopyOffload,
					},
					"[ds1] disk1.vmdk": {
						CopyMethod: vsphere.CopyMethodCDI,
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		pvcs []*core.PersistentVolumeClaim
		want bool
	}{
		{
			name: "list with one copy-offload PVC returns true",
			pvcs: []*core.PersistentVolumeClaim{
				{
					ObjectMeta: meta.ObjectMeta{
						Labels:      map[string]string{"vmID": "vm-1"},
						Annotations: map[string]string{planbase.AnnDiskSource: "[ds1] disk0.vmdk"},
					},
				},
				{
					ObjectMeta: meta.ObjectMeta{
						Labels:      map[string]string{"vmID": "vm-1"},
						Annotations: map[string]string{planbase.AnnDiskSource: "[ds1] disk1.vmdk"},
					},
				},
			},
			want: true,
		},
		{
			name: "list with no copy-offload PVCs returns false",
			pvcs: []*core.PersistentVolumeClaim{
				{
					ObjectMeta: meta.ObjectMeta{
						Labels:      map[string]string{"vmID": "vm-1"},
						Annotations: map[string]string{planbase.AnnDiskSource: "[ds1] disk1.vmdk"},
					},
				},
			},
			want: false,
		},
		{
			name: "empty list returns false",
			pvcs: []*core.PersistentVolumeClaim{},
			want: false,
		},
		{
			name: "nil list returns false",
			pvcs: nil,
			want: false,
		},
		{
			name: "list with all copy-offload PVCs returns true",
			pvcs: []*core.PersistentVolumeClaim{
				{
					ObjectMeta: meta.ObjectMeta{
						Labels:      map[string]string{"vmID": "vm-1"},
						Annotations: map[string]string{planbase.AnnDiskSource: "[ds1] disk0.vmdk"},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := &KubeVirt{
				StorageMapper: &mockStorageMapper{maps: diskStorageMaps},
			}
			got := kv.StorageMapper.IsAnyPVCCopyOffload(tt.pvcs)
			if got != tt.want {
				t.Errorf("StorageMapper.IsAnyPVCCopyOffload() = %v, want %v", got, tt.want)
			}
		})
	}
}
