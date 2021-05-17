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
	DataCenter string `sql:"d0,index(dataCenter)"`
}

type Network struct {
	Base
	DataCenter   string   `sql:"d0,index(dataCenter)"`
	VLan         Ref      `sql:""`
	Usages       []string `sql:""`
	VNICProfiles []Ref    `sql:""`
}

type VNICProfile struct {
	Base
	DataCenter string `sql:"d0,index(dataCenter)"`
	QoS        Ref    `sql:""`
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
	Cluster string `sql:"d0,index(cluster)"`
}

type VM struct {
	Base
	Cluster string `sql:"d0,index(cluster)"`
	Host    string `sql:"d0,index(host)"`
}
