package vsphere

import (
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/vmware/govmomi/vim25/types"
)

//
// Model adapter.
// Each adapter provides provider-specific management of a model.
type Adapter interface {
	// The adapter model.
	Model() model.Model
	// Apply the update to the model.
	Apply(types.ObjectUpdate)
}

//
// Base adapter.
type Base struct {
}

//
// Apply the update to the model `Base`.
func (v *Base) Apply(m *model.Base, u types.ObjectUpdate) {
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fName:
				if s, cast := p.Val.(string); cast {
					m.Name = s
				}
			case fParent:
				ref := Ref{}
				ref.With(p.Val)
				m.Parent = ref.Encode()
			}
		}
	}
}

//
// Ref.
type Ref struct {
	// A wrapped ref.
	model.Ref
}

//
// Set the ref properties.
func (v *Ref) With(ref types.AnyType) {
	if r, cast := ref.(types.ManagedObjectReference); cast {
		v.ID = r.Value
		switch r.Type {
		case Folder:
			v.Kind = model.FolderKind
		case Datacenter:
			v.Kind = model.DatacenterKind
		case Cluster:
			v.Kind = model.ClusterKind
		case Network:
			v.Kind = model.NetKind
		case Datastore:
			v.Kind = model.DsKind
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
// RefList
type RefList struct {
	// A wrapped list.
	list model.RefList
}

//
// Set the list content.
func (v *RefList) With(ref types.AnyType) {
	if a, cast := ref.(types.ArrayOfManagedObjectReference); cast {
		list := a.ManagedObjectReference
		for _, r := range list {
			v.Append(r)
		}
	}
}

//
// Append reference.
func (v *RefList) Append(r types.ManagedObjectReference) {
	ref := Ref{}
	ref.With(r)
	v.list = append(
		v.list,
		model.Ref{
			Kind: ref.Kind,
			ID:   ref.ID,
		})
}

//
// Encode the enclosed list.
func (v *RefList) Encode() string {
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
func (v *FolderAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fChildEntity:
				list := RefList{}
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
func (v *DatacenterAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fVmFolder:
				ref := Ref{}
				ref.With(p.Val)
				v.model.Vms = ref.Encode()
			case fHostFolder:
				ref := Ref{}
				ref.With(p.Val)
				v.model.Clusters = ref.Encode()
			case fNetFolder:
				ref := Ref{}
				ref.With(p.Val)
				v.model.Networks = ref.Encode()
			case fDsFolder:
				ref := Ref{}
				ref.With(p.Val)
				v.model.Datastores = ref.Encode()
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

func (v *ClusterAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fHost:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Hosts = refList.Encode()
			case fNetwork:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Networks = refList.Encode()
			case fDatastore:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Datastores = refList.Encode()
			case fDasEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.DasEnabled = b
				}
			case fDasVmCfg:
				refList := RefList{}
				if list, cast := p.Val.([]types.ClusterDasVmConfigInfo); cast {
					for _, v := range list {
						refList.Append(v.Key)
					}
				}
				v.model.DasVms = refList.Encode()
			case fDrsEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.DrsEnabled = b
				}
			case fDrsVmCfg:
				refList := RefList{}
				if list, cast := p.Val.([]types.ClusterDrsVmConfigInfo); cast {
					for _, v := range list {
						refList.Append(v.Key)
					}
				}
				v.model.DrsVms = refList.Encode()
			case fDrsVmBehavior:
				if b, cast := p.Val.(types.DrsBehavior); cast {
					v.model.DrsBehavior = string(b)
				}
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

func (v *HostAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fInMaintMode:
				if b, cast := p.Val.(bool); cast {
					v.model.InMaintenanceMode = b
				}
			case fVm:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Vms = refList.Encode()
			case fProductName:
				if s, cast := p.Val.(string); cast {
					v.model.ProductName = s
				}
			case fProductVersion:
				if s, cast := p.Val.(string); cast {
					v.model.ProductVersion = s
				}
			case fThumbprint:
				if s, cast := p.Val.(string); cast {
					v.model.Thumbprint = s
				}
			case fNetwork:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Networks = refList.Encode()
			case fDatastore:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Datastores = refList.Encode()
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
func (v *NetworkAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fTag:
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
func (v *DatastoreAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fDsType:
				if s, cast := p.Val.(string); cast {
					v.model.Type = s
				}
			case fCapacity:
				if n, cast := p.Val.(int64); cast {
					v.model.Capacity = n
				}
			case fFreeSpace:
				if n, cast := p.Val.(int64); cast {
					v.model.Free = n
				}
			case fDsMaintMode:
				if s, cast := p.Val.(string); cast {
					v.model.MaintenanceMode = s
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
func (v *VmAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fUUID:
				if s, cast := p.Val.(string); cast {
					v.model.UUID = s
				}
			case fFirmware:
				if s, cast := p.Val.(string); cast {
					v.model.Firmware = s
				}
			case fCpuAffinity:
				if a, cast := p.Val.(types.VirtualMachineAffinityInfo); cast {
					v.model.EncodeCpuAffinity(a.AffinitySet)
				}
			case fCpuHotAddEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.CpuHotAddEnabled = b
				}
			case fCpuHotRemoveEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.CpuHotRemoveEnabled = b
				}
			case fMemoryHotAddEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.MemoryHotAddEnabled = b
				}
			case fNumCpu:
				if n, cast := p.Val.(int32); cast {
					v.model.CpuCount = n
				}
			case fNumCoresPerSocket:
				if n, cast := p.Val.(int32); cast {
					v.model.CoresPerSocket = n
				}
			case fMemorySize:
				if n, cast := p.Val.(int32); cast {
					v.model.MemoryMB = n
				}
			case fGuestName:
				if s, cast := p.Val.(string); cast {
					v.model.GuestName = s
				}
			case fBalloonedMemory:
				if n, cast := p.Val.(int32); cast {
					v.model.BalloonedMemory = n
				}
			case fRuntimeHost:
				ref := Ref{}
				ref.With(p.Val)
				v.model.Host = ref.Encode()
			case fVmIpAddress:
				if s, cast := p.Val.(string); cast {
					v.model.IpAddress = s
				}
			case fNetwork:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Networks = refList.Encode()
			case fDevices:
				disks := []model.Disk{}
				if devArray, cast := p.Val.(types.ArrayOfVirtualDevice); cast {
					for _, dev := range devArray.VirtualDevice {
						switch dev.(type) {
						case *types.VirtualDisk:
							disk := dev.(*types.VirtualDisk)
							backing := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
							md := model.Disk{
								File:     backing.FileName,
								Capacity: disk.CapacityInBytes,
								Datastore: model.Ref{
									Kind: model.DsKind,
									ID:   backing.Datastore.Value,
								},
							}
							disks = append(disks, md)
						}
					}
				}
				v.model.EncodeDisks(disks)
			}
		}
	}
}
