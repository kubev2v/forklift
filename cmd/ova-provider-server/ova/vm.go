package ova

import "strconv"

// vm struct
type VM struct {
	Name                  string
	OvaPath               string
	OvaSource             string
	OsType                string
	RevisionValidated     int64
	PolicyVersion         int
	UUID                  string
	Firmware              string
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
	Network string
	Config  []Conf
}

type VmNetwork struct {
	Name        string
	Description string
	ID          string
}
