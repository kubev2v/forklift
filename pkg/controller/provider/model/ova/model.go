package ova

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
type ListOptions = base.ListOptions
type Concern = base.Concern
type Ref = base.Ref

// Model.
type Model interface {
	base.Model
	GetName() string
}

// Base OVA model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Variant
	Variant string `sql:"d0,index(variant)"`
	// Name
	Name string `sql:"d0,index(name)"`
	// Revision
	Revision int64 `sql:"incremented,d0,index(revision)"`
}

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

// Determine if current revision has been validated.
func (m *VM) Validated() bool {
	return m.RevisionValidated == m.Revision
}

type Network struct {
	Base
	Description string `sql:""`
}

type VM struct {
	Base
	OvaPath               string    `sql:""`
	OvaSource             string    `sql:""`
	RevisionValidated     int64     `sql:"d0,index(revisionValidated)"`
	PolicyVersion         int       `sql:"d0,index(policyVersion)"`
	UUID                  string    `sql:""`
	Firmware              string    `sql:""`
	SecureBoot            bool      `sql:""`
	CpuAffinity           []int32   `sql:""`
	CpuHotAddEnabled      bool      `sql:""`
	CpuHotRemoveEnabled   bool      `sql:""`
	MemoryHotAddEnabled   bool      `sql:""`
	FaultToleranceEnabled bool      `sql:""`
	CpuCount              int32     `sql:""`
	CoresPerSocket        int32     `sql:""`
	MemoryMB              int32     `sql:""`
	MemoryUnits           string    `sql:""`
	CpuUnits              string    `sql:""`
	BalloonedMemory       int32     `sql:""`
	IpAddress             string    `sql:""`
	NumaNodeAffinity      []string  `sql:""`
	StorageUsed           int64     `sql:""`
	ChangeTrackingEnabled bool      `sql:""`
	Devices               []Device  `sql:""`
	NICs                  []NIC     `sql:""`
	Disks                 []Disk    `sql:""`
	Networks              []Network `sql:""`
	Concerns              []Concern `sql:""`
}

// Virtual Disk.
type Disk struct {
	Base
	FilePath                string `sql:""`
	Capacity                int64  `sql:""`
	CapacityAllocationUnits string `sql:""`
	DiskId                  string `sql:""`
	FileRef                 string `sql:""`
	Format                  string `sql:""`
	PopulatedSize           int64  `sql:""`
}

// Virtual Device.
type Device struct {
	Kind string `sql:""`
}

type Conf struct {
	Key   string `sql:""`
	Value string `sql:""`
}

// Virtual ethernet card.
type NIC struct {
	Name    string `sql:""`
	MAC     string `sql:""`
	Network string `sql:""`
	Config  []Conf `sql:""`
}

type Storage struct {
	Base
}
