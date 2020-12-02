package vsphere

import (
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/vmware/govmomi/vim25/types"
	"sort"
	"strings"
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
		case Network,
			DVPortGroup,
			DVSwitch:
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
// RefList
type List struct {
	// A wrapped list.
	list model.List
}

//
// Encode the enclosed list.
func (v *List) Encode() string {
	return v.list.Encode()
}

//
// Set the list content.
func (v *List) With(in interface{}) {
	v.list = model.List{}
	switch in.(type) {
	case []int:
		list := in.([]int)
		for _, n := range list {
			v.list = append(v.list, n)
		}
	case []int8:
		list := in.([]int8)
		for _, n := range list {
			v.list = append(v.list, n)
		}
	case []int16:
		list := in.([]int16)
		for _, n := range list {
			v.list = append(v.list, n)
		}
	case []int32:
		list := in.([]int32)
		for _, n := range list {
			v.list = append(v.list, n)
		}
	case []int64:
		list := in.([]int64)
		for _, n := range list {
			v.list = append(v.list, n)
		}
	case []string:
		list := in.([]string)
		for _, s := range list {
			v.list = append(v.list, s)
		}
	}
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
			case fThumbprint:
				if s, cast := p.Val.(string); cast {
					v.model.Thumbprint = s
				}
			case fCpuSockets:
				if b, cast := p.Val.(int16); cast {
					v.model.CpuSockets = b
				}
			case fCpuCores:
				if b, cast := p.Val.(int16); cast {
					v.model.CpuCores = b
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
			case fNetwork:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Networks = refList.Encode()
			case fDatastore:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Datastores = refList.Encode()
			case fVSwitch:
				if array, cast := p.Val.(types.ArrayOfHostVirtualSwitch); cast {
					network := v.model.DecodeNetwork()
					for _, vSwitch := range array.HostVirtualSwitch {
						network.Switches = append(
							network.Switches,
							model.Switch{
								Key:        vSwitch.Key,
								Name:       vSwitch.Name,
								PortGroups: vSwitch.Portgroup,
								PNICs:      vSwitch.Pnic,
							})
					}
					v.model.EncodeNetwork(network)
				}
			case fPortGroup:
				if array, cast := p.Val.(types.ArrayOfHostPortGroup); cast {
					network := v.model.DecodeNetwork()
					for _, portGroup := range array.HostPortGroup {
						network.PortGroups = append(
							network.PortGroups,
							model.PortGroup{
								Key:    portGroup.Key,
								Name:   portGroup.Spec.Name,
								Switch: portGroup.Vswitch,
							})
					}
					v.model.EncodeNetwork(network)
				}
			case fPNIC:
				if array, cast := p.Val.(types.ArrayOfPhysicalNic); cast {
					network := v.model.DecodeNetwork()
					for _, nic := range array.PhysicalNic {
						linkSpeed := int32(0)
						if nic.LinkSpeed != nil {
							linkSpeed = nic.LinkSpeed.SpeedMb
						}
						network.PNICs = append(
							network.PNICs,
							model.PNIC{
								Key:       nic.Key,
								LinkSpeed: linkSpeed,
							})
					}
					sort.Slice(
						network.PNICs,
						func(i, j int) bool {
							return network.PNICs[i].LinkSpeed > network.PNICs[j].LinkSpeed
						})
					v.model.EncodeNetwork(network)
				}
			case fVNIC:
				if array, cast := p.Val.(types.ArrayOfHostVirtualNic); cast {
					network := v.model.DecodeNetwork()
					for _, nic := range array.HostVirtualNic {
						dGroup := func() (key string) {
							dp := nic.Spec.DistributedVirtualPort
							if dp != nil {
								key = dp.PortgroupKey
							}
							return
						}
						network.VNICs = append(
							network.VNICs,
							model.VNIC{
								Key:        nic.Key,
								PortGroup:  nic.Portgroup,
								DPortGroup: dGroup(),
								IpAddress:  nic.Spec.Ip.IpAddress,
								MTU:        nic.Spec.Mtu,
							})
					}
					sort.Slice(
						network.VNICs,
						func(i, j int) bool {
							return network.VNICs[i].MTU > network.VNICs[j].MTU
						})
					v.model.EncodeNetwork(network)
				}
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
			case fDVSwitch:
				ref := Ref{}
				ref.With(p.Val)
				v.model.DVSwitch = ref.Encode()
			}
		}
	}
}

//
// DVSwitch model adapter.
type DVSwitchAdapter struct {
	Base
	// The adapter model.
	model model.DVSwitch
}

//
// The adapter model.
func (v *DVSwitchAdapter) Model() model.Model {
	return &v.model
}

//
// Apply the update to the model.
func (v *DVSwitchAdapter) Apply(u types.ObjectUpdate) {
	v.Base.Apply(&v.model.Base, u)
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fDVSwitchHost:
				if array, cast := p.Val.(types.ArrayOfDistributedVirtualSwitchHostMember); cast {
					v.addHost(array)
				}
			}
		}
	}
}

//
// Add hosts.
func (v *DVSwitchAdapter) addHost(array types.ArrayOfDistributedVirtualSwitchHostMember) {
	list := []model.DVSHost{}
	for _, member := range array.DistributedVirtualSwitchHostMember {
		hostRef := Ref{}
		hostRef.With(*member.Config.Host)
		if backing, cast := member.Config.Backing.(*types.DistributedVirtualSwitchHostMemberPnicBacking); cast {
			names := []string{}
			for _, pn := range backing.PnicSpec {
				names = append(names, pn.PnicDevice)
			}
			list = append(list,
				model.DVSHost{
					Host: hostRef.Encode(),
					PNIC: names,
				})
		}
	}

	v.model.EncodeHost(list)
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
					list := List{}
					list.With(a.AffinitySet)
					v.model.CpuAffinity = list.Encode()
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
			case fStorageUsed:
				if n, cast := p.Val.(int64); cast {
					v.model.StorageUsed = n
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
			case fFtInfo:
				if _, cast := p.Val.(types.FaultToleranceConfigInfo); cast {
					v.model.FaultToleranceEnabled = true
				}
			case fNetwork:
				refList := RefList{}
				refList.With(p.Val)
				v.model.Networks = refList.Encode()
			case fExtraConfig:
				if options, cast := p.Val.(types.ArrayOfOptionValue); cast {
					for _, val := range options.OptionValue {
						opt := val.GetOptionValue()
						switch opt.Key {
						case "numa.nodeAffinity":
							if s, cast := opt.Value.(string); cast {
								list := List{}
								list.With(strings.Split(s, ","))
								v.model.NumaNodeAffinity = list.Encode()
							}
						}
					}
				}
			case fDevices:
				if devArray, cast := p.Val.(types.ArrayOfVirtualDevice); cast {
					for _, dev := range devArray.VirtualDevice {
						switch dev.(type) {
						case *types.VirtualSriovEthernetCard:
							v.model.SriovSupported = true
						case *types.VirtualPCIPassthrough,
							*types.VirtualSCSIPassthrough:
							v.model.PassthroughSupported = true
						case *types.VirtualUSBController:
							v.model.UsbSupported = true
						}
					}
					v.updateDisks(&devArray)
				}
			}
		}
	}
}

//
// Update virtual disk devices.
func (v *VmAdapter) updateDisks(devArray *types.ArrayOfVirtualDevice) {
	disks := []model.Disk{}
	for _, dev := range devArray.VirtualDevice {
		switch dev.(type) {
		case *types.VirtualDisk:
			disk := dev.(*types.VirtualDisk)
			switch disk.Backing.(type) {
			case *types.VirtualDiskFlatVer1BackingInfo:
				backing := disk.Backing.(*types.VirtualDiskFlatVer1BackingInfo)
				md := model.Disk{
					File:     backing.FileName,
					Capacity: disk.CapacityInBytes,
					Datastore: model.Ref{
						Kind: model.DsKind,
						ID:   backing.Datastore.Value,
					},
				}
				disks = append(disks, md)
			case *types.VirtualDiskFlatVer2BackingInfo:
				backing := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
				md := model.Disk{
					File:     backing.FileName,
					Capacity: disk.CapacityInBytes,
					Shared:   backing.Sharing != "sharingNone",
					Datastore: model.Ref{
						Kind: model.DsKind,
						ID:   backing.Datastore.Value,
					},
				}
				disks = append(disks, md)
			case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
				backing := disk.Backing.(*types.VirtualDiskRawDiskMappingVer1BackingInfo)
				md := model.Disk{
					File:     backing.FileName,
					Capacity: disk.CapacityInBytes,
					Shared:   backing.Sharing != "sharingNone",
					Datastore: model.Ref{
						Kind: model.DsKind,
						ID:   backing.Datastore.Value,
					},
					RDM: true,
				}
				disks = append(disks, md)
			case *types.VirtualDiskRawDiskVer2BackingInfo:
				backing := disk.Backing.(*types.VirtualDiskRawDiskVer2BackingInfo)
				md := model.Disk{
					Capacity: disk.CapacityInBytes,
					Shared:   backing.Sharing != "sharingNone",
					RDM:      true,
				}
				disks = append(disks, md)
			}
		}
	}

	v.model.EncodeDisks(disks)
}
