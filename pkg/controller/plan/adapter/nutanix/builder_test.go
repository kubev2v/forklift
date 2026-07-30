package nutanix

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	liblogging "github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	libvirtxml "libvirt.org/go/libvirtxml"
)

// newOrderingTestVM builds a VM whose Disks are deliberately not in
// SCSI→SATA→IDE→PCI/DeviceIndex order, to verify that ordering is derived
// rather than assumed from inventory order.
func newOrderingTestVM() *model.VM {
	vm := &model.VM{}
	vm.Name = "test-vm"
	vm.Disks = []model.Disk{
		{UUID: "disk-cdrom", IsCdrom: true, DeviceIndex: 4},
		{UUID: "disk-ide", AdapterType: adapterIDE, DeviceIndex: 0},
		{UUID: "disk-scsi-1", AdapterType: adapterSCSI, DeviceIndex: 3},
		{UUID: "disk-sata", AdapterType: adapterSATA, DeviceIndex: 1},
		{UUID: "disk-scsi-0", AdapterType: adapterSCSI, DeviceIndex: 2},
	}
	return vm
}

// pvcFor builds a minimal PVC annotated as the destination for diskUUID.
func pvcFor(name, diskUUID string, blockMode bool) *core.PersistentVolumeClaim {
	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{planbase.AnnDiskSource: diskUUID},
		},
	}
	if blockMode {
		mode := core.PersistentVolumeBlock
		pvc.Spec.VolumeMode = &mode
	}
	return pvc
}

func TestMapDisksOrdersBySCSIThenSATAThenIDE(t *testing.T) {
	vm := newOrderingTestVM()

	// Deliberately scrambled relative to both inventory order and the
	// expected SCSI/SATA/IDE order, to prove lookups are by disk UUID.
	pvcs := []*core.PersistentVolumeClaim{
		pvcFor("pvc-sata", "disk-sata", false),
		pvcFor("pvc-scsi-0", "disk-scsi-0", false),
		pvcFor("pvc-ide", "disk-ide", false),
		pvcFor("pvc-scsi-1", "disk-scsi-1", true),
	}

	builder := &Builder{Context: &plancontext.Context{Log: liblogging.WithName("test")}}
	object := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
	builder.mapDisks(vm, pvcs, object)

	wantOrder := []string{"disk-scsi-0", "disk-scsi-1", "disk-sata", "disk-ide"}
	volumes := object.Template.Spec.Volumes
	if len(volumes) != len(wantOrder) {
		t.Fatalf("expected %d volumes, got %d: %+v", len(wantOrder), len(volumes), volumes)
	}
	for i, name := range wantOrder {
		if volumes[i].Name != name {
			t.Errorf("volume[%d]: expected %q, got %q", i, name, volumes[i].Name)
		}
		if volumes[i].PersistentVolumeClaim == nil {
			t.Fatalf("volume[%d] %q: expected a PVC volume source", i, name)
		}
	}

	// The disk with the globally-lowest DeviceIndex (disk-ide, index 0) is
	// the boot disk, regardless of where it lands in the bus-priority order.
	disks := object.Template.Spec.Domain.Devices.Disks
	if len(disks) != len(wantOrder) {
		t.Fatalf("expected %d disks, got %d", len(wantOrder), len(disks))
	}
	for i, disk := range disks {
		isBootDisk := disk.Name == "disk-ide"
		if isBootDisk && (disk.BootOrder == nil || *disk.BootOrder != 1) {
			t.Errorf("disk[%d] %q: expected BootOrder 1", i, disk.Name)
		}
		if !isBootDisk && disk.BootOrder != nil {
			t.Errorf("disk[%d] %q: expected no BootOrder, got %v", i, disk.Name, *disk.BootOrder)
		}
	}
}

// TestBuildDomainXMLMatchesMapDisksOrder verifies the domain XML disk
// ordering -- and thus the /mnt/disks/disk{i} indices it embeds -- matches
// the order Builder.mapDisks() gives the same VM and PVCs, which is what
// podVolumeMounts() uses to decide where each PVC is actually mounted in the
// conversion pod.
func TestBuildDomainXMLMatchesMapDisksOrder(t *testing.T) {
	vm := newOrderingTestVM()
	pvcs := []*core.PersistentVolumeClaim{
		pvcFor("pvc-sata", "disk-sata", false),
		pvcFor("pvc-scsi-0", "disk-scsi-0", false),
		pvcFor("pvc-ide", "disk-ide", false),
		pvcFor("pvc-scsi-1", "disk-scsi-1", true),
	}

	builder := &Builder{Context: &plancontext.Context{Log: liblogging.WithName("test")}}
	object := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
	builder.mapDisks(vm, pvcs, object)

	var wantUUIDs []string
	for _, v := range object.Template.Spec.Volumes {
		wantUUIDs = append(wantUUIDs, v.Name)
	}

	xmlStr, err := buildDomainXML(vm, pvcs)
	if err != nil {
		t.Fatalf("buildDomainXML: %v", err)
	}

	domain := &libvirtxml.Domain{}
	if err := xml.Unmarshal([]byte(xmlStr), domain); err != nil {
		t.Fatalf("failed to parse generated domain XML: %v", err)
	}

	if len(domain.Devices.Disks) != len(wantUUIDs) {
		t.Fatalf("expected %d disks in domain XML, got %d", len(wantUUIDs), len(domain.Devices.Disks))
	}
	for i, uuid := range wantUUIDs {
		d := domain.Devices.Disks[i]
		switch uuid {
		case "disk-scsi-1":
			// The only PVC with block volume mode; must land at the same
			// index (1) as its /mnt/disks/disk1 pod mount would.
			if d.Source == nil || d.Source.Block == nil {
				t.Errorf("disk[%d] (%s): expected a block source, got %+v", i, uuid, d.Source)
			} else if d.Source.Block.Dev != fmt.Sprintf("/dev/block%d", i) {
				t.Errorf("disk[%d] (%s): expected block dev /dev/block%d, got %s", i, uuid, i, d.Source.Block.Dev)
			}
		default:
			wantFile := fmt.Sprintf("/mnt/disks/disk%d/disk.img", i)
			if d.Source == nil || d.Source.File == nil || d.Source.File.File != wantFile {
				t.Errorf("disk[%d] (%s): expected file source %q, got %+v", i, uuid, wantFile, d.Source)
			}
		}
	}
}

// TestBuildDomainXMLOmitsDisksWithoutPVC verifies that a disk with no
// matching PVC (e.g. unmapped storage) is dropped entirely rather than
// merely defaulting its volume mode -- otherwise every disk index after it
// would drift out of sync with the pod's mounted volumes.
func TestBuildDomainXMLOmitsDisksWithoutPVC(t *testing.T) {
	vm := newOrderingTestVM()
	// Omit disk-sata entirely -- simulates an unmapped storage container.
	pvcs := []*core.PersistentVolumeClaim{
		pvcFor("pvc-scsi-0", "disk-scsi-0", false),
		pvcFor("pvc-scsi-1", "disk-scsi-1", false),
		pvcFor("pvc-ide", "disk-ide", false),
	}

	xmlStr, err := buildDomainXML(vm, pvcs)
	if err != nil {
		t.Fatalf("buildDomainXML: %v", err)
	}
	domain := &libvirtxml.Domain{}
	if err := xml.Unmarshal([]byte(xmlStr), domain); err != nil {
		t.Fatalf("failed to parse generated domain XML: %v", err)
	}

	// disk-sata is omitted, so the remaining order is scsi-0, scsi-1, ide --
	// distinguishable here by bus, since only the trailing disk is IDE.
	wantBuses := []string{"scsi", "scsi", "ide"}
	if len(domain.Devices.Disks) != len(wantBuses) {
		t.Fatalf("expected %d disks, got %d", len(wantBuses), len(domain.Devices.Disks))
	}
	for i, bus := range wantBuses {
		target := domain.Devices.Disks[i].Target
		if target == nil || target.Bus != bus {
			t.Errorf("disk[%d]: expected bus %q, got %+v", i, bus, target)
		}
		wantFile := fmt.Sprintf("/mnt/disks/disk%d/disk.img", i)
		source := domain.Devices.Disks[i].Source
		if source == nil || source.File == nil || source.File.File != wantFile {
			t.Errorf("disk[%d]: expected file source %q, got %+v", i, wantFile, source)
		}
	}
}

func TestConfigMapSetsCDICertKeys(t *testing.T) {
	cacert := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	secret := &core.Secret{
		Data: map[string][]byte{
			"ca.crt": cacert,
		},
	}
	configMap := &core.ConfigMap{}
	builder := &Builder{}

	err := builder.ConfigMap(ref.Ref{}, secret, configMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(configMap.BinaryData["ca.pem"], cacert) {
		t.Fatalf("expected ca.pem to match provider CA")
	}
	if !bytes.Equal(configMap.BinaryData["tls.crt"], cacert) {
		t.Fatalf("expected tls.crt to match provider CA for CDI nbdkit cainfo")
	}
}

func TestBootDiskUUID(t *testing.T) {
	tests := []struct {
		name            string
		bootDeviceOrder string
		disks           []model.Disk
		want            string
	}{
		{
			name:            "first block disk in disk_list when indices collide",
			bootDeviceOrder: "DISK,CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
			},
			want: "sata-0",
		},
		{
			name:            "skips cdrom before first disk in inventory order",
			bootDeviceOrder: "CDROM,DISK,NETWORK",
			disks: []model.Disk{
				{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
			},
			want: "sata-0",
		},
		{
			name: "defaults to disk cdrom network when boot order empty",
			disks: []model.Disk{
				{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
				{UUID: "disk-1", DeviceType: "DISK"},
			},
			want: "disk-1",
		},
		{
			name:            "falls back to first non-cdrom when boot order has no disk",
			bootDeviceOrder: "CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
				{UUID: "disk-1", DeviceType: "DISK"},
			},
			want: "disk-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &model.VM{VM1: model.VM1{
				BootDeviceOrder: tt.bootDeviceOrder,
				Disks:           tt.disks,
			}}
			if got := bootDiskUUID(vm); got != tt.want {
				t.Fatalf("bootDiskUUID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapDisks_BootOrder(t *testing.T) {
	tests := []struct {
		name            string
		bootDeviceOrder string
		disks           []model.Disk
		wantBootDisk    string
	}{
		{
			name:            "exactly one boot disk when adapter indices collide",
			bootDeviceOrder: "DISK,CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
			},
			wantBootDisk: "sata-0",
		},
		{
			name:            "boot order follows inventory not adapter priority",
			bootDeviceOrder: "DISK,CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
			},
			wantBootDisk: "scsi-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &model.VM{VM1: model.VM1{
				BootDeviceOrder: tt.bootDeviceOrder,
				Disks:           tt.disks,
			}}
			pvcs := make([]*core.PersistentVolumeClaim, 0, len(tt.disks))
			for _, disk := range tt.disks {
				if disk.IsCdrom {
					continue
				}
				pvcs = append(pvcs, &core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Annotations: map[string]string{planbase.AnnDiskSource: disk.UUID},
					},
				})
			}

			object := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
			(&Builder{}).mapDisks(vm, pvcs, object)

			var bootCount int
			for _, disk := range object.Template.Spec.Domain.Devices.Disks {
				if disk.BootOrder == nil {
					continue
				}
				bootCount++
				if disk.Name != tt.wantBootDisk {
					t.Fatalf("expected boot order on %q, got %q", tt.wantBootDisk, disk.Name)
				}
			}
			if bootCount != 1 {
				t.Fatalf("expected exactly one boot disk, got %d", bootCount)
			}
		})
	}
}
