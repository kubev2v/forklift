package vsphere

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
)

func TestVMDiskStorageMap_IsCopyOffload(t *testing.T) {
	tests := []struct {
		name     string
		vmMap    *VMDiskStorageMap
		diskFile string
		want     bool
	}{
		{
			name: "disk uses copy-offload",
			vmMap: &VMDiskStorageMap{
				VMID:   "vm-1",
				VMName: "test-vm",
				Disks: map[string]*DiskStorageInfo{
					"[datastore1] vm/disk.vmdk": {
						DiskFile:   "[datastore1] vm/disk.vmdk",
						CopyMethod: CopyMethodCopyOffload,
					},
				},
			},
			diskFile: "[datastore1] vm/disk.vmdk",
			want:     true,
		},
		{
			name: "disk uses CDI",
			vmMap: &VMDiskStorageMap{
				VMID:   "vm-1",
				VMName: "test-vm",
				Disks: map[string]*DiskStorageInfo{
					"[datastore1] vm/disk.vmdk": {
						DiskFile:   "[datastore1] vm/disk.vmdk",
						CopyMethod: CopyMethodCDI,
					},
				},
			},
			diskFile: "[datastore1] vm/disk.vmdk",
			want:     false,
		},
		{
			name: "disk uses virt-v2v",
			vmMap: &VMDiskStorageMap{
				VMID:   "vm-1",
				VMName: "test-vm",
				Disks: map[string]*DiskStorageInfo{
					"[datastore1] vm/disk.vmdk": {
						DiskFile:   "[datastore1] vm/disk.vmdk",
						CopyMethod: CopyMethodVirtV2v,
					},
				},
			},
			diskFile: "[datastore1] vm/disk.vmdk",
			want:     false,
		},
		{
			name: "disk not found",
			vmMap: &VMDiskStorageMap{
				VMID:   "vm-1",
				VMName: "test-vm",
				Disks:  map[string]*DiskStorageInfo{},
			},
			diskFile: "[datastore1] vm/disk.vmdk",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.vmMap.IsCopyOffload(tt.diskFile)
			if got != tt.want {
				t.Errorf("VMDiskStorageMap.IsCopyOffload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVMDiskStorageMap_GetCopyMethod(t *testing.T) {
	tests := []struct {
		name     string
		vmMap    *VMDiskStorageMap
		diskFile string
		want     DiskCopyMethod
	}{
		{
			name: "get copy-offload method",
			vmMap: &VMDiskStorageMap{
				Disks: map[string]*DiskStorageInfo{
					"disk1": {
						DiskFile:   "disk1",
						CopyMethod: CopyMethodCopyOffload,
					},
				},
			},
			diskFile: "disk1",
			want:     CopyMethodCopyOffload,
		},
		{
			name: "get CDI method",
			vmMap: &VMDiskStorageMap{
				Disks: map[string]*DiskStorageInfo{
					"disk1": {
						DiskFile:   "disk1",
						CopyMethod: CopyMethodCDI,
					},
				},
			},
			diskFile: "disk1",
			want:     CopyMethodCDI,
		},
		{
			name: "get virt-v2v method",
			vmMap: &VMDiskStorageMap{
				Disks: map[string]*DiskStorageInfo{
					"disk1": {
						DiskFile:   "disk1",
						CopyMethod: CopyMethodVirtV2v,
					},
				},
			},
			diskFile: "disk1",
			want:     CopyMethodVirtV2v,
		},
		{
			name: "disk not found returns empty string",
			vmMap: &VMDiskStorageMap{
				Disks: map[string]*DiskStorageInfo{},
			},
			diskFile: "nonexistent",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.vmMap.GetCopyMethod(tt.diskFile)
			if got != tt.want {
				t.Errorf("VMDiskStorageMap.GetCopyMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanDiskStorageMaps_GetVMMap(t *testing.T) {
	vm1Map := &VMDiskStorageMap{
		VMID:   "vm-1",
		VMName: "vm1",
		Disks:  map[string]*DiskStorageInfo{},
	}
	vm2Map := &VMDiskStorageMap{
		VMID:   "vm-2",
		VMName: "vm2",
		Disks:  map[string]*DiskStorageInfo{},
	}

	planMaps := &PlanDiskStorageMaps{
		VMs: map[string]*VMDiskStorageMap{
			"vm-1": vm1Map,
			"vm-2": vm2Map,
		},
	}

	tests := []struct {
		name      string
		vmID      string
		wantMap   *VMDiskStorageMap
		wantFound bool
	}{
		{
			name:      "get existing VM map",
			vmID:      "vm-1",
			wantMap:   vm1Map,
			wantFound: true,
		},
		{
			name:      "get another existing VM map",
			vmID:      "vm-2",
			wantMap:   vm2Map,
			wantFound: true,
		},
		{
			name:      "VM not found",
			vmID:      "vm-3",
			wantMap:   nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMap, gotFound := planMaps.GetVMMap(tt.vmID)
			if gotFound != tt.wantFound {
				t.Errorf("PlanDiskStorageMaps.GetVMMap() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotMap != tt.wantMap {
				t.Errorf("PlanDiskStorageMaps.GetVMMap() map = %v, want %v", gotMap, tt.wantMap)
			}
		})
	}
}

func TestDiskStorageInfo_Structure(t *testing.T) {
	// Test that DiskStorageInfo can be properly instantiated
	info := &DiskStorageInfo{
		DiskFile:           "[datastore1] vm/disk.vmdk",
		DiskKey:            2000,
		DiskIndex:          0,
		SourceDatastore:    base.Ref{ID: "ds-1"},
		TargetStorageClass: "test-sc",
		CopyMethod:         CopyMethodCopyOffload,
	}

	if info.DiskFile != "[datastore1] vm/disk.vmdk" {
		t.Errorf("DiskStorageInfo.DiskFile = %v, want [datastore1] vm/disk.vmdk", info.DiskFile)
	}
	if info.DiskKey != 2000 {
		t.Errorf("DiskStorageInfo.DiskKey = %v, want 2000", info.DiskKey)
	}
	if info.DiskIndex != 0 {
		t.Errorf("DiskStorageInfo.DiskIndex = %v, want 0", info.DiskIndex)
	}
	if info.SourceDatastore.ID != "ds-1" {
		t.Errorf("DiskStorageInfo.SourceDatastore.ID = %v, want ds-1", info.SourceDatastore.ID)
	}
	if info.TargetStorageClass != "test-sc" {
		t.Errorf("DiskStorageInfo.TargetStorageClass = %v, want test-sc", info.TargetStorageClass)
	}
	if info.CopyMethod != CopyMethodCopyOffload {
		t.Errorf("DiskStorageInfo.CopyMethod = %v, want %v", info.CopyMethod, CopyMethodCopyOffload)
	}
}

func TestVMDiskStorageMap_DisksByIndex(t *testing.T) {
	// Test that DisksByIndex maintains order
	disk0 := &DiskStorageInfo{
		DiskFile:  "disk0",
		DiskIndex: 0,
	}
	disk1 := &DiskStorageInfo{
		DiskFile:  "disk1",
		DiskIndex: 1,
	}
	disk2 := &DiskStorageInfo{
		DiskFile:  "disk2",
		DiskIndex: 2,
	}

	vmMap := &VMDiskStorageMap{
		VMID:   "vm-1",
		VMName: "test-vm",
		Disks: map[string]*DiskStorageInfo{
			"disk0": disk0,
			"disk1": disk1,
			"disk2": disk2,
		},
		DisksByIndex: []*DiskStorageInfo{disk0, disk1, disk2},
	}

	if len(vmMap.DisksByIndex) != 3 {
		t.Errorf("VMDiskStorageMap.DisksByIndex length = %v, want 3", len(vmMap.DisksByIndex))
	}

	for i, disk := range vmMap.DisksByIndex {
		if disk.DiskIndex != i {
			t.Errorf("DisksByIndex[%d].DiskIndex = %v, want %v", i, disk.DiskIndex, i)
		}
	}
}
