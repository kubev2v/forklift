package vsphere

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

func TestGetDiskBackingSnapshotState(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		vm, err := find.NewFinder(c).VirtualMachine(ctx, "/DC0/vm/DC0_H0_VM0")
		if err != nil {
			t.Fatal(err)
		}
		devices, err := vm.Device(ctx)
		if err != nil {
			t.Fatal(err)
		}
		disks := devices.SelectByType((*types.VirtualDisk)(nil))
		if len(disks) == 0 {
			t.Fatal("simulated VM has no disk")
		}
		diskFile := disks[0].(*types.VirtualDisk).Backing.(*types.VirtualDiskFlatVer2BackingInfo).FileName
		r := &Client{client: &govmomi.Client{Client: c}}

		backing, err := r.getDiskBacking(ctx, vm.Reference().Value, diskFile)
		if err != nil || backing.VMHasSnapshots {
			t.Fatalf("expected no snapshots, got %+v, %v", backing, err)
		}

		task, err := vm.CreateSnapshot(ctx, "snap", "", false, false)
		if err != nil || task.Wait(ctx) != nil {
			t.Fatalf("create snapshot: %v", err)
		}
		backing, err = r.getDiskBacking(ctx, vm.Reference().Value, diskFile)
		if err != nil || !backing.VMHasSnapshots {
			t.Fatalf("expected snapshots, got %+v, %v", backing, err)
		}
	})
}
