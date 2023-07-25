package openstack

import (
	"time"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/regions"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
)

type GetOpts struct {
	ID string
}

type DeleteOpts struct{}

type Region struct {
	regions.Region
}

type RegionCreateOpts struct {
	regions.CreateOpts
}

type RegionUpdateOpts struct {
	regions.UpdateOpts
}

type RegionListOpts struct {
	regions.ListOpts
}

type Project struct {
	projects.Project
}

type ProjectCreateOpts struct {
	projects.CreateOpts
}

type ProjectUpdateOpts struct {
	projects.UpdateOpts
}

type ProjectListOpts struct {
	projects.ListOpts
}

type Image struct {
	images.Image
}

const (
	FilterGTE                = images.FilterGTE
	FilterGT                 = images.FilterGT
	ImageStatusQueued        = images.ImageStatusQueued
	ImageStatusSaving        = images.ImageStatusSaving
	ImageStatusActive        = images.ImageStatusActive
	ImageStatusKilled        = images.ImageStatusKilled
	ImageStatusDeleted       = images.ImageStatusDeleted
	ImageStatusPendingDelete = images.ImageStatusPendingDelete
	ImageStatusDeactivated   = images.ImageStatusDeactivated
	ImageStatusUploading     = "uploading"
	ImageStatusImporting     = images.ImageStatusImporting
)

type ImageCreateOpts struct {
	images.CreateOpts
}

type ImageUpdateOpts struct {
	images.UpdateOpts
}

func (r *ImageUpdateOpts) AddImageProperty(name, value string) {
	r.UpdateOpts = images.UpdateOpts{
		images.UpdateImageProperty{
			Op:    images.AddOp,
			Name:  name,
			Value: value,
		},
	}
}

type ImageListOpts struct {
	images.ListOpts
}

type UpdateImageProperty struct {
	images.UpdateImageProperty
}

func (r *ImageListOpts) SetUpdateAtQueryFilterGTE(lastSync time.Time) {
	r.UpdatedAtQuery = &images.ImageDateQuery{Date: lastSync, Filter: FilterGTE}
}

func (r *ImageListOpts) SetUpdateAtQueryFilterGT(lastSync time.Time) {
	r.UpdatedAtQuery = &images.ImageDateQuery{Date: lastSync, Filter: FilterGT}
}

type ReplaceImageMetadata struct {
	Metadata map[string]string
}

func (r *ReplaceImageMetadata) ToImagePatchMap() map[string]interface{} {
	return map[string]interface{}{
		"op":    "replace",
		"path":  "/metadata",
		"value": r.Metadata,
	}
}

type Flavor struct {
	flavors.Flavor
	ExtraSpecs map[string]string
}

type FlavorCreateOpts struct {
	flavors.CreateOpts
}

type FlavorUpdateOpts struct {
	flavors.UpdateOpts
}

type FlavorListOpts struct {
	flavors.ListOpts
}

const (
	SnapshotStatusCreating      = "creating"
	SnapshotStatusAvailable     = "available"
	SnapshotStatusBackingUp     = "backing-up"
	SnapshotStatusDeleting      = "deleting"
	SnapshotStatusError         = "error"
	SnapshotStatusDeleted       = "deleted"
	SnapshotStatusUnmanaging    = "unmanaging"
	SnapshotStatusRestoring     = "restoring"
	SnapshotStatusErrorDeleting = "error_deleting"
)

type SnapshotCreateOpts struct {
	snapshots.CreateOpts
}

type SnapshotUpdateOpts struct {
	snapshots.UpdateOpts
}

type SnapshotListOpts struct {
	snapshots.ListOpts
}

type Snapshot struct {
	snapshots.Snapshot
}

const (
	VolumeStatusCreating         = "creating"
	VolumeStatusAvailable        = "available"
	VolumeStatusReserved         = "reserved"
	VolumeStatusAttacing         = "attaching"
	VolumeStatusDetaching        = "detaching"
	VolumeStatusInUse            = "in-use"
	VolumeStatusMaintenance      = "maintenance"
	VolumeStatusDeleting         = "deleting"
	VolumeStatusAwaitingTransfer = "awaiting-transfer"
	VolumeStatusError            = "error"
	VolumeStatusErrorDeleting    = "error_deleting"
	VolumeStatusBackingUp        = "backing-up"
	VolumeStatusRestoringBackup  = "restoring-backup"
	VolumeStatusErrorBackingUp   = "error_backing-up"
	VolumeStatusErrorRestoring   = "error_restoring"
	VolumeStatusErrorExtending   = "error_extending"
	VolumeStatusDownloading      = "downloading"
	VolumeStatusUploading        = "uploading"
	VolumeStatusRetyping         = "retyping"
	VolumeStatusExtending        = "extending"
)

type VolumeCreateOpts struct {
	volumes.CreateOpts
}

type VolumeUpdateOpts struct {
	volumes.UpdateOpts
}

type VolumeListOpts struct {
	volumes.ListOpts
}

type Volume struct {
	volumes.Volume
}

type VolumeTypeCreateOpts struct {
	volumetypes.CreateOpts
}

type VolumeTypeUpdateOpts struct {
	volumetypes.UpdateOpts
}

type VolumeTypeListOpts struct {
	volumetypes.ListOpts
}

type VolumeType struct {
	volumetypes.VolumeType
}

type Fault struct {
	servers.Fault
}

// VM Status
const (
	VmStatusActive           = "ACTIVE"
	VmStatusBuild            = "BUILD"
	VmStatusDeleted          = "DELETED"
	VmStatusError            = "ERROR"
	VmStatusHardReboot       = "HARD_REBOOT"
	VmStatusMigrating        = "MIGRATING"
	VmStatusPassword         = "PASSWORD"
	VmStatusPaused           = "PAUSED"
	VmStatusReboot           = "REBOOT"
	VmStatusRebuild          = "REBUILD"
	VmStatusRescue           = "RESCUE"
	VmStatusResize           = "RESIZE"
	VmStatusRevertResize     = "REVERT_RESIZE"
	VmStatusShelved          = "SHELVED"
	VmStatusShelvedOffloaded = "SHELVED_OFFLOADED"
	VmStatusShutoff          = "SHUTOFF"
	VmStatusSoftDeleted      = "SOFT_DELETED"
	VmStatusSuspended        = "SUSPENDED"
	VmStatusUnknown          = "UNKNOWN"
	VmStatusVerifyResize     = "VERIFY_RESIZE"
)

type VM struct {
	servers.Server
}

type VMCreateOpts struct {
	servers.CreateOpts
}

type VMUpdateOpts struct {
	servers.UpdateOpts
}

type VMListOpts struct {
	servers.ListOpts
}

type VMCreateImageOpts struct {
	servers.CreateImageOpts
}

type AttachedVolume struct {
	servers.AttachedVolume
}

const (
	NetworkStatusNull   = "null"
	NetworkStatusActive = "ACTIVE"
	NetworkStatusDown   = "DOWN"
)

type Network struct {
	networks.Network
}

type NetworkCreateOpts struct {
	networks.CreateOpts
}

type NetworkUpdateOpts struct {
	networks.UpdateOpts
}

type NetworkListOpts struct {
	networks.ListOpts
}

type Subnet struct {
	subnets.Subnet
}

type SubnetCreateOpts struct {
	subnets.CreateOpts
}

type SubnetUpdateOpts struct {
	subnets.UpdateOpts
}

type SubnetListOpts struct {
	subnets.ListOpts
}

type AllocationPool struct {
	subnets.AllocationPool
}

type HostRoute struct {
	subnets.HostRoute
}
