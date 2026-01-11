package vsphere

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

type ProtocolType string

// Variants.
const (
	// Network.
	NetStandard    = "Standard"
	NetDvPortGroup = "DvPortGroup"
	OpaqueNetwork  = "OpaqueNetwork"
	NetDvSwitch    = "DvSwitch"
	// Cluster.
	ComputeResource = "ComputeResource"
	// Storage Protocol Type
	ProtocolUnknown      ProtocolType = "Unknown"      // Unrecognized or unsupported
	ProtocolFibreChannel ProtocolType = "FibreChannel" // High-speed network tech
	ProtocolFCoE         ProtocolType = "FCoE"         // Fibre Channel over Ethernet
	ProtocolISCSI        ProtocolType = "iSCSI"        // Internet Small Computer Systems Interface
	ProtocolSCSI         ProtocolType = "ParallelSCSI" // Legacy parallel SCSI interface
	ProtocolSAS          ProtocolType = "SAS"          // Serial Attached SCSI
	ProtocolPCIe         ProtocolType = "PCIe"         // PCI Express storage interface
	ProtocolRDMA         ProtocolType = "RDMA"         // Remote Direct Memory Access
	ProtocolTCP          ProtocolType = "TCP"          // Generic TCP-based adapter
)

// Errors
var NotFound = libmodel.NotFound

type InvalidRefError = base.InvalidRefError

const (
	MaxDetail = base.MaxDetail
)

// Types
type ListOptions = base.ListOptions
type Concern = base.Concern
type Ref = base.Ref

// Model.
type Model interface {
	base.Model
	GetParent() Ref
	GetName() string
}

// Base VMWare model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Variant
	Variant string `sql:"d0,index(variant)"`
	// Name
	Name string `sql:"d0,index(name)"`
	// Parent
	Parent Ref `sql:"d0,index(parent)"`
	// Revision
	Revision int64 `sql:"incremented,d0,index(revision)"`
}

// Get the PK.
func (m *Base) Pk() string {
	return m.ID
}

// String representation.
func (m *Base) String() string {
	return m.ID
}

// Get labels.
func (m *Base) Labels() libmodel.Labels {
	return nil
}

func (m *Base) Equals(other libmodel.Model) bool {
	if vm, cast := other.(*VM); cast {
		return m.ID == vm.ID
	}

	return false
}

// Populate PK using the ref.
func (m *Base) WithRef(ref Ref) {
	m.ID = ref.ID
}

// Parent.
func (m *Base) GetParent() Ref {
	return m.Parent
}

// Name.
func (m *Base) GetName() string {
	return m.Name
}

type About struct {
	Base
	APIVersion   string `sql:""`
	Product      string `sql:""`
	InstanceUuid string `sql:""`
}

type Folder struct {
	Base
	Datacenter string `sql:"d0,index(datacenter)"`
	Folder     string `sql:"d0,index(folder)"`
	Children   []Ref  `sql:""`
}

type Datacenter struct {
	Base
	Clusters   Ref `sql:""`
	Networks   Ref `sql:""`
	Datastores Ref `sql:""`
	Vms        Ref `sql:""`
}

type Cluster struct {
	Base
	Folder      string `sql:"d0,index(folder)"`
	Hosts       []Ref  `sql:""`
	Networks    []Ref  `sql:""`
	Datastores  []Ref  `sql:""`
	DasEnabled  bool   `sql:""`
	DasVms      []Ref  `sql:""`
	DrsEnabled  bool   `sql:""`
	DrsBehavior string `sql:""`
	DrsVms      []Ref  `sql:""`
}

type Host struct {
	Base
	Cluster            string             `sql:"d0,index(cluster)"`
	Status             string             `sql:""`
	InMaintenanceMode  bool               `sql:""`
	ManagementServerIp string             `sql:""`
	ManagementIPs      []string           `sql:""`
	Thumbprint         string             `sql:""`
	Timezone           string             `sql:""`
	CpuSockets         int16              `sql:""`
	CpuCores           int16              `sql:""`
	MemoryBytes        int64              `sql:""`
	ProductName        string             `sql:""`
	ProductVersion     string             `sql:""`
	Model              string             `sql:""`
	Vendor             string             `sql:""`
	Network            HostNetwork        `sql:""`
	Networks           []Ref              `sql:""`
	Datastores         []Ref              `sql:""`
	HostScsiDisks      []HostScsiDisk     `sql:""`
	AdvancedOptions    Ref                `sql:""`
	HbaDiskInfo        []HbaDiskInfo      `sql:""`
	HostScsiTopology   []HostScsiTopology `sql:""`
}

type HostScsiDisk struct {
	// Canonical name of the SCSI logical unit.
	//
	// Disk partition or extent identifiers refer to this name when
	// referring to a disk. Use this property to correlate a partition
	// or extent to a specific SCSI disk.
	CanonicalName string `json:"canonicalName"`
	// The vendor of the SCSI device.
	Vendor string `json:"vendor"`
	// The model of the scsi device
	Model string `json:"model"`
	// The key of the scsi device
	Key string `json:"key"`
}

type HbaDiskInfo struct {
	// The device name of host bus adapter.
	Device string `json:"hbaDevice"`
	// The supported protocol by this device
	Protocol string `json:"protocol"`
	// The model name of the host bus adapter.
	Model string `json:"model"`
	// The linkable identifier.
	Key string `json:"key"`
}

type HostScsiTopology struct {
	// The identifier for the SCSI interface (HBA)
	HbaKey string `json:"key"`
	// List of identifiers for the SCSI targets
	ScsiDiskKeys []string `json:"scsiDiskKeys"`
}

type HostNetwork struct {
	PNICs      []PNIC      `json:"pNICs"`
	VNICs      []VNIC      `json:"vNICs"`
	PortGroups []PortGroup `json:"portGroups"`
	Switches   []Switch    `json:"switches"`
}

func (n *HostNetwork) Switch(key string) (vSwitch *Switch, found bool) {
	for _, object := range n.Switches {
		if key == object.Key {
			vSwitch = &object
			found = true
			break
		}
	}

	return
}
func (n *HostNetwork) PortGroup(name string) (portGroup *PortGroup, found bool) {
	for _, object := range n.PortGroups {
		if name == object.Name {
			portGroup = &object
			found = true
			break
		}
	}

	return
}

func (n *HostNetwork) PNIC(key string) (nic *PNIC, found bool) {
	for _, object := range n.PNICs {
		if key == object.Key {
			nic = &object
			found = true
			break
		}
	}

	return
}

type PNIC struct {
	Key       string `json:"key"`
	LinkSpeed int32  `json:"linkSpeed"`
}

type VNIC struct {
	Key        string `json:"key"`
	Device     string `json:"device"`
	PortGroup  string `json:"portGroup"`
	DPortGroup string `json:"dPortGroup"`
	IpAddress  string `json:"ipAddress"`
	SubnetMask string `json:"subnetMask"`
	MTU        int32  `json:"mtu"`
}

type PortGroup struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Switch string `json:"vSwitch"`
	VlanId int32  `json:"vlanId"`
}

type Switch struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	PortGroups []string `json:"portGroups"`
	PNICs      []string `json:"pNICs"`
}

type Network struct {
	Base
	Tag      string    `sql:""`
	DVSwitch Ref       `sql:""`
	Key      string    `sql:""`
	Host     []DVSHost `sql:""`
	VlanId   string    `sql:""`
}

type DVSHost struct {
	Host Ref
	PNIC []string
}

type Datastore struct {
	Base
	Type                string   `sql:""`
	Capacity            int64    `sql:""`
	Free                int64    `sql:""`
	MaintenanceMode     string   `sql:""`
	BackingDevicesNames []string `sql:""`
}

type VM struct {
	Base
	Folder                   string           `sql:"d0,index(folder)"`
	Host                     string           `sql:"d0,index(host)"`
	RevisionValidated        int64            `sql:"d0,index(revisionValidated)"`
	PolicyVersion            int              `sql:"d0,index(policyVersion)"`
	UUID                     string           `sql:""`
	Firmware                 string           `sql:""`
	PowerState               string           `sql:""`
	ConnectionState          string           `sql:""`
	CpuAffinity              []int32          `sql:""`
	CpuHotAddEnabled         bool             `sql:""`
	CpuHotRemoveEnabled      bool             `sql:""`
	MemoryHotAddEnabled      bool             `sql:""`
	FaultToleranceEnabled    bool             `sql:""`
	CpuCount                 int32            `sql:""`
	CoresPerSocket           int32            `sql:""`
	MemoryMB                 int32            `sql:""`
	GuestName                string           `sql:""`
	GuestNameFromVmwareTools string           `sql:""`
	HostName                 string           `sql:""`
	GuestID                  string           `sql:""`
	BalloonedMemory          int32            `sql:""`
	IpAddress                string           `sql:""`
	NumaNodeAffinity         []string         `sql:""`
	StorageUsed              int64            `sql:""`
	Snapshot                 Ref              `sql:""`
	IsTemplate               bool             `sql:""`
	ChangeTrackingEnabled    bool             `sql:""`
	TpmEnabled               bool             `sql:""`
	Devices                  []Device         `sql:""`
	NICs                     []NIC            `sql:""`
	Disks                    []Disk           `sql:""`
	Controllers              []Controller     `sql:""`
	Networks                 []Ref            `sql:""`
	Concerns                 []Concern        `sql:""`
	GuestNetworks            []GuestNetwork   `sql:""`
	GuestDisks               []DiskMountPoint `sql:""`
	GuestIpStacks            []GuestIpStack   `sql:""`
	SecureBoot               bool             `sql:""`
	DiskEnableUuid           bool             `sql:""`
	NestedHVEnabled          bool             `sql:""`
}

// Determine if current revision has been validated.
func (m *VM) Validated() bool {
	return m.RevisionValidated == m.Revision
}

// Virtual Controller.
type Controller struct {
	Key   int32   `json:"key"`
	Bus   string  `json:"bus"`
	Disks []int32 `sql:""`
}

// Virtual Disk.
type Disk struct {
	Key                   int32  `json:"key"`
	UnitNumber            int32  `json:"unitNumber"`
	ControllerKey         int32  `json:"controllerKey"`
	File                  string `json:"file"`
	Datastore             Ref    `json:"datastore"`
	Capacity              int64  `json:"capacity"`
	Shared                bool   `json:"shared"`
	RDM                   bool   `json:"rdm"`
	Bus                   string `json:"bus"`
	Mode                  string `json:"mode,omitempty"`
	Serial                string `json:"serial,omitempty"`
	WinDriveLetter        string `json:"winDriveLetter,omitempty"`
	ChangeTrackingEnabled bool   `json:"changeTrackingEnabled"`
	ParentFile            string `json:"parent"`
}

// Virtual Device.
type Device struct {
	Kind string `json:"kind"`
}

// Virtual ethernet card.
type NIC struct {
	Network   Ref    `json:"network"`
	MAC       string `json:"mac"`
	Index     int    `json:"order"`
	DeviceKey int32  `json:"deviceKey"`
}

// Guest network.
type GuestNetwork struct {
	Device         string   `json:"device"`
	DeviceConfigId int32    `json:"deviceConfigId"`
	MAC            string   `json:"mac"`
	IP             string   `json:"ip"`
	Origin         string   `json:"origin"`
	PrefixLength   int32    `json:"prefix"`
	DNS            []string `json:"dns"`
	Network        string   `json:"network"`
}

// Guest ipStack
type GuestIpStack struct {
	Device       string   `json:"device"`
	Gateway      string   `json:"gateway"`
	Network      string   `json:"network"`
	PrefixLength int32    `json:"prefix"`
	DNS          []string `json:"dns"`
}

// Guest disk.
type DiskMountPoint struct {
	// The key of the VirtualDevice.
	//
	// `VirtualDevice.key`
	Key int32 `xml:"key" json:"key"`

	// Name of the virtual disk in the guest operating system.
	//
	// For example: C:\\ ( in linux it can by a path like /home ).
	DiskPath string `xml:"diskPath,omitempty" json:"diskPath,omitempty"`
	// Total capacity of the disk, in bytes.
	//
	// This is part of the virtual machine configuration.
	Capacity int64 `xml:"capacity,omitempty" json:"capacity,omitempty"`
	// Free space on the disk, in bytes.
	//
	// This is retrieved by VMware Tools.
	FreeSpace int64 `xml:"freeSpace,omitempty" json:"freeSpace,omitempty"`
	// Filesystem type, if known.
	//
	// For example NTFS or ext3.
	FilesystemType string `xml:"filesystemType,omitempty" json:"filesystemType,omitempty"`
}
