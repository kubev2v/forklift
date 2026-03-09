package hyperv

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Power state constants.
const (
	PowerStateOn      = "On"
	PowerStateOff     = "Off"
	PowerStatePaused  = "Paused"
	PowerStateUnknown = "Unknown"
)

const (
	OriginManual = "Manual"
	OriginDhcp   = "Dhcp"
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
	GetName() string
}

// Base HyperV model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk" json:"id"`
	// Variant
	Variant string `sql:"d0,index(variant)" json:"variant,omitempty"`
	// Name
	Name string `sql:"d0,index(name)" json:"name"`
	// Revision
	Revision int64 `sql:"incremented,d0,index(revision)" json:"revision"`
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

// Name.
func (m *Base) GetName() string {
	return m.Name
}

type Network struct {
	Base
	UUID string `sql:"d0,index(uuid)"`
	// Human-readable switch name
	SwitchName string `sql:""`
	// Switch type: External, Internal, Private
	SwitchType  string `sql:""`
	Description string `sql:""`
}

type Storage struct {
	Base
	// Storage type (e.g., "SMB")
	Type string `sql:""`
	// SMB share path (e.g., //server/share)
	Path string `sql:""`
	// Total capacity in bytes
	Capacity int64 `sql:""`
	// Free space in bytes
	Free int64 `sql:""`
}

type VM struct {
	Base
	UUID              string         `sql:"d0,index(uuid)"`
	PowerState        string         `sql:"d0,index(powerState)"`
	CpuCount          int32          `sql:""`
	MemoryMB          int32          `sql:""`
	Firmware          string         `sql:""`
	GuestOS           string         `sql:""`
	TpmEnabled        bool           `sql:""`
	SecureBoot        bool           `sql:""`
	HasCheckpoint     bool           `sql:""`
	RevisionValidated int64          `sql:"d0,index(revisionValidated)"`
	PolicyVersion     int            `sql:"d0,index(policyVersion)"`
	Disks             []Disk         `sql:""`
	NICs              []NIC          `sql:""`
	Networks          []Ref          `sql:""`
	GuestNetworks     []GuestNetwork `sql:""`
	Concerns          []Concern      `sql:""`
}

// Determine if current revision has been validated.
func (m *VM) Validated() bool {
	return m.RevisionValidated == m.Revision
}

type Disk struct {
	Base
	WindowsPath string `sql:"" json:"windowsPath,omitempty"`
	// Mapped path on SMB mount (e.g., /hyperv/disk.vhdx)
	SMBPath    string `sql:"" json:"smbPath,omitempty"`
	Capacity   int64  `sql:"" json:"capacity"`
	Format     string `sql:"" json:"format,omitempty"`
	RCTEnabled bool   `sql:"" json:"rctEnabled"` // Resilient Change Tracking for warm migration
	Datastore  Ref    `sql:"" json:"datastore"`
}

type NIC struct {
	Name        string `json:"name"`
	MAC         string `json:"mac"`
	DeviceIndex int    `json:"deviceIndex"`
	Network     Ref    `json:"network"`
	NetworkName string `json:"networkName,omitempty"`
}

type GuestNetwork struct {
	MAC          string   `json:"mac"`
	IP           string   `json:"ip"`
	DeviceIndex  int      `json:"deviceIndex"`
	Origin       string   `json:"origin"`
	PrefixLength int32    `json:"prefix"`
	DNS          []string `json:"dns"`
	Gateway      string   `json:"gateway"`
}

// Device represents a generic virtual device.
type Device struct {
	Kind string `sql:""`
}
