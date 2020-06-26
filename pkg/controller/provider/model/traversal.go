package model

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libref "github.com/konveyor/controller/pkg/ref"
)

//
// Kinds
var (
	FolderKind     = libref.ToKind(Folder{})
	DatacenterKind = libref.ToKind(Datacenter{})
	ClusterKind    = libref.ToKind(Cluster{})
	HostKind       = libref.ToKind(Host{})
	NetKind        = libref.ToKind(Network{})
	DsKind         = libref.ToKind(Datastore{})
	VmKind         = libref.ToKind(VM{})
)

//
// Datacenter traversal.
type DatacenterTraversal struct {
	// DB connection.
	DB libmodel.DB
	// Traversal root.
	Root *Datacenter
}

//
// Traverse the datacenter and build a list
// of contained VMs.
func (r *DatacenterTraversal) VmList() ([]*VM, error) {
	list := []*VM{}
	path := []string{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case FolderKind:
			m := &Folder{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.Children)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case VmKind:
			m := &VM{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	root := Ref{}
	root.With(r.Root.VM)
	err := drill(root)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return list, nil
}

//
// Traverse the datacenter and build a list
// of contained clusters.
func (r *DatacenterTraversal) ClusterList() ([]*Cluster, error) {
	list := []*Cluster{}
	path := []string{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case FolderKind:
			m := &Folder{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.Children)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case ClusterKind:
			m := &Cluster{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	root := Ref{}
	root.With(r.Root.Cluster)
	err := drill(root)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return list, nil
}

//
// Traverse the datacenter and build a list
// of contained hosts.
func (r *DatacenterTraversal) HostList() ([]*Host, error) {
	list := []*Host{}
	path := []string{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case FolderKind:
			m := &Folder{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.Children)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case ClusterKind:
			m := &Cluster{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.Host)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case HostKind:
			m := &Host{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	root := Ref{}
	root.With(r.Root.Cluster)
	err := drill(root)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return list, nil
}

//
// Traverse the datacenter and build a list
// of contained networks.
func (r *DatacenterTraversal) NetList() ([]*Network, error) {
	list := []*Network{}
	path := []string{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case FolderKind:
			m := &Folder{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.Children)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case NetKind:
			m := &Network{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	root := Ref{}
	root.With(r.Root.Network)
	err := drill(root)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return list, nil
}

//
// Traverse the datacenter and build a list
// of contained datastores.
func (r *DatacenterTraversal) DsList() ([]*Datastore, error) {
	list := []*Datastore{}
	path := []string{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case FolderKind:
			m := &Folder{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.Children)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case DsKind:
			m := &Datastore{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	root := Ref{}
	root.With(r.Root.Network)
	err := drill(root)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return list, nil
}

//
// Cluster traversal.
type ClusterTraversal struct {
	// DB connection.
	DB libmodel.DB
	// Traversal root.
	Root *Cluster
}

//
// Traverse the datacenter and build a list
// of contained VMs.
func (r *ClusterTraversal) VmList() ([]*VM, error) {
	list := []*VM{}
	path := []string{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case HostKind:
			m := &Host{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			path = append(path, m.Name)
			refList := RefList{}
			refList.With(m.VM)
			for _, ref := range refList {
				err := drill(ref)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		case VmKind:
			m := &VM{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	hosts := RefList{}
	hosts.With(r.Root.Host)
	for _, ref := range hosts {
		err := drill(ref)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
	}

	return list, nil
}

//
// Traverse the cluster and build a list
// of contained VMs.
func (r *ClusterTraversal) HostList() ([]*Host, error) {
	list := []*Host{}
	var drill func(folder Ref) error
	drill = func(folder Ref) error {
		switch folder.Kind {
		case HostKind:
			m := &Host{
				Base: Base{
					ID: folder.ID,
				},
			}
			err := r.DB.Get(m)
			if err != nil {
				return liberr.Wrap(err)
			}
			list = append(list, m)
		}

		return nil
	}
	hosts := RefList{}
	hosts.With(r.Root.Host)
	for _, ref := range hosts {
		err := drill(ref)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
	}

	return list, nil
}
