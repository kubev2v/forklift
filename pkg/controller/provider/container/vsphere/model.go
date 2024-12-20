package vsphere

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
	"github.com/vmware/govmomi/vim25/types"
)

// Model adapter.
// Each adapter provides provider-specific management of a model.
type Adapter interface {
	// The adapter model.
	Model() model.Model
	// Apply the update to the model.
	Apply(types.ObjectUpdate)
}

// Base adapter.
type Base struct {
}

// Apply the update to the model `Base`.
func (b *Base) Apply(m *model.Base, u types.ObjectUpdate) {
	for _, p := range u.ChangeSet {
		switch p.Op {
		case Assign:
			switch p.Name {
			case fName:
				m.Name = b.Decoded(p.Val)
			case fParent:
				m.Parent = b.Ref(p.Val)
			}
		}
	}
}

// Build ref.
func (b *Base) Ref(in types.AnyType) (ref model.Ref) {
	if r, cast := in.(types.ManagedObjectReference); cast {
		ref.ID = r.Value
		switch r.Type {
		case Folder:
			ref.Kind = model.FolderKind
		case Datacenter:
			ref.Kind = model.DatacenterKind
		case Cluster,
			ComputeResource:
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

// Build a []Ref.
func (b *Base) RefList(in types.AnyType) (list []model.Ref) {
	if a, cast := in.(types.ArrayOfManagedObjectReference); cast {
		for _, r := range a.ManagedObjectReference {
			list = append(list, b.Ref(r))
		}
	}

	return
}

// URL decoded string.
// Some property values returned by the property
// collector are URL-encoded.
func (b *Base) Decoded(in types.AnyType) (s string) {
	var cast bool
	if s, cast = in.(string); cast {
		decoded, err := url.QueryUnescape(s)
		if err == nil {
			s = decoded
		}
	}

	return
}

// Folder model adapter.
type FolderAdapter struct {
	Base
	// The adapter model.
	model model.Folder
}

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

// The new model.
func (v *FolderAdapter) Model() model.Model {
	return &v.model
}

// Datacenter model adapter.
type DatacenterAdapter struct {
	Base
	// The adapter model.
	model model.Datacenter
}

// The adapter model.
func (v *DatacenterAdapter) Model() model.Model {
	return &v.model
}

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

// Cluster model adapter.
type ClusterAdapter struct {
	Base
	// The adapter model.
	model model.Cluster
}

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

// Host model adapter.
type HostAdapter struct {
	Base
	// The adapter model.
	model model.Host
}

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
			case fOverallStatus:
				if s, cast := p.Val.(types.ManagedEntityStatus); cast {
					v.model.Status = string(s)
				}
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
			case fTimezone:
				if s, cast := p.Val.(string); cast {
					v.model.Timezone = s
				}
			case fCpuSockets:
				if b, cast := p.Val.(int16); cast {
					v.model.CpuSockets = b
				}
			case fCpuCores:
				if b, cast := p.Val.(int16); cast {
					v.model.CpuCores = b
				}
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
					network.Switches = nil
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
					network.PortGroups = nil
					for _, portGroup := range array.HostPortGroup {
						network.PortGroups = append(
							network.PortGroups,
							model.PortGroup{
								Key:    portGroup.Key,
								Name:   portGroup.Spec.Name,
								Switch: portGroup.Vswitch,
								VlanId: portGroup.Spec.VlanId,
							})
					}
				}
			case fPNIC:
				if array, cast := p.Val.(types.ArrayOfPhysicalNic); cast {
					network := &v.model.Network
					network.PNICs = nil
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
					network.VNICs = nil
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

// Network model adapter.
type NetworkAdapter struct {
	Base
	// The adapter model.
	model model.Network
}

// The adapter model.
func (v *NetworkAdapter) Model() model.Model {
	return &v.model
}

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
			case fKey:
				if s, cast := p.Val.(string); cast {
					v.model.Key = s
				}
			case fDVSwitch:
				v.model.DVSwitch = v.Ref(p.Val)
			case fSummary:
				if s, cast := p.Val.(types.OpaqueNetworkSummary); cast {
					v.model.Key = s.OpaqueNetworkId
				}
			case fDVSwitchVlan:
				if portSettings, cast := p.Val.(types.VMwareDVSPortSetting); cast {
					switch vlanIdSpec := portSettings.Vlan.(type) {
					case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
						if int(vlanIdSpec.VlanId) > 0 {
							v.model.VlanId = strconv.Itoa(int(vlanIdSpec.VlanId))
						}
					case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
						refList := []string{}
						for _, val := range vlanIdSpec.VlanId {
							refList = append(refList, fmt.Sprintf("%d-%d", val.Start, val.End))
						}
						v.model.VlanId = strings.Join(refList, ",")
					}
				}
			}
		}
	}
}

// DVSwitch model adapter.
type DVSwitchAdapter struct {
	Base
	// The adapter model.
	model model.Network
}

// The adapter model.
func (v *DVSwitchAdapter) Model() model.Model {
	return &v.model
}

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

// Datastore model adapter.
type DatastoreAdapter struct {
	Base
	// The adapter model.
	model model.Datastore
}

// The adapter model.
func (v *DatastoreAdapter) Model() model.Model {
	return &v.model
}

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

// VM model adapter.
type VmAdapter struct {
	Base
	// The adapter model.
	model model.VM
}

// The adapter model.
func (v *VmAdapter) Model() model.Model {
	return &v.model
}

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
			case fBootOptions:
				if a, cast := p.Val.(types.VirtualMachineBootOptions); cast {
					if a.EfiSecureBootEnabled != nil {
						v.model.SecureBoot = *a.EfiSecureBootEnabled
					}
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
			case fTpmPresent:
				if b, cast := p.Val.(bool); cast {
					v.model.TpmEnabled = b
				}
			case fGuestID:
				if s, cast := p.Val.(string); cast {
					// When the VM isn't powered on, the guest tools don't report
					// the guest id. Only set the guest id if it's being reported,
					// so that the stored value isn't erased when the VM
					// is powered down.
					if s != "" {
						v.model.GuestID = s
					}
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
						case "ctkEnabled":
							if s, cast := opt.Value.(string); cast {
								boolVal, err := strconv.ParseBool(s)
								if err != nil {
									return
								}
								v.model.ChangeTrackingEnabled = boolVal
							}
						}
					}
				}
			case fGuestNet:
				if nics, cast := p.Val.(types.ArrayOfGuestNicInfo); cast {
					guestNetworksList := []model.GuestNetwork{}
					for _, info := range nics.GuestNicInfo {
						if info.IpConfig == nil {
							continue
						}
						for _, ip := range info.IpConfig.IpAddress {
							var dnsList []string
							if info.DnsConfig != nil {
								dnsList = info.DnsConfig.IpAddress
							}
							guestNetworksList = append(guestNetworksList, model.GuestNetwork{
								MAC:          info.MacAddress,
								IP:           ip.IpAddress,
								Origin:       ip.Origin,
								PrefixLength: ip.PrefixLength,
								DNS:          dnsList,
							})
						}
					}
					// when the vm goes down, we get an update with empty values - the following check keeps the previously reported data.
					if len(guestNetworksList) > 0 {
						v.model.GuestNetworks = guestNetworksList
					}
				}
			case fGuestIpStack:
				if ipas, cast := p.Val.(types.ArrayOfGuestStackInfo); cast {
					guestIpStackList := []model.GuestIpStack{}
					for _, ipa := range ipas.GuestStackInfo {
						routes := ipa.IpRouteConfig.IpRoute
						for _, route := range routes {
							var dnsList []string
							if ipa.DnsConfig != nil {
								dnsList = ipa.DnsConfig.IpAddress
							}
							if len(route.Gateway.IpAddress) > 0 {
								guestIpStackList = append(guestIpStackList, model.GuestIpStack{
									Gateway: route.Gateway.IpAddress,
									DNS:     dnsList,
								})
							}
						}
					}
					// when the vm goes down, we get an update with empty values - the following check keeps the previously reported data.
					if len(guestIpStackList) > 0 {
						v.model.GuestIpStacks = guestIpStackList
					}
				}
			case fDevices:
				if devArray, cast := p.Val.(types.ArrayOfVirtualDevice); cast {
					devList := []model.Device{}
					nicList := []model.NIC{}
					for _, dev := range devArray.VirtualDevice {
						var nic *types.VirtualEthernetCard
						switch device := dev.(type) {
						case *types.VirtualSriovEthernetCard,
							*types.VirtualPCIPassthrough,
							*types.VirtualSCSIPassthrough,
							*types.VirtualUSBController:
							devList = append(
								devList,
								model.Device{
									Kind: libref.ToKind(dev),
								})
						case *types.VirtualE1000:
							nic = &device.VirtualEthernetCard
						case *types.VirtualE1000e:
							nic = &device.VirtualEthernetCard
						case *types.VirtualVmxnet:
							nic = &device.VirtualEthernetCard
						case *types.VirtualVmxnet2:
							nic = &device.VirtualEthernetCard
						case *types.VirtualVmxnet3:
							nic = &device.VirtualEthernetCard
						}

						if nic != nil && nic.Backing != nil {
							var network string
							switch backing := dev.GetVirtualDevice().Backing.(type) {
							case *types.VirtualEthernetCardNetworkBackingInfo:
								if backing.Network != nil {
									network = backing.Network.Value
								}
							case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
								network = backing.Port.PortgroupKey
							case *types.VirtualEthernetCardOpaqueNetworkBackingInfo:
								network = backing.OpaqueNetworkId
							}

							devList = append(
								devList,
								model.Device{
									Kind: libref.ToKind(dev),
								})

							nicList = append(
								nicList,
								model.NIC{
									MAC: nic.MacAddress,
									Network: model.Ref{
										Kind: model.NetKind,
										ID:   network,
									},
								})
						}
					}
					v.model.Devices = devList
					v.model.NICs = nicList
					v.updateDisks(&devArray)
				}
			}
		}
	}
}

// Update virtual disk devices.
func (v *VmAdapter) updateDisks(devArray *types.ArrayOfVirtualDevice) {
	disks := []model.Disk{}
	for _, dev := range devArray.VirtualDevice {
		switch dev.(type) {
		case *types.VirtualDisk:
			disk := dev.(*types.VirtualDisk)
			switch backing := disk.Backing.(type) {
			case *types.VirtualDiskFlatVer1BackingInfo:
				md := model.Disk{
					Key:      disk.Key,
					File:     backing.FileName,
					Capacity: disk.CapacityInBytes,
					Mode:     backing.DiskMode,
				}
				if backing.Datastore != nil {
					datastoreId, _ := sanitize(backing.Datastore.Value)
					md.Datastore = model.Ref{
						Kind: model.DsKind,
						ID:   datastoreId,
					}
				}
				disks = append(disks, md)
			case *types.VirtualDiskFlatVer2BackingInfo:
				md := model.Disk{
					Key:      disk.Key,
					File:     backing.FileName,
					Capacity: disk.CapacityInBytes,
					Shared:   backing.Sharing != "sharingNone",
					Mode:     backing.DiskMode,
				}
				if backing.Datastore != nil {
					datastoreId, _ := sanitize(backing.Datastore.Value)
					md.Datastore = model.Ref{
						Kind: model.DsKind,
						ID:   datastoreId,
					}
				}
				disks = append(disks, md)
			case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
				md := model.Disk{
					Key:      disk.Key,
					File:     backing.FileName,
					Capacity: disk.CapacityInBytes,
					Shared:   backing.Sharing != "sharingNone",
					Mode:     backing.DiskMode,
					RDM:      true,
				}
				if backing.Datastore != nil {
					datastoreId, _ := sanitize(backing.Datastore.Value)
					md.Datastore = model.Ref{
						Kind: model.DsKind,
						ID:   datastoreId,
					}
				}
				disks = append(disks, md)
			case *types.VirtualDiskRawDiskVer2BackingInfo:
				md := model.Disk{
					Key:      disk.Key,
					File:     backing.DescriptorFileName,
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
