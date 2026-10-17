package ovf

import (
	"strconv"
	"strings"
)

// VM struct represents a virtual machine from OVF descriptor
type VM struct {
	Name                  string
	OvfPath               string
	ExportSource          string
	OsType                string
	RevisionValidated     int64
	PolicyVersion         int
	UUID                  string
	Firmware              string
	SecureBoot            bool
	CpuAffinity           []int32
	CpuHotAddEnabled      bool
	CpuHotRemoveEnabled   bool
	MemoryHotAddEnabled   bool
	FaultToleranceEnabled bool
	CpuCount              int32
	CoresPerSocket        int32
	MemoryMB              int32
	MemoryUnits           string
	CpuUnits              string
	BalloonedMemory       int32
	IpAddress             string
	NumaNodeAffinity      []string
	StorageUsed           int64
	ChangeTrackingEnabled bool
	Devices               []Device
	NICs                  []NIC
	Disks                 []VmDisk
	Networks              []VmNetwork
	// The following fields are Nutanix OVA only (ntnx:NtnxConfig);
	// unset for VMware and other sources.
	NumSockets            int32
	ThreadsPerCore        int32
	TpmEnabled            bool
	MachineType           string
	IsAgentVm             bool
	CpuPassthroughEnabled bool
	NestedVirtualization  bool
	BootDeviceOrder       string
	HardwareClockTimezone string
}

func (r *VM) ApplyVirtualConfig(configs []VirtualConfig) {
	for _, config := range configs {
		r.apply(config.Key, config.Value)
	}
}

func (r *VM) ApplyExtraVirtualConfig(configs []ExtraVirtualConfig) {
	for _, config := range configs {
		r.apply(config.Key, config.Value)
	}
}

// ApplyNtnxFromVirtualSystem applies Prism Central OVA extension keys from
// CPU items, the virtual hardware section, boot-order section, and vTPM
// section. Call after VMware Config/ExtraConfig so Nutanix values win for
// Nutanix-exported appliances (which also declare the VMware schema).
func (r *VM) ApplyNtnxFromVirtualSystem(vs VirtualSystem) {
	for _, item := range vs.HardwareSection.Items {
		r.ApplyNtnxConfigs(item.NtnxConfigs)
	}
	r.ApplyNtnxConfigs(vs.HardwareSection.NtnxConfigs)
	r.ApplyNtnxConfigs(vs.BootOrderSection.Configs)
	r.ApplyNtnxConfigs(vs.VtpmSection.Configs)
	r.normalizeNtnxTopology()
}

// ApplyNtnxConfigs applies Prism Central ntnx:NtnxConfig key/value pairs to the VM.
func (r *VM) ApplyNtnxConfigs(configs []NtnxConfig) {
	for _, config := range configs {
		r.applyNtnx(config.Key, config.Value)
	}
}

func (r *VM) apply(key string, value string) {
	switch key {
	case "firmware":
		r.Firmware = value
	case "bootOptions.efiSecureBootEnabled":
		r.SecureBoot, _ = strconv.ParseBool(value)
	case "uefi.secureBoot.enabled":
		// Legacy key used in some vSphere and Workstation/Fusion OVAs
		r.SecureBoot, _ = strconv.ParseBool(value)
	case "memoryHotAddEnabled":
		r.MemoryHotAddEnabled, _ = strconv.ParseBool(value)
	case "cpuHotAddEnabled":
		r.CpuHotAddEnabled, _ = strconv.ParseBool(value)
	case "cpuHotRemoveEnabled":
		r.CpuHotRemoveEnabled, _ = strconv.ParseBool(value)
	}
}

// applyNtnx maps a single Nutanix OVA extension key onto VM fields.
func (r *VM) applyNtnx(key, value string) {
	switch key {
	case "uefi_boot":
		if parseNtnxBool(value) {
			r.Firmware = "efi"
		} else if !r.SecureBoot {
			// Explicit Nutanix BIOS wins over a prior VMware firmware=efi,
			// unless secure_boot already forced EFI.
			r.Firmware = "bios"
		}
	case "secure_boot":
		r.SecureBoot = parseNtnxBool(value)
		if r.SecureBoot {
			r.Firmware = "efi"
		}
	case "num_vcpus_per_socket":
		if n, err := strconv.ParseInt(value, 10, 32); err == nil && n > 0 {
			r.CoresPerSocket = int32(n)
		}
	case "num_sockets":
		if n, err := strconv.ParseInt(value, 10, 32); err == nil && n > 0 {
			r.NumSockets = int32(n)
		}
	case "num_threads_per_core":
		if n, err := strconv.ParseInt(value, 10, 32); err == nil && n > 0 {
			r.ThreadsPerCore = int32(n)
		}
	case "vtpmEnabled":
		r.TpmEnabled = parseNtnxBool(value)
	case "machineType":
		r.MachineType = value
	case "isAgentVm":
		r.IsAgentVm = parseNtnxBool(value)
	case "cpuPassthroughEnabled":
		r.CpuPassthroughEnabled = parseNtnxBool(value)
	case "hardware_virtualization":
		r.NestedVirtualization = parseNtnxBool(value)
	case "boot_device_order":
		r.BootDeviceOrder = value
	case "hardwareClockTimeZone":
		r.HardwareClockTimezone = value
	}
}

// normalizeNtnxTopology recomputes CpuCount when Nutanix supplied a full
// sockets × cores × threads description, so mapCPU can derive topology
// consistently.
func (r *VM) normalizeNtnxTopology() {
	if r.NumSockets <= 0 || r.CoresPerSocket <= 0 {
		return
	}
	threads := r.ThreadsPerCore
	if threads <= 0 {
		threads = 1
		r.ThreadsPerCore = 1
	}
	r.CpuCount = r.NumSockets * r.CoresPerSocket * threads
}

// parseNtnxBool parses Nutanix "True"/"False" (and other strconv.ParseBool forms).
func parseNtnxBool(value string) bool {
	// Nutanix writes "True"/"False"; strconv.ParseBool accepts those.
	v, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && v
}

// VmDisk represents a virtual disk
type VmDisk struct {
	ID                      string
	Name                    string
	FilePath                string
	Capacity                int64
	CapacityAllocationUnits string
	DiskId                  string
	FileRef                 string
	Format                  string
	PopulatedSize           int64
}

// Device represents a virtual device
type Device struct {
	Kind string `json:"kind"`
}

type Conf struct {
	//nolint:unused
	key string

	Value string
}

// NIC represents a virtual ethernet card
type NIC struct {
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	Network string
	Config  []Conf
}

// VmNetwork represents a virtual network
type VmNetwork struct {
	Name        string
	Description string
	ID          string
}
