package ovirt

import (
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
)

//
// Errors
var NotFound = libmodel.NotFound

type InvalidRefError = base.InvalidRefError

//
// Types
type Model = base.Model
type ListOptions = base.ListOptions
type Ref = base.Ref

//
// Base oVirt model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Name
	Name string `sql:"d0,index(name)"`
	// Revision
	Description string `sql:"d0"`
	// Revision
	Revision int64 `sql:"d0,index(revision)"`
}

//
// Get the PK.
func (m *Base) Pk() string {
	return m.ID
}

//
// String representation.
func (m *Base) String() string {
	return m.ID
}

//
// Get labels.
func (m *Base) Labels() libmodel.Labels {
	return nil
}

func (m *Base) Equals(other libmodel.Model) bool {
	if vm, cast := other.(*Base); cast {
		return m.ID == vm.ID
	}

	return false
}

//
// Updated.
// Increment revision. Should ONLY be called by
// the reconciler.
func (m *Base) Updated() {
	m.Revision++
}

type DataCenter struct {
	Base
}

type Cluster struct {
	Base
	DataCenter    string `sql:"d0,index(dataCenter)"`
	HaReservation bool   `sql:""`
}

type Network struct {
	Base
	DataCenter string   `sql:"d0,index(dataCenter)"`
	VLan       string   `sql:""`
	Usages     []string `sql:""`
	Profiles   []string `sql:""`
}

type NICProfile struct {
	Base
	Network string `sql:"d0,index(network)"`
	QoS     string `sql:""`
}

type StorageDomain struct {
	Base
	DataCenter string `sql:"d0,index(dataCenter)"`
	Type       string `sql:""`
	Storage    struct {
		Type string
	} `sql:""`
	Available int64 `sql:""`
	Used      int64 `sql:""`
}

type Host struct {
	Base
	Cluster        string   `sql:"d0,index(cluster)"`
	ProductName    string   `sql:""`
	ProductVersion string   `sql:""`
	InMaintenance  bool     `sql:""`
	KsmEnabled     bool     `sql:""`
	Thumbprint     string   `sql:""`
	CpuSockets     int16    `sql:""`
	CpuCores       int16    `sql:""`
	Networks       []string `sql:""`
}

type VM struct {
	Base
	Cluster        string           `sql:"d0,index(cluster)"`
	Host           string           `sql:"d0,index(host)"`
	GuestName      string           `sql:""`
	CpuSockets     int16            `sql:""`
	CpuCores       int16            `sql:""`
	Memory         int64            `sql:""`
	BIOS           string           `sql:""`
	Display        string           `sql:""`
	CpuAffinity    []CpuPinning     `sql:""`
	DiskAttachments []DiskAttachment `sql:""`
	NICs            []NIC            `sql:""`
}

type DiskAttachment struct {
	ID string `json:"id"`
	Interface string `json:"interface"`
	Disk      string `json:"disk"`
}

type NIC struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Interface string `json:"interface"`
	Profile   string `json:"profile"`
}

type CpuPinning struct {
	Set int32 `json:"set"`
	Cpu int32 `json:"cpu"`
}

type Disk struct {
	Base
	Shared        bool   `sql:""`
	StorageDomain string `sql:""`
}
