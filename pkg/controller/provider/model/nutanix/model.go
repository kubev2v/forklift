package nutanix

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Errors
var NotFound = libmodel.NotFound

type InvalidRefError = base.InvalidRefError

const (
	MaxDetail = base.MaxDetail
)

// Types
type Model = base.Model
type ListOptions = base.ListOptions
type Concern = base.Concern
type Ref = base.Ref

// Base Nutanix model.
type Base struct {
	// Managed object ID (UUID).
	ID string `sql:"pk"`
	// Name
	Name string `sql:"d0,index(name)"`
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
	if b, cast := other.(*Base); cast {
		return m.ID == b.ID
	}
	return false
}

// Populate PK using the ref.
func (m *Base) WithRef(ref Ref) {
	m.ID = ref.ID
}

// Name.
func (m *Base) GetName() string {
	return m.Name
}

// Cluster represents a Nutanix cluster
type Cluster struct {
	Base
	ClusterUUID   string `sql:""`
	Version       string `sql:""`
	BuildVersion  string `sql:""`
	Timezone      string `sql:""`
	ClusterArch   string `sql:""`
	OperationMode string `sql:""`
	ExternalIP    string `sql:""`
	NumNodes      int    `sql:""`
	VMCount       int64  `sql:""`
	TotalCapacity int64  `sql:""`
	UsedCapacity  int64  `sql:""`
}

// Host represents a Nutanix AHV hypervisor node
type Host struct {
	Base
	Cluster           string `sql:"d0,index(cluster)"`
	HostUUID          string `sql:""`
	SerialNumber      string `sql:""`
	BlockModel        string `sql:""`
	HypervisorType    string `sql:""`
	NumVMs            int    `sql:""`
	State             string `sql:""`
	HostType          string `sql:""`
	CPUModel          string `sql:""`
	CPUCapacityHz     int64  `sql:""`
	NumCpuSockets     int    `sql:""`
	NumCpuCores       int    `sql:""`
	NumCpuThreads     int    `sql:""`
	MemoryCapacityMiB int64  `sql:""`
	IPMIAddress       string `sql:""`
}

// Network represents a Nutanix subnet/network
type Network struct {
	Base
	Cluster        string `sql:"d0,index(cluster)"`
	NetworkUUID    string `sql:""`
	VlanID         int    `sql:""`
	SubnetType     string `sql:""`
	NetworkAddress string `sql:""`
	PrefixLength   int    `sql:""`
	DefaultGateway string `sql:""`
	DHCPServerIP   string `sql:""`
	DHCPDomainName string `sql:""`
	IPPoolRanges   string `sql:""` // Comma-separated list
}

// StorageContainer represents a Nutanix storage container
type StorageContainer struct {
	Base
	Cluster              string `sql:"d0,index(cluster)"`
	StorageContainerUUID string `sql:""`
	ReplicationFactor    int    `sql:""`
	MaxCapacityBytes     int64  `sql:""`
	UsageBytes           int64  `sql:""`
	FreeBytes            int64  `sql:""`
	CompressionEnabled   bool   `sql:""`
	OnDiskDedup          string `sql:""`
	ErasureCode          string `sql:""`
}

// VM represents a Nutanix virtual machine
type VM struct {
	Base
	Cluster             string            `sql:"d0,index(cluster)"`
	Host                string            `sql:"d0,index(host)"`
	RevisionValidated   int64             `sql:"d0,index(revisionValidated)"`
	PolicyVersion       int               `sql:"d0,index(policyVersion)"`
	UUID                string            `sql:"d0"`
	Description         string            `sql:""`
	PowerState          string            `sql:""`
	NumSockets          int               `sql:""`
	NumVcpusPerSocket   int               `sql:""`
	NumThreadsPerCore   int               `sql:""`
	MemorySizeMiB       int64             `sql:""`
	BootType            string            `sql:""` // LEGACY, UEFI, SECURE_BOOT
	BootDeviceOrder     string            `sql:""` // Comma-separated
	MachineType         string            `sql:""` // PC, Q35
	HardwareClockTZ     string            `sql:""`
	VGAConsoleEnabled   bool              `sql:""`
	HypervisorType      string            `sql:""`
	GuestOSID           string            `sql:""`
	GuestOSVersion      string            `sql:""`
	SerialPorts         []SerialPort      `sql:""`
	NICs                []NIC             `sql:""`
	Disks               []Disk            `sql:""`
	GuestToolsVersion   string            `sql:""`
	GuestToolsEnabled   bool              `sql:""`
	GuestToolsMounted   bool              `sql:""`
	GuestToolsReachable bool              `sql:""`
	Categories          map[string]string `sql:""`
	Concerns            []Concern         `sql:""`
}

// SerialPort represents a VM serial port
type SerialPort struct {
	Index       int  `json:"index"`
	IsConnected bool `json:"isConnected"`
}

// NIC represents a VM network interface
type NIC struct {
	UUID        string   `json:"uuid"`
	NicType     string   `json:"nicType"` // NORMAL_NIC, DIRECT_NIC
	MACAddress  string   `json:"macAddress"`
	Model       string   `json:"model"` // VIRTIO, E1000
	IsConnected bool     `json:"isConnected"`
	SubnetUUID  string   `json:"subnetUuid"`
	SubnetName  string   `json:"subnetName"`
	IPAddresses []string `json:"ipAddresses"`
	VlanMode    string   `json:"vlanMode"`
}

// Disk represents a VM disk
type Disk struct {
	UUID                 string `json:"uuid"`
	DeviceType           string `json:"deviceType"` // DISK, CDROM
	DiskSizeMiB          int64  `json:"diskSizeMib"`
	DiskSizeBytes        int64  `json:"diskSizeBytes"`
	StorageContainerUUID string `json:"storageContainerUuid"`
	StorageContainerName string `json:"storageContainerName"`
	AdapterType          string `json:"adapterType"` // SCSI, IDE, PCI, SATA
	DeviceIndex          int    `json:"deviceIndex"`
	SourceImageUUID      string `json:"sourceImageUuid"`
	IsCdrom              bool   `json:"isCdrom"`
	FlashMode            bool   `json:"flashMode"`
}

// Image represents a Nutanix disk image or ISO
type Image struct {
	Base
	ImageUUID    string `sql:""`
	ImageType    string `sql:""` // DISK_IMAGE, ISO_IMAGE
	SizeBytes    int64  `sql:""`
	Architecture string `sql:""`
	SourceURI    string `sql:""`
}
