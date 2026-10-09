package ovf

import (
	"encoding/xml"
	"testing"
)

func TestApplyNtnxConfigs(t *testing.T) {
	vm := &VM{}
	vm.ApplyNtnxConfigs([]NtnxConfig{
		{Key: "uefi_boot", Value: "True"},
		{Key: "secure_boot", Value: "False"},
		{Key: "num_sockets", Value: "2"},
		{Key: "num_vcpus_per_socket", Value: "4"},
		{Key: "num_threads_per_core", Value: "1"},
		{Key: "vtpmEnabled", Value: "True"},
		{Key: "machineType", Value: "pc"},
		{Key: "hardware_virtualization", Value: "True"},
		{Key: "boot_device_order", Value: "DISK,CDROM,NETWORK"},
		{Key: "hardwareClockTimeZone", Value: "America/Los_Angeles"},
		{Key: "isAgentVm", Value: "False"},
		{Key: "cpuPassthroughEnabled", Value: "True"},
	})
	vm.normalizeNtnxTopology()

	if vm.Firmware != "efi" {
		t.Fatalf("Firmware: got %q, want efi", vm.Firmware)
	}
	if vm.SecureBoot {
		t.Fatal("SecureBoot: got true, want false")
	}
	if vm.NumSockets != 2 || vm.CoresPerSocket != 4 || vm.ThreadsPerCore != 1 {
		t.Fatalf("topology: sockets=%d cores=%d threads=%d", vm.NumSockets, vm.CoresPerSocket, vm.ThreadsPerCore)
	}
	if vm.CpuCount != 8 {
		t.Fatalf("CpuCount: got %d, want 8", vm.CpuCount)
	}
	if !vm.TpmEnabled {
		t.Fatal("TpmEnabled: got false, want true")
	}
	if vm.MachineType != "pc" {
		t.Fatalf("MachineType: got %q, want pc", vm.MachineType)
	}
	if !vm.NestedVirtualization {
		t.Fatal("NestedVirtualization: got false, want true")
	}
	if vm.BootDeviceOrder != "DISK,CDROM,NETWORK" {
		t.Fatalf("BootDeviceOrder: got %q", vm.BootDeviceOrder)
	}
	if vm.HardwareClockTimezone != "America/Los_Angeles" {
		t.Fatalf("HardwareClockTimezone: got %q", vm.HardwareClockTimezone)
	}
	if vm.IsAgentVm {
		t.Fatal("IsAgentVm: got true, want false")
	}
	if !vm.CpuPassthroughEnabled {
		t.Fatal("CpuPassthroughEnabled: got false, want true")
	}
}

func TestApplyNtnxConfigs_SecureBootImpliesEFI(t *testing.T) {
	vm := &VM{}
	vm.ApplyNtnxConfigs([]NtnxConfig{{Key: "secure_boot", Value: "True"}})
	if vm.Firmware != "efi" {
		t.Fatalf("Firmware: got %q, want efi", vm.Firmware)
	}
	if !vm.SecureBoot {
		t.Fatal("SecureBoot: got false, want true")
	}
}

func TestApplyNtnxConfigs_UefiBootFalseDefaultsBIOS(t *testing.T) {
	vm := &VM{}
	vm.ApplyNtnxConfigs([]NtnxConfig{{Key: "uefi_boot", Value: "False"}})
	if vm.Firmware != "bios" {
		t.Fatalf("Firmware: got %q, want bios", vm.Firmware)
	}
}

func TestNormalizeNtnxTopology_DefaultsThreads(t *testing.T) {
	vm := &VM{NumSockets: 2, CoresPerSocket: 3}
	vm.normalizeNtnxTopology()
	if vm.ThreadsPerCore != 1 {
		t.Fatalf("ThreadsPerCore: got %d, want 1", vm.ThreadsPerCore)
	}
	if vm.CpuCount != 6 {
		t.Fatalf("CpuCount: got %d, want 6", vm.CpuCount)
	}
}

func TestGuessSource_NutanixPreferredOverVMware(t *testing.T) {
	got := GuessSource(Envelope{Attributes: []xml.Attr{
		{Value: "http://www.vmware.com/schema/ovf"},
		{Value: "http://www.nutanix.com/ova"},
	}})
	if got != SourceNutanix {
		t.Fatalf("got %q, want %q", got, SourceNutanix)
	}
}

func TestGuessSource_VMware(t *testing.T) {
	got := GuessSource(Envelope{Attributes: []xml.Attr{
		{Value: "http://www.vmware.com/schema/ovf"},
	}})
	if got != SourceVMware {
		t.Fatalf("got %q, want %q", got, SourceVMware)
	}
}
