package model

import (
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

var NotFound = libmodel.NotFound

// d0..d3 = detail levels for inventory queries (d0 = lightest list, d3 = full).
const (
	MaxDetail = 3
)

// Base is embedded by every inventory model (VM, Disk, Network, Storage).
// pk = primary key. UID holds the qualified ID ({resourceGroup}--{name}).
// Revision auto-increments on every DB update (optimistic concurrency).
type Base struct {
	UID           string `sql:"pk"`
	Name          string `sql:"d0,index(name)"`
	ResourceGroup string `sql:"d0,index(resourceGroup)"`
	Kind          string `sql:"d0,index(kind)"`
	Provider      string `sql:"d0,index(provider)"`
	Revision      int64  `sql:"incremented,d0,index(revision)"`
}

func (m *Base) Pk() string {
	return m.UID
}

func (m *Base) String() string {
	return m.UID
}

type VM struct {
	Base
	VMSize     string   `sql:"d0,index(vmSize)"`     // Azure SKU size, e.g. "Standard_D2s_v3"
	PowerState string   `sql:"d0,index(powerState)"` // e.g. "PowerState/running", "PowerState/deallocated"
	OSType     string   `sql:"d0,index(osType)"`
	CpuCount   int32    `sql:"d0"`
	MemoryMB   int32    `sql:"d0"`
	GuestId    string   `sql:"d0"` // guest OS identifier
	Disks      []VMDisk `sql:"d0"`
}

func (m *VM) Labels() libmodel.Labels {
	return nil
}

func (m *VM) GetDetails() (*VMDetails, error) {
	return &VMDetails{
		ID:            m.UID,
		Name:          m.Name,
		ResourceGroup: m.ResourceGroup,
		Kind:          m.Kind,
		Provider:      m.Provider,
		Revision:      m.Revision,
	}, nil
}

// HasChanged is a cheap diff for inventory reconciliation — only update the DB row if needed.
func (m *VM) HasChanged(new *VM) bool {
	return m.Name != new.Name || m.Kind != new.Kind ||
		m.VMSize != new.VMSize || m.PowerState != new.PowerState
}

type VMDisk struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	SizeGB int32  `json:"sizeGB"`
	SKU    string `json:"sku"`  // disk performance tier, e.g. "Premium_LRS", "Standard_SSD_LRS"
	IsOS   bool   `json:"isOS"` // true = boot/OS disk, false = data disk
	OSType string `json:"osType,omitempty"`
}

type VMNetworkInterface struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SubnetID string `json:"subnetId,omitempty"` // Azure resource ID of the subnet this NIC is in
	Primary  bool   `json:"primary"`
}

// VMDetails is the JSON-friendly view returned by the REST API.
type VMDetails struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	ResourceGroup     string               `json:"resourceGroup"`
	Kind              string               `json:"kind"`
	Provider          string               `json:"provider"`
	Revision          int64                `json:"revision"`
	PowerState        string               `json:"powerState"`
	CpuCount          int32                `json:"cpuCount"`
	MemoryMB          int32                `json:"memoryMB"`
	GuestId           string               `json:"guestId"`
	Disks             []VMDisk             `json:"disks,omitempty"`
	ManagedDisks      []VMDisk             `json:"managedDisks,omitempty"` // Azure-managed disks (vs. unmanaged/blob)
	NetworkInterfaces []VMNetworkInterface `json:"networkInterfaces,omitempty"`
	Location          *string              `json:"location,omitempty"` // Azure region, e.g. "eastus"
}

// Disk represents an Azure managed disk.
type Disk struct {
	Base
	DiskType string `sql:"d0,index(diskType)"` // performance tier, e.g. "Premium_LRS"
	State    string `sql:"d0,index(state)"`    // lifecycle state: "Attached", "Unattached", etc.
	SizeGB   int64  `sql:"d0,index(sizeGB)"`
}

func (m *Disk) Labels() libmodel.Labels {
	return nil
}

func (m *Disk) GetDetails() (*DiskDetails, error) {
	return &DiskDetails{
		ID:            m.UID,
		Name:          m.Name,
		ResourceGroup: m.ResourceGroup,
		Kind:          m.Kind,
		Provider:      m.Provider,
		Revision:      m.Revision,
	}, nil
}

func (m *Disk) HasChanged(new *Disk) bool {
	return m.Name != new.Name || m.DiskType != new.DiskType ||
		m.State != new.State || m.SizeGB != new.SizeGB
}

type DiskDetails struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
	Kind          string `json:"kind"`
	Provider      string `json:"provider"`
	Revision      int64  `json:"revision"`
	SizeGB        int32  `json:"sizeGB"`
	SKU           string `json:"sku"` // disk performance tier
}

// Network represents either a VNet or a Subnet (both stored as Network rows).
type Network struct {
	Base
	NetworkType string `sql:"d0,index(networkType)"` // "VNet" or "Subnet"
	CIDR        string `sql:"d0,index(cidr)"`        // CIDR block, e.g. "10.0.0.0/16"
}

func (m *Network) Labels() libmodel.Labels {
	return nil
}

func (m *Network) GetDetails() (*NetworkDetails, error) {
	return &NetworkDetails{
		ID:            m.UID,
		Name:          m.Name,
		ResourceGroup: m.ResourceGroup,
		Kind:          m.Kind,
		Provider:      m.Provider,
		Revision:      m.Revision,
		Variant:       m.NetworkType,
		CIDR:          m.CIDR,
	}, nil
}

func (m *Network) HasChanged(new *Network) bool {
	return m.Name != new.Name || m.NetworkType != new.NetworkType ||
		m.CIDR != new.CIDR
}

type NetworkDetails struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
	Kind          string `json:"kind"`
	Provider      string `json:"provider"`
	Revision      int64  `json:"revision"`
	Variant       string `json:"variant"`
	CIDR          string `json:"cidr,omitempty"`
}

// Storage represents an Azure disk SKU (performance tier definition, not a specific disk).
type Storage struct {
	Base
	SKU    string      `sql:"d0,index(sku)"` // e.g. "Premium_LRS", "StandardSSD_LRS"
	Object StorageData `sql:"d0"`
}

// StorageData holds the performance characteristics of a disk SKU.
type StorageData struct {
	SKU         string `json:"sku"`
	Description string `json:"description"`
	MaxIOPS     int32  `json:"maxIOPS"` // max I/O operations per second
	MaxMBps     int32  `json:"maxMBps"` // max throughput in MB/s
}

func (m *Storage) Labels() libmodel.Labels {
	return nil
}

func (m *Storage) GetDetails() (*StorageDetails, error) {
	return &StorageDetails{
		ID:            m.UID,
		Name:          m.Name,
		ResourceGroup: m.ResourceGroup,
		Kind:          m.Kind,
		Provider:      m.Provider,
		Revision:      m.Revision,
		SKU:           m.Object.SKU,
		Description:   m.Object.Description,
		MaxIOPS:       m.Object.MaxIOPS,
		MaxMBps:       m.Object.MaxMBps,
	}, nil
}

func (m *Storage) HasChanged(new *Storage) bool {
	return m.Name != new.Name || m.SKU != new.SKU
}

type StorageDetails struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
	Kind          string `json:"kind"`
	Provider      string `json:"provider"`
	Revision      int64  `json:"revision"`
	SKU           string `json:"sku"`
	Description   string `json:"description"`
	MaxIOPS       int32  `json:"maxIOPS"`
	MaxMBps       int32  `json:"maxMBps"`
}

// All returns every model type so the DB can auto-create all tables.
func All() []interface{} {
	return []interface{}{
		&VM{},
		&Disk{},
		&Network{},
		&Storage{},
	}
}
