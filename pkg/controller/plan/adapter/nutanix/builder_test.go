package nutanix

import (
	"bytes"
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
)

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
