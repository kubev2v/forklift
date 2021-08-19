package ovirt

import (
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"path"
)

//
// Cluster.
func (m *Cluster) Path(db libmodel.DB) (p string, err error) {
	parent := &DataCenter{
		Base: Base{ID: m.DataCenter},
	}
	err = db.Get(parent)
	if err != nil {
		return
	}
	p = path.Join(
		parent.Name,
		m.Name)

	return
}

//
// Network.
func (m *Network) Path(db libmodel.DB) (p string, err error) {
	parent := &DataCenter{
		Base: Base{ID: m.DataCenter},
	}
	err = db.Get(parent)
	if err != nil {
		return
	}

	p = path.Join(parent.Name, m.Name)

	return
}

//
// StorageDomain.
func (m *StorageDomain) Path(db libmodel.DB) (p string, err error) {
	parent := &DataCenter{
		Base: Base{ID: m.DataCenter},
	}
	err = db.Get(parent)
	if err != nil {
		return
	}

	p = path.Join(parent.Name, m.Name)

	return
}

//
// Host
func (m *Host) Path(db libmodel.DB) (p string, err error) {
	parent := &Cluster{
		Base: Base{ID: m.Cluster},
	}
	err = db.Get(parent)
	if err != nil {
		return
	}
	var root string
	root, err = parent.Path(db)
	if err != nil {
		return
	}

	p = path.Join(root, m.Name)

	return
}

//
// VM
func (m *VM) Path(db libmodel.DB) (p string, err error) {
	parent := &Cluster{
		Base: Base{ID: m.Cluster},
	}
	err = db.Get(parent)
	if err != nil {
		return
	}
	var root string
	root, err = parent.Path(db)
	if err != nil {
		return
	}

	p = path.Join(root, m.Name)

	return
}
