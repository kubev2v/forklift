package copyappliance

import (
	"testing"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func TestAttachedDiskPathSet(t *testing.T) {
	appliance := testAppliance()
	paths := attachedDiskPathSet(appliance.Spec)
	if len(paths) != 2 {
		t.Fatalf("expected 2 attached disk paths, got %d", len(paths))
	}
	if !paths["[datastore13] vm-a/disk-0.vmdk"] {
		t.Fatal("expected vm-a disk path")
	}
}

// scsiController is an empty controller for a disk to be assigned to.
func scsiController() *types.ParaVirtualSCSIController {
	return &types.ParaVirtualSCSIController{
		VirtualSCSIController: types.VirtualSCSIController{
			VirtualController: types.VirtualController{
				VirtualDevice: types.VirtualDevice{Key: 1000},
				BusNumber:     0,
			},
			ScsiCtlrUnitNumber: 7,
		},
	}
}

func TestBuildAttachDiskChanges(t *testing.T) {
	r := &ApplianceContext{Appliance: testAppliance()}
	devices := object.VirtualDeviceList{scsiController()}

	changes, err := r.buildAttachDiskChanges(devices)
	if err != nil {
		t.Fatalf("buildAttachDiskChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("changes = %d, want one per attached disk", len(changes))
	}

	units := map[int32]bool{}
	for i, change := range changes {
		spec := change.GetVirtualDeviceConfigSpec()
		if spec.Operation != types.VirtualDeviceConfigSpecOperationAdd {
			t.Errorf("changes[%d].Operation = %q, want add", i, spec.Operation)
		}
		// An empty FileOperation is what attaches an existing vmdk rather than
		// creating one.
		if spec.FileOperation != "" {
			t.Errorf("changes[%d].FileOperation = %q, want empty", i, spec.FileOperation)
		}
		disk, ok := spec.Device.(*types.VirtualDisk)
		if !ok {
			t.Fatalf("changes[%d].Device = %T, want a disk", i, spec.Device)
		}
		backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			t.Fatalf("changes[%d] backing = %T, want flat", i, disk.Backing)
		}
		want := r.Appliance.Spec.AttachDisks[i].VMDKPath
		if backing.FileName != want {
			t.Errorf("changes[%d] FileName = %q, want %q", i, backing.FileName, want)
		}
		// Spec.Datastore is where the appliance itself lives, not where the
		// disks it serves do. Naming it here would contradict the path above.
		if backing.Datastore != nil {
			t.Errorf("changes[%d] backing names datastore %v, want none", i, backing.Datastore)
		}
		if backing.DiskMode != string(types.VirtualDiskModeIndependent_nonpersistent) {
			t.Errorf("changes[%d] DiskMode = %q, want independent_nonpersistent", i, backing.DiskMode)
		}
		if disk.UnitNumber == nil {
			t.Fatalf("changes[%d] has no unit number", i)
		}
		if units[*disk.UnitNumber] {
			t.Errorf("changes[%d] reuses unit number %d", i, *disk.UnitNumber)
		}
		units[*disk.UnitNumber] = true
	}
}

// Attaching is retried until the appliance reports every export, so it has to
// be safe to run against a VM that already carries some of the disks.
func TestBuildAttachDiskChangesSkipsDisksAlreadyOnTheVM(t *testing.T) {
	r := &ApplianceContext{Appliance: testAppliance()}
	attached := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key: 2000,
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: "[datastore13] vm-a/disk-0.vmdk",
				},
			},
		},
	}
	devices := object.VirtualDeviceList{scsiController(), attached}

	changes, err := r.buildAttachDiskChanges(devices)
	if err != nil {
		t.Fatalf("buildAttachDiskChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want only the disk that is not attached yet", len(changes))
	}
	disk := changes[0].GetVirtualDeviceConfigSpec().Device.(*types.VirtualDisk)
	backing := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
	if backing.FileName != "[datastore13] vm-b/disk-0.vmdk" {
		t.Errorf("FileName = %q, want vm-b's disk", backing.FileName)
	}
}
