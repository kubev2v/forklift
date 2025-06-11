package openstack

import (
	"fmt"
	"reflect"
	"time"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/openstack"
)

type Region struct {
	libclient.Region
}

func (r *Region) ApplyTo(m *model.Region) {
	m.Name = r.ID
	m.Description = r.Description
	m.ParentRegionID = r.ParentRegionID
}

func (r *Region) equalsTo(m *model.Region) bool {
	return m.Name == r.ID &&
		m.Description == r.Description &&
		m.ParentRegionID == r.ParentRegionID
}

type RegionListOpts struct {
	libclient.RegionListOpts
}

type Project struct {
	libclient.Project
}

func (r *Project) ApplyTo(m *model.Project) {
	m.Name = r.Name
	m.Description = r.Description
	m.Enabled = r.Enabled
	m.IsDomain = r.IsDomain
	m.DomainID = r.DomainID
	m.ParentID = r.ParentID
}

func (r *Project) equalsTo(m *model.Project) bool {
	return m.Name == r.Name &&
		m.Description == r.Description &&
		m.Enabled == r.Enabled &&
		m.IsDomain == r.IsDomain &&
		m.DomainID == r.DomainID &&
		m.ParentID == r.ParentID
}

type ProjectListOpts struct {
	libclient.ProjectListOpts
}

type Image struct {
	libclient.Image
}

func (r *Image) ApplyTo(m *model.Image) {
	m.Name = r.Name
	m.Status = string(r.Status)
	m.Tags = r.Tags
	m.ContainerFormat = r.ContainerFormat
	m.DiskFormat = r.DiskFormat
	m.MinDiskGigabytes = r.MinDiskGigabytes
	m.MinRAMMegabytes = r.MinRAMMegabytes
	m.Owner = r.Owner
	m.Protected = r.Protected
	m.Visibility = string(r.Visibility)
	m.Hidden = r.Hidden
	m.Checksum = r.Checksum
	m.SizeBytes = r.SizeBytes
	m.Metadata = r.Metadata
	m.Properties = r.Properties
	m.CreatedAt = r.CreatedAt
	m.UpdatedAt = r.UpdatedAt
	m.File = r.File
	m.Schema = r.Schema
	m.VirtualSize = r.VirtualSize
	m.OpenStackImageImportMethods = r.OpenStackImageImportMethods
	m.OpenStackImageStoreIDs = r.OpenStackImageStoreIDs
}

func (r *Image) updatedAfter(m *model.Image) bool {
	return r.UpdatedAt.After(m.UpdatedAt)
}

type ImageListOpts struct {
	libclient.ImageListOpts
}

type ImageCreateOpts struct {
	libclient.ImageCreateOpts
}

type Flavor struct {
	libclient.Flavor
	ExtraSpecs map[string]string
}

type FlavorListOpts struct {
	libclient.FlavorListOpts
}

func (r *Flavor) ApplyTo(m *model.Flavor) {
	m.Disk = r.Disk
	m.RAM = r.RAM
	m.Name = r.Name
	m.RxTxFactor = fmt.Sprintf("%f", r.RxTxFactor)
	m.Swap = r.Swap
	m.VCPUs = r.VCPUs
	m.IsPublic = r.IsPublic
	m.Ephemeral = r.Ephemeral
	m.Description = r.Description
	m.ExtraSpecs = r.ExtraSpecs
}

func (r *Flavor) equalsTo(m *model.Flavor) bool {
	if !reflect.DeepEqual(r.ExtraSpecs, m.ExtraSpecs) {
		return false
	}
	return m.Disk == r.Disk &&
		m.RAM == r.RAM &&
		m.Name == r.Name &&
		m.RxTxFactor == fmt.Sprintf("%f", r.RxTxFactor) &&
		m.Swap == r.Swap &&
		m.VCPUs == r.VCPUs &&
		m.IsPublic == r.IsPublic &&
		m.Ephemeral == r.Ephemeral &&
		m.Description == r.Description
}

type SnapshotListOpts struct {
	libclient.SnapshotListOpts
}

type Snapshot struct {
	libclient.Snapshot
}

func (r *Snapshot) ApplyTo(m *model.Snapshot) {
	m.CreatedAt = r.CreatedAt
	m.UpdatedAt = r.UpdatedAt
	m.Name = r.Name
	m.Description = r.Description
	m.VolumeID = r.VolumeID
	m.Status = r.Status
	m.Size = r.Size
	m.Metadata = r.Metadata
}

func (r *Snapshot) updatedAfter(m *model.Snapshot) bool {
	return r.UpdatedAt.After(m.UpdatedAt)
}

const (
	VolumeStatusDeleting = "deleting"
)

type VolumeListOpts struct {
	libclient.VolumeListOpts
}

type Volume struct {
	libclient.Volume
}

func (r *Volume) ApplyTo(m *model.Volume) {
	m.Status = r.Status
	m.Size = r.Size
	m.AvailabilityZone = r.AvailabilityZone
	m.CreatedAt = r.CreatedAt
	m.UpdatedAt = r.UpdatedAt
	r.addAttachMents(m)
	m.Name = r.Name
	m.Description = r.Description
	m.VolumeType = r.VolumeType
	m.SnapshotID = r.SnapshotID
	m.SourceVolID = r.SourceVolID
	m.BackupID = r.BackupID
	m.Metadata = r.Metadata
	m.UserID = r.UserID
	m.Bootable = r.Bootable
	m.Encrypted = r.Encrypted
	m.ReplicationStatus = r.ReplicationStatus
	m.ConsistencyGroupID = r.ConsistencyGroupID
	m.Multiattach = r.Multiattach
	m.VolumeImageMetadata = r.VolumeImageMetadata
}

func (r *Volume) addAttachMents(m *model.Volume) {
	m.Attachments = []model.Attachment{}
	for _, n := range r.Attachments {
		m.Attachments = append(
			m.Attachments,
			model.Attachment{
				ID: n.ID,
			})
	}
}

func (r *Volume) updatedAfter(m *model.Volume) bool {
	return r.UpdatedAt.After(m.UpdatedAt)
}

type VolumeTypeListOpts struct {
	libclient.VolumeTypeListOpts
}

type VolumeType struct {
	libclient.VolumeType
}

func (r *VolumeType) ApplyTo(m *model.VolumeType) {
	m.ID = r.ID
	m.Name = r.Name
	m.Description = r.Description
	m.ExtraSpecs = r.ExtraSpecs
	m.IsPublic = r.IsPublic
	m.QosSpecID = r.QosSpecID
	m.PublicAccess = r.PublicAccess
}

func (r *VolumeType) equalsTo(m *model.VolumeType) bool {
	if !reflect.DeepEqual(r.ExtraSpecs, m.ExtraSpecs) {
		return false
	}
	return m.ID == r.ID &&
		m.Name == r.Name &&
		m.Description == r.Description &&
		m.IsPublic == r.IsPublic &&
		m.QosSpecID == r.QosSpecID &&
		m.PublicAccess == r.PublicAccess
}

type Fault struct {
	libclient.Fault
}

func (r *Fault) ApplyTo(m *model.Fault) {
	m.Code = r.Code
	m.Created = r.Created
	m.Details = r.Details
	m.Message = r.Message
}

func (r *Fault) equalsTo(m *model.Fault) bool {
	return m.Code == r.Code &&
		m.Created == r.Created &&
		m.Details == r.Details &&
		m.Message == r.Message
}

type VM struct {
	libclient.VM
}

type VMListOpts struct {
	libclient.VMListOpts
}

func (r *VM) ApplyTo(m *model.VM) {
	m.Name = r.Name
	m.ID = r.ID
	m.TenantID = r.TenantID
	m.UserID = r.UserID
	m.Name = r.Name
	m.Updated = r.Updated
	m.Created = r.Created
	m.HostID = r.HostID
	m.Status = r.Status
	m.Progress = r.Progress
	m.AccessIPv4 = r.AccessIPv4
	m.AccessIPv6 = r.AccessIPv6
	r.addImageID(m)
	r.addFlavorID(m)
	m.Addresses = r.Addresses
	m.Metadata = r.Metadata
	m.KeyName = r.KeyName
	m.AdminPass = r.AdminPass
	m.SecurityGroups = r.SecurityGroups
	r.addAttachedVolumes(m)
	r.addFault(m)
	m.Tags = r.Tags
	m.ServerGroups = r.ServerGroups
}

func (r *VM) addImageID(m *model.VM) {
	m.ImageID, _ = r.Image["id"].(string)
}

func (r *VM) addFlavorID(m *model.VM) {
	m.FlavorID, _ = r.Flavor["id"].(string)
}

func (r *VM) addFault(m *model.VM) {
	m.Fault = model.Fault{}
	f := &Fault{libclient.Fault{Fault: r.Fault}}
	f.ApplyTo(&m.Fault)
}

func (r *VM) addAttachedVolumes(m *model.VM) {
	m.AttachedVolumes = []model.AttachedVolume{}
	for _, n := range r.AttachedVolumes {
		m.AttachedVolumes = append(
			m.AttachedVolumes,
			model.AttachedVolume{
				ID: n.ID,
			})
	}
}

func (r *VM) equalAttachedVolumes(m *model.VM) bool {
	if len(r.AttachedVolumes) != len(m.AttachedVolumes) {
		return false
	}
	for _, rAttachedVolume := range r.AttachedVolumes {
		found := false
		attachedVolume := &AttachedVolume{libclient.AttachedVolume{AttachedVolume: rAttachedVolume}}
		for _, mAttachedVolume := range m.AttachedVolumes {
			if attachedVolume.equalsTo(&mAttachedVolume) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (r *VM) equalsTo(m *model.VM) bool {
	imageID, _ := r.Image["id"].(string)
	if m.ImageID != imageID {
		return false
	}
	flavorID, _ := r.Flavor["id"].(string)
	if m.FlavorID != flavorID {
		return false
	}
	f := &Fault{libclient.Fault{Fault: r.Fault}}
	if !f.equalsTo(&m.Fault) {
		return false
	}
	if !r.equalAttachedVolumes(m) {
		return false
	}
	return m.Name == r.Name &&
		m.ID == r.ID &&
		m.TenantID == r.TenantID &&
		m.UserID == r.UserID &&
		m.Updated.Equal(r.Updated) &&
		m.Created.Equal(r.Created) &&
		m.HostID == r.HostID &&
		m.Status == r.Status &&
		m.Progress == r.Progress &&
		m.AccessIPv4 == r.AccessIPv4 &&
		m.AccessIPv6 == r.AccessIPv6 &&
		reflect.DeepEqual(m.Addresses, r.Addresses) &&
		reflect.DeepEqual(m.Metadata, r.Metadata) &&
		m.KeyName == r.KeyName &&
		m.AdminPass == r.AdminPass &&
		reflect.DeepEqual(m.SecurityGroups, r.SecurityGroups) &&
		reflect.DeepEqual(m.Tags, r.Tags) &&
		reflect.DeepEqual(m.ServerGroups, r.ServerGroups)
}

type AttachedVolume struct {
	libclient.AttachedVolume
}

func (r *AttachedVolume) ApplyTo(m *model.AttachedVolume) {
	m.ID = r.ID
}

func (r *AttachedVolume) equalsTo(m *model.AttachedVolume) bool {
	return m.ID == r.ID
}

type Network struct {
	libclient.Network
}

type NetworkListOpts struct {
	libclient.NetworkListOpts
}

func (r *Network) ApplyTo(m *model.Network) {
	m.Name = r.Name
	m.Description = r.Description
	m.AdminStateUp = r.AdminStateUp
	m.Status = r.Status
	m.Subnets = r.Subnets
	m.TenantID = r.TenantID
	m.CreatedAt = time.Time(r.CreatedAt)
	m.UpdatedAt = time.Time(r.UpdatedAt)
	m.ProjectID = r.ProjectID
	m.Shared = r.Shared
	m.AvailabilityZoneHints = r.AvailabilityZoneHints
	m.Tags = r.Tags
	m.RevisionNumber = r.RevisionNumber
}

func (r *Network) updatedAfter(m *model.Network) bool {
	return time.Time(r.UpdatedAt).After(m.UpdatedAt)
}

type Subnet struct {
	libclient.Subnet
}

type SubnetListOpts struct {
	libclient.SubnetListOpts
}

func (r *Subnet) ApplyTo(m *model.Subnet) {
	m.ID = r.ID
	m.NetworkID = r.NetworkID
	m.Name = r.Name
	m.Description = r.Description
	m.IPVersion = r.IPVersion
	m.CIDR = r.CIDR
	m.GatewayIP = r.GatewayIP
	m.DNSNameservers = r.DNSNameservers
	m.ServiceTypes = r.ServiceTypes
	r.addAllocationPools(m)
	r.addHostRoutes(m)
	m.EnableDHCP = r.EnableDHCP
	m.TenantID = r.TenantID
	m.ProjectID = r.ProjectID
	m.IPv6AddressMode = r.IPv6AddressMode
	m.IPv6RAMode = r.IPv6RAMode
	m.SubnetPoolID = r.SubnetPoolID
	m.Tags = r.Tags
	m.RevisionNumber = r.RevisionNumber
}

func (r *Subnet) addAllocationPools(m *model.Subnet) {
	m.AllocationPools = []model.AllocationPool{}
	for i := range r.AllocationPools {
		allocationPool := AllocationPool{libclient.AllocationPool{AllocationPool: r.AllocationPools[i]}}
		ap := &model.AllocationPool{}
		allocationPool.ApplyTo(ap)
		m.AllocationPools = append(m.AllocationPools, *ap)
	}
}
func (r *Subnet) addHostRoutes(m *model.Subnet) {
	m.HostRoutes = []model.HostRoute{}
	for i := range r.HostRoutes {
		hostRoute := HostRoute{libclient.HostRoute{HostRoute: r.HostRoutes[i]}}
		hr := &model.HostRoute{}
		hostRoute.ApplyTo(hr)
		m.HostRoutes = append(m.HostRoutes, *hr)
	}
}

func (r *Subnet) equalsTo(m *model.Subnet) bool {
	if !reflect.DeepEqual(r.DNSNameservers, m.DNSNameservers) {
		return false
	}
	if !reflect.DeepEqual(r.ServiceTypes, m.ServiceTypes) {
		return false
	}
	if !reflect.DeepEqual(r.Tags, m.Tags) {
		return false
	}
	for i := range r.HostRoutes {
		hostRoute := HostRoute{libclient.HostRoute{HostRoute: r.HostRoutes[i]}}
		if !hostRoute.equalsTo(&m.HostRoutes[i]) {
			return false
		}
	}
	for i := range r.AllocationPools {
		allocationPool := AllocationPool{libclient.AllocationPool{AllocationPool: r.AllocationPools[i]}}
		if !allocationPool.equalsTo(&m.AllocationPools[i]) {
			return false
		}
	}
	return m.ID == r.ID &&
		m.NetworkID == r.NetworkID &&
		m.Name == r.Name &&
		m.Description == r.Description &&
		m.IPVersion == r.IPVersion &&
		m.CIDR == r.CIDR &&
		m.GatewayIP == r.GatewayIP &&
		m.EnableDHCP == r.EnableDHCP &&
		m.TenantID == r.TenantID &&
		m.ProjectID == r.ProjectID &&
		m.IPv6AddressMode == r.IPv6AddressMode &&
		m.IPv6RAMode == r.IPv6RAMode &&
		m.SubnetPoolID == r.SubnetPoolID &&
		m.RevisionNumber == r.RevisionNumber
}

type AllocationPool struct {
	libclient.AllocationPool
}

func (r *AllocationPool) ApplyTo(m *model.AllocationPool) {
	m.Start = r.Start
	m.End = r.End
}

func (r *AllocationPool) equalsTo(m *model.AllocationPool) bool {
	return m.Start == r.Start && m.End == r.End
}

type HostRoute struct {
	libclient.HostRoute
}

func (r *HostRoute) ApplyTo(m *model.HostRoute) {
	m.DestinationCIDR = r.DestinationCIDR
	m.NextHop = r.NextHop
}

func (r *HostRoute) equalsTo(m *model.HostRoute) bool {
	return m.DestinationCIDR == r.DestinationCIDR && m.NextHop == r.NextHop
}
