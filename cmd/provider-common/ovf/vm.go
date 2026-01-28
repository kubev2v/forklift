package ovf

import "strconv"

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
