package builder

import (
	"testing"

	builder "github.com/kubev2v/forklift/pkg/provider/builder"
	cnv "kubevirt.io/api/core/v1"
)

// TestDefaultTemplate_BasicBIOS verifies the default template produces correct output
// for a basic BIOS instance with pod networking (the most common EC2 migration case).
func TestDefaultTemplate_BasicBIOS(t *testing.T) {
	values := &builder.VMBuildValues{
		Name:         "my-ec2-instance",
		ID:           "i-0abc123def456",
		InstanceType: "m5.large",
		RunStrategy:  "Halted",
		Sockets:      1,
		Cores:        2,
		MemoryMiB:    8192,
		IsUEFI:       false,
		HasACPI:      true,
		Serial:       "i-0abc123def456",
		InputBus:     "virtio",
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "my-plan-boot-disk", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
			{Name: "disk-1", PVCName: "my-plan-data-disk", Bus: "virtio"},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
		NodeSelector: map[string]string{
			"topology.kubernetes.io/zone": "us-east-1a",
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	// Run strategy
	if spec.RunStrategy == nil || *spec.RunStrategy != cnv.RunStrategyHalted {
		t.Errorf("expected RunStrategy Halted, got %v", spec.RunStrategy)
	}

	// Memory
	mem := spec.Template.Spec.Domain.Resources.Requests["memory"]
	if mem.String() != "8Gi" {
		t.Errorf("expected memory 8Gi, got %s", mem.String())
	}

	// CPU topology
	if spec.Template.Spec.Domain.CPU == nil {
		t.Fatal("expected CPU to be set")
	}
	if spec.Template.Spec.Domain.CPU.Sockets != 1 {
		t.Errorf("expected 1 socket, got %d", spec.Template.Spec.Domain.CPU.Sockets)
	}
	if spec.Template.Spec.Domain.CPU.Cores != 2 {
		t.Errorf("expected 2 cores, got %d", spec.Template.Spec.Domain.CPU.Cores)
	}
	if len(spec.Template.Spec.Domain.CPU.Features) != 0 {
		t.Errorf("expected no CPU features for non-metal, got %d", len(spec.Template.Spec.Domain.CPU.Features))
	}

	// Firmware - BIOS
	if spec.Template.Spec.Domain.Firmware == nil {
		t.Fatal("expected firmware to be set")
	}
	if spec.Template.Spec.Domain.Firmware.Serial != "i-0abc123def456" {
		t.Errorf("expected serial i-0abc123def456, got %s", spec.Template.Spec.Domain.Firmware.Serial)
	}
	if spec.Template.Spec.Domain.Firmware.Bootloader == nil || spec.Template.Spec.Domain.Firmware.Bootloader.BIOS == nil {
		t.Error("expected BIOS bootloader")
	}
	if spec.Template.Spec.Domain.Firmware.Bootloader.EFI != nil {
		t.Error("expected no EFI bootloader for BIOS instance")
	}

	// Features - ACPI only (no SMM for BIOS)
	if spec.Template.Spec.Domain.Features == nil {
		t.Fatal("expected features to be set")
	}
	if spec.Template.Spec.Domain.Features.SMM != nil {
		t.Error("expected no SMM for BIOS instance")
	}

	// Input devices - tablet with virtio
	if len(spec.Template.Spec.Domain.Devices.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(spec.Template.Spec.Domain.Devices.Inputs))
	}
	if spec.Template.Spec.Domain.Devices.Inputs[0].Bus != "virtio" {
		t.Errorf("expected virtio bus, got %s", spec.Template.Spec.Domain.Devices.Inputs[0].Bus)
	}
	if spec.Template.Spec.Domain.Devices.Inputs[0].Name != "tablet" {
		t.Errorf("expected tablet input, got %s", spec.Template.Spec.Domain.Devices.Inputs[0].Name)
	}

	// Disks
	if len(spec.Template.Spec.Domain.Devices.Disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(spec.Template.Spec.Domain.Devices.Disks))
	}
	// Boot disk
	d0 := spec.Template.Spec.Domain.Devices.Disks[0]
	if d0.Name != "disk-0" {
		t.Errorf("expected disk-0, got %s", d0.Name)
	}
	if d0.Disk == nil || d0.Disk.Bus != "virtio" {
		t.Error("expected virtio bus on disk-0")
	}
	if d0.BootOrder == nil || *d0.BootOrder != 1 {
		t.Error("expected boot order 1 on disk-0")
	}
	// Data disk
	d1 := spec.Template.Spec.Domain.Devices.Disks[1]
	if d1.BootOrder != nil {
		t.Error("expected no boot order on disk-1")
	}

	// Interfaces
	if len(spec.Template.Spec.Domain.Devices.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(spec.Template.Spec.Domain.Devices.Interfaces))
	}
	iface := spec.Template.Spec.Domain.Devices.Interfaces[0]
	if iface.Name != "net-0" {
		t.Errorf("expected net-0, got %s", iface.Name)
	}
	if iface.Model != "virtio" {
		t.Errorf("expected virtio model, got %s", iface.Model)
	}
	if iface.Masquerade == nil {
		t.Error("expected masquerade binding")
	}

	// Networks
	if len(spec.Template.Spec.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(spec.Template.Spec.Networks))
	}
	if spec.Template.Spec.Networks[0].Pod == nil {
		t.Error("expected pod network")
	}

	// Volumes
	if len(spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(spec.Template.Spec.Volumes))
	}
	if spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName != "my-plan-boot-disk" {
		t.Errorf("expected my-plan-boot-disk, got %s", spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName)
	}
	if spec.Template.Spec.Volumes[1].PersistentVolumeClaim.ClaimName != "my-plan-data-disk" {
		t.Errorf("expected my-plan-data-disk, got %s", spec.Template.Spec.Volumes[1].PersistentVolumeClaim.ClaimName)
	}

	// Node selector
	if spec.Template.Spec.NodeSelector == nil {
		t.Fatal("expected node selector")
	}
	if spec.Template.Spec.NodeSelector["topology.kubernetes.io/zone"] != "us-east-1a" {
		t.Errorf("expected zone us-east-1a, got %s", spec.Template.Spec.NodeSelector["topology.kubernetes.io/zone"])
	}
}

// TestDefaultTemplate_UEFI verifies UEFI firmware rendering with SMM.
func TestDefaultTemplate_UEFI(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy: "Halted",
		Sockets:     1,
		Cores:       4,
		MemoryMiB:   16384,
		IsUEFI:      true,
		HasACPI:     true,
		HasSMM:      true,
		Serial:      "i-uefi001",
		InputBus:    "virtio",
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	// UEFI firmware
	if spec.Template.Spec.Domain.Firmware.Bootloader.EFI == nil {
		t.Fatal("expected EFI bootloader")
	}
	if spec.Template.Spec.Domain.Firmware.Bootloader.BIOS != nil {
		t.Error("expected no BIOS for UEFI instance")
	}

	// SMM feature
	if spec.Template.Spec.Domain.Features == nil || spec.Template.Spec.Domain.Features.SMM == nil {
		t.Fatal("expected SMM feature for UEFI")
	}
}

// TestDefaultTemplate_MetalInstance verifies CPU features for bare metal instances.
func TestDefaultTemplate_MetalInstance(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy:       "Halted",
		Sockets:           1,
		Cores:             96,
		MemoryMiB:         393216,
		Serial:            "i-metal001",
		IsUEFI:            false,
		HasACPI:           true,
		InputBus:          "virtio",
		NestedVirtEnabled: true,
		CPUFeatures: []builder.CPUFeatureBuildValues{
			{Name: "vmx", Policy: "optional"},
			{Name: "svm", Policy: "optional"},
		},
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	if len(spec.Template.Spec.Domain.CPU.Features) != 2 {
		t.Fatalf("expected 2 CPU features, got %d", len(spec.Template.Spec.Domain.CPU.Features))
	}
	if spec.Template.Spec.Domain.CPU.Features[0].Name != "vmx" || spec.Template.Spec.Domain.CPU.Features[0].Policy != "optional" {
		t.Errorf("expected vmx/optional, got %s/%s", spec.Template.Spec.Domain.CPU.Features[0].Name, spec.Template.Spec.Domain.CPU.Features[0].Policy)
	}
	if spec.Template.Spec.Domain.CPU.Features[1].Name != "svm" || spec.Template.Spec.Domain.CPU.Features[1].Policy != "optional" {
		t.Errorf("expected svm/optional, got %s/%s", spec.Template.Spec.Domain.CPU.Features[1].Name, spec.Template.Spec.Domain.CPU.Features[1].Policy)
	}
}

// TestDefaultTemplate_CompatibilityMode verifies SATA/E1000e/USB in compat mode.
func TestDefaultTemplate_CompatibilityMode(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy: "Halted",
		Sockets:     1,
		Cores:       2,
		MemoryMiB:   4096,
		Serial:      "i-compat",
		IsUEFI:      false,
		HasACPI:     true,
		InputBus:    "usb", // compat mode
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "sata", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "e1000e", BindingMethod: "masquerade"},
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	// Check USB tablet
	if spec.Template.Spec.Domain.Devices.Inputs[0].Bus != "usb" {
		t.Errorf("expected usb bus, got %s", spec.Template.Spec.Domain.Devices.Inputs[0].Bus)
	}

	// Check SATA disk
	if spec.Template.Spec.Domain.Devices.Disks[0].Disk.Bus != "sata" {
		t.Errorf("expected sata bus, got %s", spec.Template.Spec.Domain.Devices.Disks[0].Disk.Bus)
	}

	// Check E1000e NIC
	if spec.Template.Spec.Domain.Devices.Interfaces[0].Model != "e1000e" {
		t.Errorf("expected e1000e model, got %s", spec.Template.Spec.Domain.Devices.Interfaces[0].Model)
	}
}

// TestDefaultTemplate_UDNNetwork verifies l2bridge binding for UDN pod networks.
func TestDefaultTemplate_UDNNetwork(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy: "Halted",
		Sockets:     1,
		Cores:       2,
		MemoryMiB:   4096,
		Serial:      "i-udn",
		IsUEFI:      false,
		HasACPI:     true,
		InputBus:    "virtio",
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "default", Type: "pod", Model: "virtio", BindingMethod: "l2bridge", HasUDN: true, IsUDNPod: true},
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	iface := spec.Template.Spec.Domain.Devices.Interfaces[0]
	if iface.Binding == nil || iface.Binding.Name != "l2bridge" {
		t.Error("expected l2bridge binding for UDN pod network")
	}
	if iface.Masquerade != nil {
		t.Error("expected no masquerade for UDN pod network")
	}
}

// TestDefaultTemplate_MultusWithMAC verifies multus network with MAC preservation.
func TestDefaultTemplate_MultusWithMAC(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy: "Halted",
		Sockets:     1,
		Cores:       2,
		MemoryMiB:   4096,
		Serial:      "i-multus",
		IsUEFI:      false,
		HasACPI:     true,
		InputBus:    "virtio",
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "multus", MultusName: "default/my-bridge", Model: "virtio", BindingMethod: "bridge", MACAddress: "02:42:ac:11:00:02"},
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	iface := spec.Template.Spec.Domain.Devices.Interfaces[0]
	if iface.MacAddress != "02:42:ac:11:00:02" {
		t.Errorf("expected MAC 02:42:ac:11:00:02, got %s", iface.MacAddress)
	}
	if iface.Bridge == nil {
		t.Error("expected bridge binding for multus")
	}

	net := spec.Template.Spec.Networks[0]
	if net.Multus == nil {
		t.Fatal("expected multus network")
	}
	if net.Multus.NetworkName != "default/my-bridge" {
		t.Errorf("expected default/my-bridge, got %s", net.Multus.NetworkName)
	}
}

// TestDefaultTemplate_RunStrategyAlways verifies that a running source VM produces RunStrategyAlways.
func TestDefaultTemplate_RunStrategyAlways(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy: "Always",
		Sockets:     1,
		Cores:       2,
		MemoryMiB:   4096,
		Serial:      "i-running",
		IsUEFI:      false,
		HasACPI:     true,
		InputBus:    "virtio",
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	if spec.RunStrategy == nil || *spec.RunStrategy != cnv.RunStrategyAlways {
		t.Errorf("expected RunStrategy Always, got %v", spec.RunStrategy)
	}
}

// TestDefaultTemplate_NoNodeSelector verifies template works without node selector.
func TestDefaultTemplate_NoNodeSelector(t *testing.T) {
	values := &builder.VMBuildValues{
		RunStrategy: "Halted",
		Sockets:     1,
		Cores:       2,
		MemoryMiB:   4096,
		Serial:      "i-noaz",
		IsUEFI:      false,
		HasACPI:     true,
		InputBus:    "virtio",
		Disks: []builder.DiskBuildValues{
			{Name: "disk-0", PVCName: "boot", Bus: "virtio", IsBootDisk: true, BootOrder: 1},
		},
		Networks: []builder.NetworkBuildValues{
			{Name: "net-0", Type: "pod", Model: "virtio", BindingMethod: "masquerade"},
		},
		// No NodeSelector
	}

	spec, err := builder.RenderTemplate(DefaultVMTemplate, values)
	if err != nil {
		t.Fatalf("DefaultVMTemplate render failed: %v", err)
	}

	if len(spec.Template.Spec.NodeSelector) > 0 {
		t.Error("expected no node selector")
	}
}
