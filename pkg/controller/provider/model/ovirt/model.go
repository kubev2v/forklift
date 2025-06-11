package ovirt

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

// Base oVirt model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Name
	Name string `sql:"d0,index(name)"`
	// Revision
	Description string `sql:"d0"`
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

type DataCenter struct {
	Base
}

type Cluster struct {
	Base
	DataCenter    string  `sql:"d0,fk(dataCenter +cascade)"`
	HaReservation bool    `sql:""`
	KsmEnabled    bool    `sql:""`
	BiosType      string  `sql:""`
	CPU           CPU     `sql:""`
	Version       Version `sql:""`
}

type ServerCpu struct {
	Base
	DataCenter        string            `sql:"d0,fk(dataCenter +cascade)"`
	SystemOptionValue SystemOptionValue `sql:""`
}

type SystemOptionValue struct {
	Value   string `json:"value"`
	Version string `json:"version"`
}

type CPU struct {
	Architecture string `json:"architecture"`
	Type         string `json:"type"`
}

type Version struct {
	Minor string `json:"minor"`
	Major string `json:"major"`
}

type Network struct {
	Base
	DataCenter string   `sql:"d0,fk(dataCenter +cascade)"`
	VLan       string   `sql:""`
	Usages     []string `sql:""`
	Profiles   []string `sql:""`
}

type NICProfile struct {
	Base
	Network       string     `sql:"d0,fk(network +cascade)"`
	PortMirroring bool       `sql:""`
	NetworkFilter string     `sql:""`
	QoS           string     `sql:""`
	Properties    []Property `sql:""`
	PassThrough   bool       `sql:""`
}

type DiskProfile struct {
	Base
	StorageDomain string `sql:"d0,fk(storageDomain +cascade)"`
	QoS           string `sql:""`
}

type StorageDomain struct {
	Base
	DataCenter string `sql:"d0,fk(dataCenter +cascade)"`
	Type       string `sql:""`
	Storage    struct {
		Type string
	} `sql:""`
	Available int64 `sql:""`
	Used      int64 `sql:""`
}

type Host struct {
	Base
	Cluster            string              `sql:"d0,fk(cluster +cascade)"`
	Status             string              `sql:""`
	ProductName        string              `sql:""`
	ProductVersion     string              `sql:""`
	InMaintenance      bool                `sql:""`
	CpuSockets         int16               `sql:""`
	CpuCores           int16               `sql:""`
	NetworkAttachments []NetworkAttachment `sql:""`
	NICs               []HostNIC           `sql:""`
}

type NetworkAttachment struct {
	ID      string `json:"id"`
	Network string `json:"network"`
}

type HostNIC struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	LinkSpeed int64  `json:"linkSpeed"`
	MTU       int64  `json:"mtu"`
	VLan      string `json:"vlan"`
}

type VM struct {
	Base
	Cluster                     string           `sql:"d0,fk(cluster +cascade)"`
	Host                        string           `sql:"d0,index(host)"`
	RevisionValidated           int64            `sql:"d0,index(revisionValidated)" eq:"-"`
	PolicyVersion               int              `sql:"d0,index(policyVersion)" eq:"-"`
	GuestName                   string           `sql:""`
	CpuSockets                  int16            `sql:""`
	CpuCores                    int16            `sql:""`
	CpuThreads                  int16            `sql:""`
	CpuAffinity                 []CpuPinning     `sql:""`
	CpuPinningPolicy            string           `sql:""`
	CpuShares                   int16            `sql:""`
	Memory                      int64            `sql:""`
	BalloonedMemory             bool             `sql:""`
	BIOS                        string           `sql:""`
	Display                     string           `sql:""`
	IOThreads                   int16            `sql:""`
	StorageErrorResumeBehaviour string           `sql:""`
	HaEnabled                   bool             `sql:""`
	UsbEnabled                  bool             `sql:""`
	BootMenuEnabled             bool             `sql:""`
	PlacementPolicyAffinity     string           `sql:""`
	Timezone                    string           `sql:""`
	Status                      string           `sql:""`
	Stateless                   string           `sql:""`
	SerialNumber                string           `sql:""`
	HasIllegalImages            bool             `sql:""`
	NumaNodeAffinity            []string         `sql:""`
	LeaseStorageDomain          string           `sql:""`
	DiskAttachments             []DiskAttachment `sql:""`
	NICs                        []NIC            `sql:""`
	HostDevices                 []HostDevice     `sql:""`
	CDROMs                      []CDROM          `sql:""`
	WatchDogs                   []WatchDog       `sql:""`
	Properties                  []Property       `sql:""`
	Snapshots                   []Snapshot       `sql:""`
	Concerns                    []Concern        `sql:"" eq:"-"`
	Guest                       Guest            `sql:""`
	OSType                      string           `sql:""`
	CustomCpuModel              string           `sql:""`
}

// Determine if current revision has been validated.
func (m *VM) Validated() bool {
	return m.RevisionValidated == m.Revision
}

type Snapshot struct {
	ID            string `json:"id"`
	Description   string `json:"description"`
	Type          string `json:"type"`
	PersistMemory bool   `json:"persistMemory"`
}

type DiskAttachment struct {
	ID              string `json:"id"`
	Interface       string `json:"interface"`
	SCSIReservation bool   `json:"scsiReservation"`
	Disk            string `json:"disk"`
	Bootable        bool   `json:"bootable"`
}

type NIC struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Interface string      `json:"interface"`
	Plugged   bool        `json:"plugged"`
	IpAddress []IpAddress `json:"ipAddress"`
	Profile   string      `json:"profile"`
	MAC       string      `json:"mac"`
}

type IpAddress struct {
	Address string `json:"address"`
	Version string `json:"version"`
}

type CpuPinning struct {
	Set int32 `json:"set"`
	Cpu int32 `json:"cpu"`
}

type HostDevice struct {
	Capability string `json:"capability"`
	Product    string `json:"product"`
	Vendor     string `json:"vendor"`
}

type CDROM struct {
	ID   string `json:"id"`
	File string `json:"file,omitempty"`
}

type WatchDog struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Model  string `json:"model"`
}

type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Guest struct {
	Distribution string `json:"distribution"`
	FullVersion  string `json:"fullVersion"`
}

type Disk struct {
	Base
	Shared          bool   `sql:""`
	Profile         string `sql:"index(profile)"`
	StorageDomain   string `sql:"fk(storageDomain +cascade)"`
	Status          string `sql:""`
	ActualSize      int64  `sql:""`
	Backup          string `sql:""`
	StorageType     string `sql:""`
	ProvisionedSize int64  `sql:""`
	Lun             Lun    `sql:""`
}

type Lun struct {
	LogicalUnits struct {
		LogicalUnit []LogicalUnit `json:"logicalUnit"`
	}
}

type LogicalUnit struct {
	LunID      string `json:"lunId"`
	Address    string `json:"address"`
	Port       string `json:"port"`
	Target     string `json:"target"`
	LunMapping int32  `json:"lunMapping"`
	Size       int64  `json:"size"`
}
