package ova

import "strconv"

// vm struct
type VM struct {
	ID                    string      `json:"id"` // Maps to UUID for dynamic inventory
	Name                  string      `json:"name"`
	OvaPath               string      `json:"ovaPath"`
	OvaSource             string      `json:"ovaSource"`
	OsType                string      `json:"osType"`
	RevisionValidated     int64       `json:"revisionValidated"`
	PolicyVersion         int         `json:"policyVersion"`
	UUID                  string      `json:"uuid"` // Keep for backwards compatibility
	Firmware              string      `json:"firmware"`
	SecureBoot            bool        `json:"secureBoot"`
	CpuAffinity           []int32     `json:"cpuAffinity"`
	CpuHotAddEnabled      bool        `json:"cpuHotAddEnabled"`
	CpuHotRemoveEnabled   bool        `json:"cpuHotRemoveEnabled"`
	MemoryHotAddEnabled   bool        `json:"memoryHotAddEnabled"`
	FaultToleranceEnabled bool        `json:"faultToleranceEnabled"`
	CpuCount              int32       `json:"cpuCount"` // Maps to cpus for change detection
	CoresPerSocket        int32       `json:"coresPerSocket"`
	MemoryMB              int32       `json:"memoryMB"` // Maps to memory for change detection
	MemoryUnits           string      `json:"memoryUnits"`
	CpuUnits              string      `json:"cpuUnits"`
	BalloonedMemory       int32       `json:"balloonedMemory"`
	IpAddress             string      `json:"ipAddress"`
	NumaNodeAffinity      []string    `json:"numaNodeAffinity"`
	StorageUsed           int64       `json:"storageUsed"`
	ChangeTrackingEnabled bool        `json:"changeTrackingEnabled"`
	Devices               []Device    `json:"devices"`
	NICs                  []NIC       `json:"nics"`
	Disks                 []VmDisk    `json:"disks"`
	Networks              []VmNetwork `json:"networks"`
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

// Virtual Disk.
type VmDisk struct {
	ID                      string `json:"id"` // Maps to DiskId for dynamic inventory
	Name                    string `json:"name"`
	FilePath                string `json:"filePath"`
	Capacity                int64  `json:"capacity"`
	CapacityAllocationUnits string `json:"capacityAllocationUnits"`
	DiskId                  string `json:"diskId"` // Keep for backwards compatibility
	FileRef                 string `json:"fileRef"`
	Format                  string `json:"format"`
	PopulatedSize           int64  `json:"populatedSize"`
}

// Virtual Device.
type Device struct {
	Kind string `json:"kind"`
}

type Conf struct {
	//nolint:unused
	key string

	Value string
}

// Virtual ethernet card.
type NIC struct {
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	Network string `json:"network"`
	Config  []Conf `json:"config"`
}

type VmNetwork struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
