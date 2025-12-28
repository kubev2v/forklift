package ovfbase

import (
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
)

type Base struct {
	ID          string `json:"ID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

// VM.
type VM struct {
	Name                  string   `json:"Name"`
	OvfPath               string   `json:"OvfPath"`
	ExportSource          string   `json:"ExportSource"`
	OsType                string   `json:"OsType"`
	RevisionValidated     int64    `json:"RevisionValidated"`
	PolicyVersion         int      `json:"PolicyVersion"`
	UUID                  string   `json:"UUID"`
	Firmware              string   `json:"Firmware"`
	SecureBoot            bool     `json:"SecureBoot"`
	CpuAffinity           []int32  `json:"CpuAffinity"`
	CpuHotAddEnabled      bool     `json:"CpuHotAddEnabled"`
	CpuHotRemoveEnabled   bool     `json:"CpuHotRemoveEnabled"`
	MemoryHotAddEnabled   bool     `json:"MemoryHotAddEnabled"`
	FaultToleranceEnabled bool     `json:"FaultToleranceEnabled"`
	CpuCount              int32    `json:"CpuCount"`
	CoresPerSocket        int32    `json:"CoresPerSocket"`
	MemoryMB              int32    `json:"MemoryMB"`
	BalloonedMemory       int32    `json:"BalloonedMemory"`
	MemoryUnits           string   `json:"MemoryUnits"`
	CpuUnits              string   `json:"CpuUnits"`
	IpAddress             string   `json:"IpAddress"`
	NumaNodeAffinity      []string `json:"NumaNodeAffinity"`
	StorageUsed           int64    `json:"StorageUsed"`
	ChangeTrackingEnabled bool     `json:"ChangeTrackingEnabled"`
	Devices               []struct {
		Kind string `json:"Kind"`
	} `json:"Devices"`
	NICs []struct {
		Name    string `json:"Name"`
		MAC     string `json:"MAC"`
		Network string `json:"Network"`
		Config  []struct {
			Key   string `json:"Key"`
			Value string `json:"Value"`
		} `json:"Config"`
	} `json:"Nics"`
	Disks []struct {
		ID                      string `json:"ID"`
		Name                    string `json:"Name"`
		FilePath                string `json:"FilePath"`
		Capacity                int64  `json:"Capacity"`
		CapacityAllocationUnits string `json:"CapacityAllocationUnits"`
		DiskId                  string `json:"DiskId"`
		FileRef                 string `json:"FileRef"`
		Format                  string `json:"Format"`
		PopulatedSize           int64  `json:"PopulatedSize"`
	} `json:"Disks"`
	Networks []struct {
		ID          string `json:"ID"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
	} `json:"Networks"`
}

// Apply to (update) the model.
func (r *VM) ApplyTo(m *model.VM) {
	m.Name = r.Name
	m.ID = r.UUID
	m.OvfPath = r.OvfPath
	m.ExportSource = r.ExportSource
	m.OsType = r.OsType
	m.RevisionValidated = r.RevisionValidated
	m.PolicyVersion = r.PolicyVersion
	m.UUID = r.UUID
	m.Firmware = r.Firmware
	m.SecureBoot = r.SecureBoot
	m.CpuAffinity = r.CpuAffinity
	m.CpuHotAddEnabled = r.CpuHotAddEnabled
	m.CpuHotRemoveEnabled = r.CpuHotRemoveEnabled
	m.MemoryHotAddEnabled = r.MemoryHotAddEnabled
	m.FaultToleranceEnabled = r.FaultToleranceEnabled
	m.CpuCount = r.CpuCount
	m.CoresPerSocket = r.CoresPerSocket
	m.MemoryMB = r.MemoryMB
	m.MemoryUnits = r.MemoryUnits
	m.CpuUnits = r.CpuUnits
	m.BalloonedMemory = r.BalloonedMemory
	m.IpAddress = r.IpAddress
	m.NumaNodeAffinity = r.NumaNodeAffinity
	m.StorageUsed = r.StorageUsed
	m.ChangeTrackingEnabled = r.ChangeTrackingEnabled
	r.addNICs(m)
	r.addDisks(m)
	r.addDevices(m)
	r.addNetworks(m)
}

func (r *VM) addNICs(m *model.VM) {
	m.NICs = []model.NIC{}
	for _, n := range r.NICs {
		configs := []model.Conf{}
		for _, conf := range n.Config {
			configs = append(
				configs,
				model.Conf{
					Key:   conf.Key,
					Value: conf.Value,
				})
		}
		m.NICs = append(
			m.NICs, model.NIC{
				Name:    n.Name,
				MAC:     n.MAC,
				Config:  configs,
				Network: n.Network,
			})
	}
}

func (r *VM) addDisks(m *model.VM) {
	m.Disks = []model.Disk{}
	for _, disk := range r.Disks {
		m.Disks = append(
			m.Disks,
			model.Disk{
				Base: model.Base{
					Name: disk.Name,
					ID:   disk.ID,
				},
				FilePath:                disk.FilePath,
				Capacity:                disk.Capacity,
				CapacityAllocationUnits: disk.CapacityAllocationUnits,
				DiskId:                  disk.DiskId,
				FileRef:                 disk.FileRef,
				Format:                  disk.Format,
				PopulatedSize:           disk.PopulatedSize,
			})
	}
}

func (r *VM) addDevices(m *model.VM) {
	m.Devices = []model.Device{}
	for _, device := range r.Devices {
		m.Devices = append(
			m.Devices,
			model.Device{
				Kind: device.Kind,
			})
	}
}

func (r *VM) addNetworks(m *model.VM) {
	m.Networks = []model.Network{}
	for _, network := range r.Networks {
		m.Networks = append(
			m.Networks,
			model.Network{
				Description: network.Description,
				Base: model.Base{
					Name: network.Name,
					ID:   network.ID,
				},
			})
	}
}

// Network.
type Network struct {
	ID          string `json:"ID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
}

// Apply to (update) the model.
func (r *Network) ApplyTo(m *model.Network) {
	m.ID = r.ID
	m.Description = r.Description
	m.Base.Name = r.Name
}

// Disk.
type Disk struct {
	ID                      string `json:"ID"`
	Name                    string `json:"Name"`
	FilePath                string `json:"FilePath"`
	Capacity                int64  `json:"Capacity"`
	CapacityAllocationUnits string `json:"Capacity_allocation_units"`
	DiskId                  string `json:"DiskId"`
	FileRef                 string `json:"FileRef"`
	Format                  string `json:"Format"`
	PopulatedSize           int64  `json:"PopulatedSize"`
}

// Apply to (update) the model.
func (r *Disk) ApplyTo(m *model.Disk) {
	m.Base.Name = r.Name
	m.Base.ID = r.ID
	m.FilePath = r.FilePath
	m.Capacity = r.Capacity
	m.CapacityAllocationUnits = r.CapacityAllocationUnits
	m.DiskId = r.DiskId
	m.FileRef = r.FileRef
	m.Format = r.Format
	m.PopulatedSize = r.PopulatedSize
}

type Storage struct {
	Name string
	ID   string
}

func (r *Storage) ApplyTo(m *model.Storage) {
	m.Base.Name = m.Name
	m.Base.ID = m.ID
}
