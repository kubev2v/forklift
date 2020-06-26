package vmware

import (
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"github.com/vmware/govmomi/vim25/types"
	"strconv"
)

//
// Model adapter.
// Each adapter provides provider-specific management of a model.
type Adapter interface {
	// The adapter model.
	Model() model.Model
	// Apply the update to the model.
	With(types.ObjectUpdate)
}

//
// Base adapter.
type Base struct {
}

//
// Apply the update to the model `Base`.
func (v *Base) With(m *model.Base, u types.ObjectUpdate) {
	object := model.Object{}
	if m.Object != "" {
		object = m.DecodeObject()
	}
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case "name":
				if s, cast := p.Val.(string); cast {
					m.Name = s
				}
			case "parent":
				ref := vRef{}
				ref.With(p.Val)
				m.EncodeParent(ref.Ref)
			}
			object[p.Name] = p.Val
		}
	}

	m.EncodeObject(object)
}

//
// Ref adapter.
type vRef struct {
	// The adapter model.
	model.Ref
}

//
// Apply the update to the model.
func (v *vRef) With(ref types.AnyType) {
	if r, cast := ref.(types.ManagedObjectReference); cast {
		v.ID = r.Value
		switch r.Type {
		case Cluster:
			v.Kind = model.ClusterKind
		case Host:
			v.Kind = model.HostKind
		case VirtualMachine:
			v.Kind = model.VmKind
		default:
			v.Kind = r.Type
		}
	}
}

//
// RefList adapter.
type vRefList struct {
	// The adapter model.
	list model.RefList
}

//
// Apply the update to the model.
func (v *vRefList) With(ref types.AnyType) {
	if a, cast := ref.(types.ArrayOfManagedObjectReference); cast {
		list := a.ManagedObjectReference
		for _, r := range list {
			ref := vRef{}
			ref.With(r)
			v.list = append(
				v.list,
				model.Ref{
					Kind: ref.Kind,
					ID:   ref.ID,
				})
		}
	}
}

//
// Encode the enclosed list.
func (v *vRefList) Encode() string {
	return v.list.Encode()
}

//
// Folder model adapter.
type FolderAdapter struct {
	Base
	// The adapter model.
	model model.Folder
}

//
// Apply the update to the model.
func (v *FolderAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case ChildEntity:
				list := vRefList{}
				list.With(p.Val)
				v.model.Children = list.Encode()
			}
		}
	}
}

//
// The new model.
func (v *FolderAdapter) Model() model.Model {
	return &v.model
}

//
// Datacenter model adapter.
type DatacenterAdapter struct {
	Base
	// The adapter model.
	model model.Datacenter
}

//
// The adapter model.
func (v *DatacenterAdapter) Model() model.Model {
	return &v.model
}

//
// Apply the update to the model.
func (v *DatacenterAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case VmFolder:
				ref := vRef{}
				ref.With(p.Val)
				v.model.VM = ref.Encode()
			case HostFolder:
				ref := vRef{}
				ref.With(p.Val)
				v.model.Cluster = ref.Encode()
			case NetFolder:
				ref := vRef{}
				ref.With(p.Val)
				v.model.Network = ref.Encode()
			case DsFolder:
				ref := vRef{}
				ref.With(p.Val)
				v.model.Datastore = ref.Encode()
			}
		}
	}
}

//
// Cluster model adapter.
type ClusterAdapter struct {
	Base
	// The adapter model.
	model model.Cluster
}

//
// The adapter model.
func (v *ClusterAdapter) Model() model.Model {
	return &v.model
}

func (v *ClusterAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case "host":
				refList := vRefList{}
				refList.With(p.Val)
				v.model.Host = refList.Encode()
			}
		}
	}
}

//
// Host model adapter.
type HostAdapter struct {
	Base
	// The adapter model.
	model model.Host
}

//
// The adapter model.
func (v *HostAdapter) Model() model.Model {
	return &v.model
}

func (v *HostAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case "summary.runtime.inMaintenanceMode":
				if b, cast := p.Val.(bool); cast {
					v.model.Maintenance = strconv.FormatBool(b)
				}
			case "vm":
				refList := vRefList{}
				refList.With(p.Val)
				v.model.VM = refList.Encode()
			}
		}
	}
}

//
// Network model adapter.
type NetworkAdapter struct {
	Base
	// The adapter model.
	model model.Network
}

//
// The adapter model.
func (v *NetworkAdapter) Model() model.Model {
	return &v.model
}

//
// Apply the update to the model.
func (v *NetworkAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case "tag":
				if s, cast := p.Val.(string); cast {
					v.model.Tag = s
				}
			}
		}
	}
}

//
// Datastore model adapter.
type DatastoreAdapter struct {
	Base
	// The adapter model.
	model model.Datastore
}

//
// The adapter model.
func (v *DatastoreAdapter) Model() model.Model {
	return &v.model
}

//
// Apply the update to the model.
func (v *DatastoreAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case "summary.type":
				if s, cast := p.Val.(string); cast {
					v.model.Type = s
				}
			case "summary.capacity":
				if n, cast := p.Val.(int64); cast {
					v.model.Capacity = n
				}
			case "summary.freeSpace":
				if n, cast := p.Val.(int64); cast {
					v.model.Free = n
				}
			case "summary.maintenanceMode":
				if s, cast := p.Val.(string); cast {
					v.model.Maintenance = s
				}
			}
		}
	}
}

//
// VM model adapter.
type VmAdapter struct {
	Base
	// The adapter model.
	model model.VM
}

//
// The adapter model.
func (v *VmAdapter) Model() model.Model {
	return &v.model
}

//
// Apply the update to the model.
func (v *VmAdapter) With(u types.ObjectUpdate) {
	v.Base.With(&v.model.Base, u)
}
