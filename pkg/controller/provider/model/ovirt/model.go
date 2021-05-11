package ovirt

import (
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
)

//
// Errors
var NotFound = libmodel.NotFound

//
// Types
type Model = libmodel.Model

//
// Base oVirt model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Parent
	Parent Ref `sql:"d0,index(parent)"`
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

//
// An object reference.
type Ref struct {
	// The kind (type) of the referenced.
	Kind string `json:"kind"`
	// The ID of object referenced.
	ID string `json:"id"`
}

//
// Get referenced model.
func (r *Ref) Get(db libmodel.DB) (model Model, err error) {
	base := Base{
		ID: r.ID,
	}
	switch r.Kind {
	case DataCenterKind:
		model = &DataCenter{Base: base}
	case ClusterKind:
		model = &Cluster{Base: base}
	case HostKind:
		model = &Host{Base: base}
	case VmKind:
		model = &VM{Base: base}
	case NetKind:
		model = &Network{Base: base}
	case StorageKind:
		model = &StorageDomain{Base: base}
	default:
		err = InvalidRefError{*r}
	}
	if model != nil {
		err = db.Get(model)
	}

	return
}

type DataCenter struct {
	Base
}

type Cluster struct {
	Base
}

type Network struct {
	Base
	VLan         Ref      `sql:""`
	Usages       []string `sql:""`
	VNICProfiles []Ref    `sql:""`
}

type VNICProfile struct {
	Base
	QoS Ref `sql:""`
}

type StorageDomain struct {
	Base
	Type    string `sql:""`
	Storage struct {
		Type string
	} `sql:""`
	Available int64 `sql:""`
	Used      int64 `sql:""`
}

type Host struct {
	Base
}

type VM struct {
	Base
}
