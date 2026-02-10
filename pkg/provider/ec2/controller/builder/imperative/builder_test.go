package imperative

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ec2base "github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
)

func newTestBuilder() *Builder {
	b := &ec2base.Base{
		Context: &plancontext.Context{
			Plan: &api.Plan{},
		},
		Log: logging.WithName("test"),
	}
	return New(b)
}

func newVMSpec() *cnv.VirtualMachineSpec {
	return &cnv.VirtualMachineSpec{
		Template: &cnv.VirtualMachineInstanceTemplateSpec{},
	}
}

func strPtr(s string) *string { return &s }

// --- mapCPU ---

func TestMapCPU_StandardInstance(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.InstanceType = ec2types.InstanceType("m5.large")

	object := newVMSpec()
	b.mapCPU(inst, object)

	cpu := object.Template.Spec.Domain.CPU
	if cpu == nil {
		t.Fatal("expected CPU to be set")
	}
	if cpu.Sockets != 1 {
		t.Errorf("expected 1 socket, got %d", cpu.Sockets)
	}
	if cpu.Cores != 2 {
		t.Errorf("expected 2 cores for m5.large, got %d", cpu.Cores)
	}
	if len(cpu.Features) != 0 {
		t.Errorf("expected no CPU features, got %d", len(cpu.Features))
	}
}

func TestMapCPU_XlargeInstance(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.InstanceType = ec2types.InstanceType("c5.xlarge")

	object := newVMSpec()
	b.mapCPU(inst, object)

	if object.Template.Spec.Domain.CPU.Cores != 4 {
		t.Errorf("expected 4 cores for xlarge, got %d", object.Template.Spec.Domain.CPU.Cores)
	}
}

func TestMapCPU_MetalInstance(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.InstanceType = ec2types.InstanceType("m5.metal")

	object := newVMSpec()
	b.mapCPU(inst, object)

	cpu := object.Template.Spec.Domain.CPU
	if len(cpu.Features) != 2 {
		t.Fatalf("expected 2 CPU features for metal, got %d", len(cpu.Features))
	}
	if cpu.Features[0].Name != "vmx" || cpu.Features[0].Policy != "optional" {
		t.Errorf("expected vmx/optional, got %s/%s", cpu.Features[0].Name, cpu.Features[0].Policy)
	}
	if cpu.Features[1].Name != "svm" || cpu.Features[1].Policy != "optional" {
		t.Errorf("expected svm/optional, got %s/%s", cpu.Features[1].Name, cpu.Features[1].Policy)
	}
}

func TestMapCPU_DefaultInstance(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}

	object := newVMSpec()
	b.mapCPU(inst, object)

	if object.Template.Spec.Domain.CPU.Cores != 2 {
		t.Errorf("expected default 2 cores, got %d", object.Template.Spec.Domain.CPU.Cores)
	}
}

// --- mapMemory ---

func TestMapMemory_Large(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.InstanceType = ec2types.InstanceType("m5.large")

	object := newVMSpec()
	b.mapMemory(inst, object)

	mem := object.Template.Spec.Domain.Memory
	if mem == nil || mem.Guest == nil {
		t.Fatal("expected Memory.Guest to be set")
	}
	expectedBytes := int64(8192) * 1024 * 1024
	if mem.Guest.Value() != expectedBytes {
		t.Errorf("expected %d bytes, got %d", expectedBytes, mem.Guest.Value())
	}
}

// --- mapFirmware ---

func TestMapFirmware_BIOS(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.InstanceId = strPtr("i-0abc123")

	object := newVMSpec()
	b.mapFirmware(inst, object)

	fw := object.Template.Spec.Domain.Firmware
	if fw == nil {
		t.Fatal("expected firmware to be set")
	}
	if fw.Serial != "i-0abc123" {
		t.Errorf("expected serial i-0abc123, got %s", fw.Serial)
	}
	if fw.Bootloader == nil || fw.Bootloader.BIOS == nil {
		t.Error("expected BIOS bootloader")
	}
	if fw.Bootloader.EFI != nil {
		t.Error("expected no EFI bootloader for BIOS instance")
	}
}

func TestMapFirmware_UEFI(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.InstanceId = strPtr("i-uefi001")
	inst.BootMode = ec2types.BootModeValuesUefi

	object := newVMSpec()
	b.mapFirmware(inst, object)

	fw := object.Template.Spec.Domain.Firmware
	if fw.Bootloader.EFI == nil {
		t.Fatal("expected EFI bootloader")
	}
	if *fw.Bootloader.EFI.SecureBoot != false {
		t.Error("expected SecureBoot=false for EC2")
	}
	if fw.Bootloader.BIOS != nil {
		t.Error("expected no BIOS bootloader for UEFI instance")
	}
}

// --- mapFeatures ---

func TestMapFeatures_BIOS(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}

	object := newVMSpec()
	b.mapFeatures(inst, object)

	if object.Template.Spec.Domain.Features != nil {
		t.Error("expected no features for BIOS instance")
	}
}

func TestMapFeatures_UEFI(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{}
	inst.BootMode = ec2types.BootModeValuesUefi

	object := newVMSpec()
	b.mapFeatures(inst, object)

	feat := object.Template.Spec.Domain.Features
	if feat == nil || feat.SMM == nil {
		t.Fatal("expected SMM feature for UEFI")
	}
	if feat.SMM.Enabled == nil || !*feat.SMM.Enabled {
		t.Error("expected SMM.Enabled=true")
	}
}

// --- mapInput ---

func TestMapInput_Virtio(t *testing.T) {
	b := newTestBuilder()
	object := newVMSpec()
	b.mapInput(object)

	inputs := object.Template.Spec.Domain.Devices.Inputs
	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}
	if inputs[0].Bus != cnv.InputBusVirtio {
		t.Errorf("expected virtio bus, got %s", inputs[0].Bus)
	}
	if inputs[0].Name != ec2base.Tablet {
		t.Errorf("expected tablet, got %s", inputs[0].Name)
	}
}

// --- mapDisks ---

func TestMapDisks_TwoDisks(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{
		BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
			{Ebs: &model.EbsInstanceBlockDevice{VolumeId: strPtr("vol-aaa")}},
			{Ebs: &model.EbsInstanceBlockDevice{VolumeId: strPtr("vol-bbb")}},
		},
	}
	pvcs := []*core.PersistentVolumeClaim{
		{
			ObjectMeta: meta.ObjectMeta{
				Name:   "boot-pvc",
				Labels: map[string]string{"forklift.konveyor.io/volume-id": "vol-aaa"},
			},
		},
		{
			ObjectMeta: meta.ObjectMeta{
				Name:   "data-pvc",
				Labels: map[string]string{"forklift.konveyor.io/volume-id": "vol-bbb"},
			},
		},
	}

	object := newVMSpec()
	b.mapDisks(inst, pvcs, object)

	disks := object.Template.Spec.Domain.Devices.Disks
	vols := object.Template.Spec.Volumes

	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if len(vols) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(vols))
	}

	if disks[0].Name != "disk-0" {
		t.Errorf("expected disk-0, got %s", disks[0].Name)
	}
	if disks[0].Disk == nil || disks[0].Disk.Bus != cnv.DiskBusVirtio {
		t.Error("expected virtio bus on disk-0")
	}
	if disks[0].BootOrder == nil || *disks[0].BootOrder != 1 {
		t.Error("expected boot order 1 on disk-0")
	}
	if vols[0].PersistentVolumeClaim.ClaimName != "boot-pvc" {
		t.Errorf("expected boot-pvc, got %s", vols[0].PersistentVolumeClaim.ClaimName)
	}

	if disks[1].BootOrder != nil {
		t.Error("expected no boot order on disk-1")
	}
	if vols[1].PersistentVolumeClaim.ClaimName != "data-pvc" {
		t.Errorf("expected data-pvc, got %s", vols[1].PersistentVolumeClaim.ClaimName)
	}
}

func TestMapDisks_SkipsMissingPVC(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{
		BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
			{Ebs: &model.EbsInstanceBlockDevice{VolumeId: strPtr("vol-aaa")}},
			{Ebs: &model.EbsInstanceBlockDevice{VolumeId: strPtr("vol-missing")}},
		},
	}
	pvcs := []*core.PersistentVolumeClaim{
		{
			ObjectMeta: meta.ObjectMeta{
				Name:   "only-pvc",
				Labels: map[string]string{"forklift.konveyor.io/volume-id": "vol-aaa"},
			},
		},
	}

	object := newVMSpec()
	b.mapDisks(inst, pvcs, object)

	if len(object.Template.Spec.Domain.Devices.Disks) != 1 {
		t.Errorf("expected 1 disk (missing PVC skipped), got %d", len(object.Template.Spec.Domain.Devices.Disks))
	}
}

func TestMapDisks_SkipsNilEbs(t *testing.T) {
	b := newTestBuilder()
	inst := &model.InstanceDetails{
		BlockDeviceMappings: []model.InstanceBlockDeviceMapping{
			{Ebs: nil},
			{Ebs: &model.EbsInstanceBlockDevice{VolumeId: strPtr("vol-aaa")}},
		},
	}
	pvcs := []*core.PersistentVolumeClaim{
		{
			ObjectMeta: meta.ObjectMeta{
				Name:   "pvc-a",
				Labels: map[string]string{"forklift.konveyor.io/volume-id": "vol-aaa"},
			},
		},
	}

	object := newVMSpec()
	b.mapDisks(inst, pvcs, object)

	if len(object.Template.Spec.Domain.Devices.Disks) != 1 {
		t.Errorf("expected 1 disk (nil Ebs skipped), got %d", len(object.Template.Spec.Domain.Devices.Disks))
	}
}

// --- isUEFI ---

func TestIsUEFI(t *testing.T) {
	tb := newTestBuilder()

	tests := []struct {
		name     string
		bootMode ec2types.BootModeValues
		want     bool
	}{
		{"empty", "", false},
		{"legacy-bios", ec2types.BootModeValuesLegacyBios, false},
		{"uefi", ec2types.BootModeValuesUefi, true},
		{"uefi-preferred", ec2types.BootModeValuesUefiPreferred, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &model.InstanceDetails{}
			inst.BootMode = tt.bootMode
			if got := tb.base.IsUEFI(inst); got != tt.want {
				t.Errorf("IsUEFI(%s) = %v, want %v", tt.bootMode, got, tt.want)
			}
		})
	}
}
