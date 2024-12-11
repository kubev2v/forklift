package vsphere

import (
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
)

// Variants.
const (
	// Network.
	NetStandard    = "Standard"
	NetDvPortGroup = "DvPortGroup"
	OpaqueNetwork  = "OpaqueNetwork"
	NetDvSwitch    = "DvSwitch"
	// Cluster.
	ComputeResource = "ComputeResource"
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
	Cluster            string      `sql:"d0,index(cluster)"`
	Status             string      `sql:""`
	InMaintenanceMode  bool        `sql:""`
	ManagementServerIp string      `sql:""`
	Thumbprint         string      `sql:""`
	Timezone           string      `sql:""`
	CpuSockets         int16       `sql:""`
	CpuCores           int16       `sql:""`
	ProductName        string      `sql:""`
	ProductVersion     string      `sql:""`
	Network            HostNetwork `sql:""`
	Networks           []Ref       `sql:""`
	Datastores         []Ref       `sql:""`
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
	Type            string `sql:""`
	Capacity        int64  `sql:""`
	Free            int64  `sql:""`
	MaintenanceMode string `sql:""`
}

type VM struct {
	Base
	Folder                string         `sql:"d0,index(folder)"`
	Host                  string         `sql:"d0,index(host)"`
	RevisionValidated     int64          `sql:"d0,index(revisionValidated)"`
	PolicyVersion         int            `sql:"d0,index(policyVersion)"`
	UUID                  string         `sql:""`
	Firmware              string         `sql:""`
	PowerState            string         `sql:""`
	ConnectionState       string         `sql:""`
	CpuAffinity           []int32        `sql:""`
	CpuHotAddEnabled      bool           `sql:""`
	CpuHotRemoveEnabled   bool           `sql:""`
	MemoryHotAddEnabled   bool           `sql:""`
	FaultToleranceEnabled bool           `sql:""`
	CpuCount              int32          `sql:""`
	CoresPerSocket        int32          `sql:""`
	MemoryMB              int32          `sql:""`
	GuestName             string         `sql:""`
	GuestID               string         `sql:""`
	BalloonedMemory       int32          `sql:""`
	IpAddress             string         `sql:""`
	NumaNodeAffinity      []string       `sql:""`
	StorageUsed           int64          `sql:""`
	Snapshot              Ref            `sql:""`
	IsTemplate            bool           `sql:""`
	ChangeTrackingEnabled bool           `sql:""`
	TpmEnabled            bool           `sql:""`
	Devices               []Device       `sql:""`
	NICs                  []NIC          `sql:""`
	Disks                 []Disk         `sql:""`
	Networks              []Ref          `sql:""`
	Concerns              []Concern      `sql:""`
	GuestNetworks         []GuestNetwork `sql:""`
	GuestIpStacks         []GuestIpStack `sql:""`
	SecureBoot            bool           `sql:""`
}

// Determine if current revision has been validated.
func (m *VM) Validated() bool {
	return m.RevisionValidated == m.Revision
}

// Virtual Disk.
type Disk struct {
	Key       int32  `json:"key"`
	File      string `json:"file"`
	Datastore Ref    `json:"datastore"`
	Capacity  int64  `json:"capacity"`
	Shared    bool   `json:"shared"`
	RDM       bool   `json:"rdm"`
	Mode      string `json:"mode,omitempty"`
}

// Virtual Device.
type Device struct {
	Kind string `json:"kind"`
}

// Virtual ethernet card.
type NIC struct {
	Network Ref    `json:"network"`
	MAC     string `json:"mac"`
}

// Guest network.
type GuestNetwork struct {
	MAC          string   `json:"mac"`
	IP           string   `json:"ip"`
	Origin       string   `json:"origin"`
	PrefixLength int32    `json:"prefix"`
	DNS          []string `json:"dns"`
}

// Guest ipStack
type GuestIpStack struct {
	Gateway string   `json:"gateway"`
	DNS     []string `json:"dns"`
}
