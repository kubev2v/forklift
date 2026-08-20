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

func TestBootDiskUUID_UsesInventoryBootOrder(t *testing.T) {
	vm := &model.VM{VM1: model.VM1{
		BootDeviceOrder: "CDROM,DISK,NETWORK",
		Disks: []model.Disk{
			{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
			{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
			{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
		},
	}}

	got := bootDiskUUID(vm)
	if got != "sata-0" {
		t.Fatalf("expected first block disk in disk_list order, got %q", got)
	}
}

func TestMapDisks_AssignsBootOrderToInventoryBootDisk(t *testing.T) {
	vm := &model.VM{VM1: model.VM1{
		BootDeviceOrder: "DISK,CDROM,NETWORK",
		Disks: []model.Disk{
			{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
			{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
		},
	}}
	pvcs := []*core.PersistentVolumeClaim{
		{ObjectMeta: meta.ObjectMeta{Annotations: map[string]string{planbase.AnnDiskSource: "sata-0"}}},
		{ObjectMeta: meta.ObjectMeta{Annotations: map[string]string{planbase.AnnDiskSource: "scsi-0"}}},
	}

	object := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
	builder := &Builder{}
	builder.mapDisks(vm, pvcs, object)

	var bootCount int
	for _, disk := range object.Template.Spec.Domain.Devices.Disks {
		if disk.BootOrder != nil {
			bootCount++
			if disk.Name != "sata-0" {
				t.Fatalf("expected boot order on sata-0, got %q", disk.Name)
			}
		}
	}
	if bootCount != 1 {
		t.Fatalf("expected exactly one boot disk, got %d", bootCount)
	}
}
