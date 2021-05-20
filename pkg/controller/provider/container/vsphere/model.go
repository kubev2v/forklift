package vsphere

import (
	libref "github.com/konveyor/controller/pkg/ref"
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
func (b *Base) Apply(m *model.Base, u types.ObjectUpdate) {
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fName:
				if s, cast := p.Val.(string); cast {
					m.Name = s
				}
			case fParent:
				m.Parent = b.Ref(p.Val)
			}
		}
	}
}

//
// Build ref.
func (b *Base) Ref(in types.AnyType) (ref model.Ref) {
	if r, cast := in.(types.ManagedObjectReference); cast {
		ref.ID = r.Value
		switch r.Type {
		case Folder:
			ref.Kind = model.FolderKind
		case Datacenter:
			ref.Kind = model.DatacenterKind
		case Cluster:
			ref.Kind = model.ClusterKind
		case Network,
			DVPortGroup,
			DVSwitch:
			ref.Kind = model.NetKind
		case Datastore:
			ref.Kind = model.DsKind
		case Host:
			ref.Kind = model.HostKind
		case VirtualMachine:
			ref.Kind = model.VmKind
		default:
			ref.Kind = r.Type
		}
	}

	return
}

//
// Build a []Ref.
func (b *Base) RefList(in types.AnyType) (list []model.Ref) {
	if a, cast := in.(types.ArrayOfManagedObjectReference); cast {
		for _, r := range a.ManagedObjectReference {
			list = append(list, b.Ref(r))
		}
	}

	return
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
			case fParent:
				ref := v.Ref(p.Val)
				switch ref.Kind {
				case model.DatacenterKind:
					v.model.Datacenter = ref.ID
				case model.FolderKind:
					v.model.Folder = ref.ID
				}
			case fChildEntity:
				v.model.Children = v.RefList(p.Val)
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
				v.model.Vms = v.Ref(p.Val)
			case fHostFolder:
				v.model.Clusters = v.Ref(p.Val)
			case fNetFolder:
				v.model.Networks = v.Ref(p.Val)
			case fDsFolder:
				v.model.Datastores = v.Ref(p.Val)
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
			case fParent:
				v.model.Folder = v.Ref(p.Val).ID
			case fHost:
				v.model.Hosts = v.RefList(p.Val)
			case fNetwork:
				v.model.Networks = v.RefList(p.Val)
			case fDatastore:
				v.model.Datastores = v.RefList(p.Val)
			case fDasEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.DasEnabled = b
				}
			case fDasVmCfg:
				refList := []model.Ref{}
				if list, cast := p.Val.([]types.ClusterDasVmConfigInfo); cast {
					for _, val := range list {
						refList = append(refList, v.Ref(val.Key))
					}
				}
				v.model.DasVms = refList
			case fDrsEnabled:
				if b, cast := p.Val.(bool); cast {
					v.model.DrsEnabled = b
				}
			case fDrsVmCfg:
				refList := []model.Ref{}
				if list, cast := p.Val.([]types.ClusterDrsVmConfigInfo); cast {
					for _, val := range list {
						refList = append(refList, v.Ref(val.Key))
					}
				}
				v.model.DrsVms = refList
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
			case fParent:
				v.model.Cluster = v.Ref(p.Val).ID
			case fInMaintMode:
				if b, cast := p.Val.(bool); cast {
					v.model.InMaintenanceMode = b
				}
			case fMgtServerIp:
				if s, cast := p.Val.(string); cast {
					v.model.ManagementServerIp = s
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
				v.model.Vms = v.RefList(p.Val)
			case fProductName:
				if s, cast := p.Val.(string); cast {
					v.model.ProductName = s
				}
			case fProductVersion:
				if s, cast := p.Val.(string); cast {
					v.model.ProductVersion = s
				}
			case fNetwork:
				v.model.Networks = v.RefList(p.Val)
			case fDatastore:
				v.model.Datastores = v.RefList(p.Val)
			case fVSwitch:
				if array, cast := p.Val.(types.ArrayOfHostVirtualSwitch); cast {
					network := &v.model.Network
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
				}
			case fPortGroup:
				if array, cast := p.Val.(types.ArrayOfHostPortGroup); cast {
					network := &v.model.Network
					for _, portGroup := range array.HostPortGroup {
						network.PortGroups = append(
							network.PortGroups,
							model.PortGroup{
								Key:    portGroup.Key,
								Name:   portGroup.Spec.Name,
								Switch: portGroup.Vswitch,
							})
					}
				}
			case fPNIC:
				if array, cast := p.Val.(types.ArrayOfPhysicalNic); cast {
					network := &v.model.Network
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
				}
			case fVNIC:
				if array, cast := p.Val.(types.ArrayOfHostVirtualNic); cast {
					network := &v.model.Network
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
								SubnetMask: nic.Spec.Ip.SubnetMask,
								MTU:        nic.Spec.Mtu,
							})
					}
					sort.Slice(
						network.VNICs,
						func(i, j int) bool {
							return network.VNICs[i].MTU > network.VNICs[j].MTU
						})
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
				v.model.DVSwitch = v.Ref(p.Val)
			}
		}
	}
}

//
// DVSwitch model adapter.
type DVSwitchAdapter struct {
	Base
	// The adapter model.
	model model.Network
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
		hostRef := v.Ref(*member.Config.Host)
		if backing, cast := member.Config.Backing.(*types.DistributedVirtualSwitchHostMemberPnicBacking); cast {
			names := []string{}
			for _, pn := range backing.PnicSpec {
				names = append(names, pn.PnicDevice)
			}
			list = append(list,
				model.DVSHost{
					Host: hostRef,
					PNIC: names,
				})
		}
	}

	v.model.Host = list
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
			case fParent:
				v.model.Folder = v.Ref(p.Val).ID
			case fUUID:
				if s, cast := p.Val.(string); cast {
					v.model.UUID = s
				}
			case fFirmware:
				if s, cast := p.Val.(string); cast {
					v.model.Firmware = s
				}
			case fPowerState:
				if s, cast := p.Val.(types.VirtualMachinePowerState); cast {
					v.model.PowerState = string(s)
				}
			case fConnectionState:
				if s, cast := p.Val.(types.VirtualMachineConnectionState); cast {
					v.model.ConnectionState = string(s)
				}
			case fIsTemplate:
				if b, cast := p.Val.(bool); cast {
					v.model.IsTemplate = b
				}
			case fSnapshot:
				if snapshot, cast := p.Val.(types.VirtualMachineSnapshotInfo); cast {
					ref := snapshot.CurrentSnapshot
					if ref != nil {
						v.model.Snapshot = v.Ref(*ref)
					}
				}
			case fChangeTracking:
				if b, cast := p.Val.(bool); cast {
					v.model.ChangeTrackingEnabled = b
				}
			case fCpuAffinity:
				if a, cast := p.Val.(types.VirtualMachineAffinityInfo); cast {
					v.model.CpuAffinity = a.AffinitySet
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
				v.model.Host = v.Ref(p.Val).ID
			case fVmIpAddress:
				if s, cast := p.Val.(string); cast {
					v.model.IpAddress = s
				}
			case fFtInfo:
				if _, cast := p.Val.(types.FaultToleranceConfigInfo); cast {
					v.model.FaultToleranceEnabled = true
				}
			case fNetwork:
				v.model.Networks = v.RefList(p.Val)
			case fExtraConfig:
				if options, cast := p.Val.(types.ArrayOfOptionValue); cast {
					for _, val := range options.OptionValue {
						opt := val.GetOptionValue()
						switch opt.Key {
						case "numa.nodeAffinity":
							if s, cast := opt.Value.(string); cast {
								v.model.NumaNodeAffinity = strings.Split(s, ",")
							}
						}
					}
				}
			case fDevices:
				if devArray, cast := p.Val.(types.ArrayOfVirtualDevice); cast {
					list := []model.Device{}
					for _, dev := range devArray.VirtualDevice {
						switch dev.(type) {
						case *types.VirtualSriovEthernetCard,
							*types.VirtualPCIPassthrough,
							*types.VirtualSCSIPassthrough,
							*types.VirtualUSBController:
							list = append(
								list,
								model.Device{
									Kind: libref.ToKind(dev),
								})
						}
					}
					v.model.Devices = list
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

	v.model.Disks = disks
}
