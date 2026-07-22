package model

import (
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

var NotFound = libmodel.NotFound

const (
	MaxDetail = 3
)

type Base struct {
	UID      string `sql:"pk"`
	Name     string `sql:"d0,index(name)"`
	Kind     string `sql:"d0,index(kind)"`
	Provider string `sql:"d0,index(provider)"`
	Revision int64  `sql:"incremented,d0,index(revision)"`
}

func (m *Base) Pk() string {
	return m.UID
}

func (m *Base) String() string {
	return m.UID
}

// VM represents an Azure Virtual Machine.
type VM struct {
	Base
	VMSize     string                    `sql:"d0,index(vmSize)"`
	PowerState string                    `sql:"d0,index(powerState)"`
	OSType     string                    `sql:"d0,index(osType)"`
	CpuCount   int32                     `sql:"d0"`
	MemoryMB   int32                     `sql:"d0"`
	GuestId    string                    `sql:"d0"`
	Disks      []VMDisk                  `sql:"d0"`
	Object     armcompute.VirtualMachine `sql:"d0"`
}

func (m *VM) Labels() libmodel.Labels {
	if len(m.Object.Tags) == 0 {
		return nil
	}
	labels := make(libmodel.Labels, len(m.Object.Tags))
	for key, val := range m.Object.Tags {
		if val != nil {
			labels[key] = *val
		}
	}
	return labels
}

func (m *VM) GetDetails() (*VMDetails, error) {
	details := &VMDetails{
		VirtualMachine: m.Object,
		ID:             m.UID,
		Name:           m.Name,
		Kind:           m.Kind,
		Provider:       m.Provider,
		Revision:       m.Revision,
		PowerState:     m.PowerState,
		CpuCount:       m.CpuCount,
		MemoryMB:       m.MemoryMB,
		GuestId:        m.GuestId,
		Disks:          m.Disks,
	}

	if props := m.Object.Properties; props != nil {
		if sp := props.StorageProfile; sp != nil {
			if osDisk := sp.OSDisk; osDisk != nil && osDisk.ManagedDisk != nil {
				d := VMDisk{
					IsOS: true,
				}
				if osDisk.Name != nil {
					d.Name = *osDisk.Name
				}
				if osDisk.ManagedDisk.ID != nil {
					d.ID = *osDisk.ManagedDisk.ID
				}
				if osDisk.DiskSizeGB != nil {
					d.SizeGB = *osDisk.DiskSizeGB
				}
				if osDisk.ManagedDisk.StorageAccountType != nil {
					d.Sku = string(*osDisk.ManagedDisk.StorageAccountType)
				}
				if osDisk.OSType != nil {
					d.OSType = string(*osDisk.OSType)
				}
				details.ManagedDisks = append(details.ManagedDisks, d)
			}
			for _, dataDisk := range sp.DataDisks {
				if dataDisk == nil || dataDisk.ManagedDisk == nil {
					continue
				}
				d := VMDisk{}
				if dataDisk.Name != nil {
					d.Name = *dataDisk.Name
				}
				if dataDisk.ManagedDisk.ID != nil {
					d.ID = *dataDisk.ManagedDisk.ID
				}
				if dataDisk.DiskSizeGB != nil {
					d.SizeGB = *dataDisk.DiskSizeGB
				}
				if dataDisk.ManagedDisk.StorageAccountType != nil {
					d.Sku = string(*dataDisk.ManagedDisk.StorageAccountType)
				}
				details.ManagedDisks = append(details.ManagedDisks, d)
			}
		}
		if np := props.NetworkProfile; np != nil {
			for _, nic := range np.NetworkInterfaces {
				if nic == nil {
					continue
				}
				iface := VMNetworkInterface{}
				if nic.ID != nil {
					iface.ID = *nic.ID
				}
				if nic.Properties != nil && nic.Properties.Primary != nil {
					iface.Primary = *nic.Properties.Primary
				}
				details.NetworkInterfaces = append(details.NetworkInterfaces, iface)
			}
		}
	}

	return details, nil
}

func (m *VM) HasChanged(new *VM) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.VMSize != new.VMSize || m.PowerState != new.PowerState || m.OSType != new.OSType {
		return true
	}
	if m.CpuCount != new.CpuCount || m.MemoryMB != new.MemoryMB || m.GuestId != new.GuestId {
		return true
	}
	if !reflect.DeepEqual(m.Disks, new.Disks) {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type VMDisk struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	SizeGB int32  `json:"sizeGB"`
	Sku    string `json:"sku"`
	IsOS   bool   `json:"isOS"`
	OSType string `json:"osType,omitempty"`
}

type VMNetworkInterface struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SubnetID string `json:"subnetId,omitempty"`
	Primary  bool   `json:"primary"`
}

type VMDetails struct {
	armcompute.VirtualMachine
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Kind              string               `json:"kind"`
	Provider          string               `json:"provider"`
	Revision          int64                `json:"revision"`
	PowerState        string               `json:"powerState"`
	CpuCount          int32                `json:"cpuCount"`
	MemoryMB          int32                `json:"memoryMB"`
	GuestId           string               `json:"guestId"`
	Disks             []VMDisk             `json:"disks,omitempty"`
	ManagedDisks      []VMDisk             `json:"managedDisks,omitempty"`
	NetworkInterfaces []VMNetworkInterface `json:"networkInterfaces,omitempty"`
}

// Disk represents an Azure Managed Disk.
type Disk struct {
	Base
	DiskType string          `sql:"d0,index(diskType)"`
	State    string          `sql:"d0,index(state)"`
	SizeGB   int64           `sql:"d0,index(sizeGB)"`
	Object   armcompute.Disk `sql:"d0"`
}

func (m *Disk) Labels() libmodel.Labels {
	if len(m.Object.Tags) == 0 {
		return nil
	}
	labels := make(libmodel.Labels, len(m.Object.Tags))
	for key, val := range m.Object.Tags {
		if val != nil {
			labels[key] = *val
		}
	}
	return labels
}

func (m *Disk) GetDetails() (*DiskDetails, error) {
	d := &DiskDetails{
		Disk:     m.Object,
		ID:       m.UID,
		Name:     m.Name,
		Kind:     m.Kind,
		Provider: m.Provider,
		Revision: m.Revision,
	}
	if m.Object.Properties != nil && m.Object.Properties.DiskSizeGB != nil {
		d.SizeGB = *m.Object.Properties.DiskSizeGB
	}
	if m.Object.SKU != nil && m.Object.SKU.Name != nil {
		d.Sku = string(*m.Object.SKU.Name)
	}
	return d, nil
}

func (m *Disk) HasChanged(new *Disk) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.DiskType != new.DiskType || m.State != new.State || m.SizeGB != new.SizeGB {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type DiskDetails struct {
	armcompute.Disk
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Revision int64  `json:"revision"`
	SizeGB   int32  `json:"sizeGB"`
	Sku      string `json:"sku"`
}

// Network represents an Azure Subnet within a Virtual Network.
type Network struct {
	Base
	NetworkType   string            `sql:"d0,index(networkType)"`
	AddressPrefix string            `sql:"d0,index(addressPrefix)"`
	Object        armnetwork.Subnet `sql:"d0"`
}

func (m *Network) Labels() libmodel.Labels {
	return nil
}

func (m *Network) GetDetails() (*NetworkDetails, error) {
	return &NetworkDetails{
		Subnet:        m.Object,
		ID:            m.UID,
		Name:          m.Name,
		Kind:          m.Kind,
		Provider:      m.Provider,
		Revision:      m.Revision,
		Variant:       m.NetworkType,
		AddressPrefix: m.AddressPrefix,
	}, nil
}

func (m *Network) HasChanged(new *Network) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.NetworkType != new.NetworkType || m.AddressPrefix != new.AddressPrefix {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type NetworkDetails struct {
	armnetwork.Subnet
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Provider      string `json:"provider"`
	Revision      int64  `json:"revision"`
	Variant       string `json:"variant"`
	AddressPrefix string `json:"addressPrefix,omitempty"`
}

// Storage represents Azure disk types (SKUs).
type Storage struct {
	Base
	SKU    string      `sql:"d0,index(sku)"`
	Object StorageData `sql:"d0"`
}

type StorageData struct {
	SKU         string `json:"sku"`
	Description string `json:"description"`
	MaxIOPS     int32  `json:"maxIOPS"`
	MaxMBps     int32  `json:"maxMBps"`
}

func (m *Storage) Labels() libmodel.Labels {
	return nil
}

func (m *Storage) GetDetails() (*StorageDetails, error) {
	return &StorageDetails{
		ID:          m.UID,
		Name:        m.Name,
		Kind:        m.Kind,
		Provider:    m.Provider,
		Revision:    m.Revision,
		SKU:         m.Object.SKU,
		Description: m.Object.Description,
		MaxIOPS:     m.Object.MaxIOPS,
		MaxMBps:     m.Object.MaxMBps,
	}, nil
}

func (m *Storage) HasChanged(new *Storage) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.SKU != new.SKU {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type StorageDetails struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Provider    string `json:"provider"`
	Revision    int64  `json:"revision"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	MaxIOPS     int32  `json:"maxIOPS"`
	MaxMBps     int32  `json:"maxMBps"`
}

// All returns all Azure model types for database registration.
func All() []interface{} {
	return []interface{}{
		&VM{},
		&Disk{},
		&Network{},
		&Storage{},
	}
}
