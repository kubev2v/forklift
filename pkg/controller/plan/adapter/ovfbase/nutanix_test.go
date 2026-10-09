package ovfbase

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	ovfmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
)

func emptyVMSpec() *cnv.VirtualMachineSpec {
	return &cnv.VirtualMachineSpec{
		Template: &cnv.VirtualMachineInstanceTemplateSpec{},
	}
}

func TestMapMachine(t *testing.T) {
	tests := []struct {
		name        string
		machineType string
		wantType    string
		wantSet     bool
	}{
		{name: "empty leaves unset", machineType: "", wantSet: false},
		{name: "pc leaves unset for KubeVirt default", machineType: "pc", wantSet: false},
		{name: "q35 passed through", machineType: "q35", wantType: "q35", wantSet: true},
		{name: "pc-q35 passed through", machineType: "pc-q35-rhel9.4.0", wantType: "pc-q35-rhel9.4.0", wantSet: true},
		{name: "Q35 case preserved from inventory", machineType: "Q35", wantType: "Q35", wantSet: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := emptyVMSpec()
			(&Builder{}).mapMachine(&model.VM{MachineType: tt.machineType}, spec)
			if !tt.wantSet {
				if spec.Template.Spec.Domain.Machine != nil {
					t.Fatalf("Machine set to %+v, want unset", spec.Template.Spec.Domain.Machine)
				}
				return
			}
			if spec.Template.Spec.Domain.Machine == nil || spec.Template.Spec.Domain.Machine.Type != tt.wantType {
				t.Fatalf("Machine=%+v, want type %q", spec.Template.Spec.Domain.Machine, tt.wantType)
			}
		})
	}
}

func TestMapClock(t *testing.T) {
	t.Run("empty leaves unset", func(t *testing.T) {
		spec := emptyVMSpec()
		(&Builder{}).mapClock(&model.VM{}, spec)
		if spec.Template.Spec.Domain.Clock != nil {
			t.Fatalf("Clock set to %+v, want unset", spec.Template.Spec.Domain.Clock)
		}
	})
	t.Run("UTC", func(t *testing.T) {
		spec := emptyVMSpec()
		(&Builder{}).mapClock(&model.VM{HardwareClockTimezone: "UTC"}, spec)
		want := &cnv.Clock{ClockOffset: cnv.ClockOffset{UTC: &cnv.ClockOffsetUTC{}}}
		if diff := cmp.Diff(want, spec.Template.Spec.Domain.Clock); diff != "" {
			t.Fatalf("Clock mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("named timezone", func(t *testing.T) {
		spec := emptyVMSpec()
		(&Builder{}).mapClock(&model.VM{HardwareClockTimezone: "America/Los_Angeles"}, spec)
		tz := cnv.ClockOffsetTimezone("America/Los_Angeles")
		want := &cnv.Clock{ClockOffset: cnv.ClockOffset{Timezone: &tz}}
		if diff := cmp.Diff(want, spec.Template.Spec.Domain.Clock); diff != "" {
			t.Fatalf("Clock mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestOvaBootDiskPath(t *testing.T) {
	disks := []ovfmodel.Disk{
		{Base: ovfmodel.Base{Name: "disk0"}, FilePath: "/ova/disk0.vmdk"},
		{Base: ovfmodel.Base{Name: "disk1"}, FilePath: "/ova/disk1.vmdk"},
	}
	tests := []struct {
		name  string
		order string
		want  string
	}{
		{name: "empty order", order: "", want: ""},
		{name: "disk first", order: "DISK,CDROM,NETWORK", want: "/ova/disk0.vmdk::disk0"},
		{name: "cdrom before disk", order: "CDROM,DISK,NETWORK", want: "/ova/disk0.vmdk::disk0"},
		{name: "no disk entry", order: "CDROM,NETWORK", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ovaBootDiskPath(tt.order, disks); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapDisks_BootOrder(t *testing.T) {
	disks := []ovfmodel.Disk{
		{Base: ovfmodel.Base{Name: "disk0"}, FilePath: "/ova/disk0.vmdk", DiskId: "d0"},
		{Base: ovfmodel.Base{Name: "disk1"}, FilePath: "/ova/disk1.vmdk", DiskId: "d1"},
	}
	pvcs := make([]*core.PersistentVolumeClaim, 0, len(disks))
	for _, disk := range disks {
		d := disk
		pvcs = append(pvcs, &core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc-" + d.Name,
				Annotations: map[string]string{
					planbase.AnnDiskSource: getDiskFullPath(&d),
				},
			},
		})
	}

	t.Run("nutanix order sets first disk", func(t *testing.T) {
		spec := emptyVMSpec()
		(&Builder{}).mapDisks(&model.VM{
			BootDeviceOrder: "DISK,CDROM,NETWORK",
			Disks:           disks,
		}, pvcs, spec)
		got := spec.Template.Spec.Domain.Devices.Disks
		if len(got) != 2 {
			t.Fatalf("got %d disks", len(got))
		}
		if got[0].BootOrder == nil || *got[0].BootOrder != 1 {
			t.Fatalf("disk0 BootOrder=%v, want 1", got[0].BootOrder)
		}
		if got[1].BootOrder != nil {
			t.Fatalf("disk1 BootOrder=%v, want unset", got[1].BootOrder)
		}
	})

	t.Run("empty order leaves unset", func(t *testing.T) {
		spec := emptyVMSpec()
		(&Builder{}).mapDisks(&model.VM{Disks: disks}, pvcs, spec)
		for i, disk := range spec.Template.Spec.Domain.Devices.Disks {
			if disk.BootOrder != nil {
				t.Fatalf("disk[%d] BootOrder=%v, want unset", i, disk.BootOrder)
			}
		}
	})
}
