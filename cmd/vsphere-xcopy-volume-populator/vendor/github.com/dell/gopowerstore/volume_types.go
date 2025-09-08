/*
 *
 * Copyright © 2020-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

import (
	"net/http"

	"github.com/dell/gopowerstore/api"
)

// VolumeStateEnum Volume life cycle states.
type VolumeStateEnum string

const (
	// VolumeStateEnumReady - Volume is operating normally
	VolumeStateEnumReady VolumeStateEnum = "Ready"
	// VolumeStateEnumInitializing - Volume is starting but not yet ready for use
	VolumeStateEnumInitializing VolumeStateEnum = "Initializing"
	// VolumeStateEnumOffline - Volume is not available
	VolumeStateEnumOffline VolumeStateEnum = "Offline"
	// VolumeStateEnumDestroying - Volume is being deleted. No new operations are allowed
	VolumeStateEnumDestroying VolumeStateEnum = "Destroying"
)

type NodeAffinityEnum string

const (
	NodeAffinityEnumSelectAtAttach NodeAffinityEnum = "System_Select_At_Attach"
	NodeAffinityEnumSelectNodeA    NodeAffinityEnum = "System_Selected_Node_A"
	NodeAffinityEnumSelectNodeB    NodeAffinityEnum = "System_Selected_Node_B"
	NodeAffinityEnumPreferredNodeA NodeAffinityEnum = "Preferred_Node_A"
	NodeAffinityEnumPreferredNodeB NodeAffinityEnum = "Preferred_Node_B"
)

// VolumeTypeEnum Type of volume.
type VolumeTypeEnum string

const (
	// VolumeTypeEnumPrimary - A base object.
	VolumeTypeEnumPrimary VolumeTypeEnum = "Primary"
	// VolumeTypeEnumClone - A read-write object that shares storage with the object from which it is sourced.
	VolumeTypeEnumClone VolumeTypeEnum = "Clone"
	// VolumeTypeEnumSnapshot - A read-only object created from a volume or clone.
	VolumeTypeEnumSnapshot VolumeTypeEnum = "Snapshot"
)

// StorageCreatorTypeEnum Creator type of the storage resource.
type StorageCreatorTypeEnum string

const (
	// StorageCreatorTypeEnumUser - A resource created by a user
	StorageCreatorTypeEnumUser StorageCreatorTypeEnum = "User"
	// StorageCreatorTypeEnumSystem - A resource created by the replication engine.
	StorageCreatorTypeEnumSystem StorageCreatorTypeEnum = "System"
	// StorageCreatorTypeEnumScheduler - A resource created by the snapshot scheduler
	StorageCreatorTypeEnumScheduler StorageCreatorTypeEnum = "Scheduler"
)

// StorageTypeEnum Possible types of storage for a volume.
type StorageTypeEnum string

const (
	// StorageTypeEnumBlock - Typical storage type that is displayed for all system management.
	StorageTypeEnumBlock StorageTypeEnum = "Block"
	// StorageTypeEnumFile - Volume internal to an SD-NAS file_system or nas_server object. Not manageable by the external user
	StorageTypeEnumFile StorageTypeEnum = "File"
)

type AppTypeEnum string

const (
	AppTypeEnumRelationDB                 AppTypeEnum = "Relational_Databases_Other"
	AppTypeEnumOracle                     AppTypeEnum = "Relational_Databases_Oracle"
	AppTypeEnumSQLServer                  AppTypeEnum = "Relational_Databases_SQL_Server"
	AppTypeEnumPostgreSQL                 AppTypeEnum = "Relational_Databases_PostgreSQL"
	AppTypeEnumMySQL                      AppTypeEnum = "Relational_Databases_MySQL"
	AppTypeEnumIBMDB2                     AppTypeEnum = "Relational_Databases_IBM_DB2"
	AppTypeEnumBigData                    AppTypeEnum = "Big_Data_Analytics_Other" // #nosec G101
	AppTypeEnumMongoDB                    AppTypeEnum = "Big_Data_Analytics_MongoDB"
	AppTypeEnumCassandra                  AppTypeEnum = "Big_Data_Analytics_Cassandra"
	AppTypeEnumSAPHANA                    AppTypeEnum = "Big_Data_Analytics_SAP_HANA"
	AppTypeEnumSpark                      AppTypeEnum = "Big_Data_Analytics_Spark" // #nosec G101
	AppTypeEnumSplunk                     AppTypeEnum = "Big_Data_Analytics_Splunk"
	AppTypeEnumElasticSearch              AppTypeEnum = "Big_Data_Analytics_ElasticSearch"
	AppTypeEnumExchange                   AppTypeEnum = "Business_Applications_Exchange"
	AppTypeEnumSharepoint                 AppTypeEnum = "Business_Applications_Sharepoint"
	AppTypeEnumRBusinessApplicationsOther AppTypeEnum = "Business_Applications_Other"
	AppTypeEnumRelationERPSAP             AppTypeEnum = "Business_Applications_ERP_SAP"
	AppTypeEnumCRM                        AppTypeEnum = "Business_Applications_CRM"
	AppTypeEnumHealthcareOther            AppTypeEnum = "Healthcare_Other"
	AppTypeEnumEpic                       AppTypeEnum = "Healthcare_Epic"
	AppTypeEnumMEDITECH                   AppTypeEnum = "Healthcare_MEDITECH"
	AppTypeEnumAllscripts                 AppTypeEnum = "Healthcare_Allscripts"
	AppTypeEnumCerner                     AppTypeEnum = "Healthcare_Cerner"
	AppTypeEnumVirtualization             AppTypeEnum = "Virtualization_Other"
	AppTypeEnumVirtualServers             AppTypeEnum = "Virtualization_Virtual_Servers_VSI"
	AppTypeEnumContainers                 AppTypeEnum = "Virtualization_Containers_Kubernetes"
	AppTypeEnumVirtualDesktops            AppTypeEnum = "Virtualization_Virtual_Desktops_VDI"
	AppTypeEnumRelationOther              AppTypeEnum = "Other"
)

// Actions are used to build a PowerStore API query. Each action represents an
// endpoint under the /volume/ prefix.
const (
	VolumeActionClone              string = "clone"
	VolumeActionComputeDifferences string = "compute_differences"
	VolumeActionConfigureMetro     string = "configure_metro"
	VolumeActionEndMetro           string = "end_metro"
	VolumeActionSnapshot           string = "snapshot"
)

// VolumeCreate create volume request
type VolumeCreate struct {
	// Unique name for the volume to be created.
	// This value must contain 128 or fewer printable Unicode characters.
	Name *string `json:"name"`
	// Optional sector size, in bytes. Only 512-byte and 4096-byte sectors are supported.
	SectorSize *int64 `json:"sector_size,omitempty"`
	// Size of the volume to be created, in bytes. Minimum volume size is 1MB.
	// Maximum volume size is 256TB. Size must be a multiple of 8192.
	Size *int64 `json:"size"`
	// Volume group to add the volume to. If not specified, the volume is not added to a volume group.
	VolumeGroupID string `json:"volume_group_id,omitempty"`
	// Appliance on which volume will be placed on. If not specified, an appliance is chosen by the array.
	ApplianceID string `json:"appliance_id,omitempty"`
	// Description of the volume
	Description string `json:"description,omitempty"`
	// Protection policy to associate the volume with. If not specified, protection policy is not associated to the volume.
	ProtectionPolicyID string `json:"protection_policy_id,omitempty"`
	// Performance policy to associate the volume with. If not specified, performance policy is not associated to the volume.
	PerformancePolicyID string `json:"performance_policy_id,omitempty"`
	// Type of application using the volume
	AppType AppTypeEnum `json:"app_type,omitempty"`
	// More details on type of application using the volume
	AppTypeOther string `json:"app_type_other,omitempty"`
	// Unique identifier of a host attached to a volume
	HostID string `json:"host_id,omitempty"`
	// Unique identifier of a host group attached to a volume. The host_id and host_group_id cannot both be set.
	HostGroupID string `json:"host_group_id,omitempty"`
	// Logical unit number for the host volume access.
	LogicalUnitNumber int64 `json:"logical_unit_number,omitempty"`
	// Minimum size for the volume, in bytes.
	MinimumSize int64 `json:"min_size,omitempty"`

	// Metadata addition for volumes on array with OE version 3.0 and above
	Metadata *map[string]string `json:"metadata,omitempty"`

	MetaDataHeader
}

// VolumeComputeDifferences compute snapshot differences in a volume request
type VolumeComputeDifferences struct {
	// Unique identifier of the snapshot used to determine the differences from the current snapshot.
	// If not specified, returns all allocated extents of the current snapshot.
	// The base snapshot must be from the same base volume as the snapshot being compared with.
	BaseSnapshotID *string `json:"base_snapshot_id"`
	// The position of the first logical byte to be used in the comparison.
	// If not specified, the comparison starts at the beginning of the snapshot.
	// The offset must be a multiple of the chunk_size. For best performance, use a multiple of 4K bytes.
	Offset *int64 `json:"offset"`
	// Length of the comparison scan segment in bytes. length / chunk_size is the number of chunks,
	// with each chunk represented as a bit in the bitmap returned in the response. The number of chunks
	// must be divisible by 8 so that the returned bitmap is a byte array. The length and chunk_size
	// must be chosen so that there are no more than 32K chunks, resulting in a returned byte array
	// bitmap of at most 4K bytes. The length starting from the offset must not exceed the size of
	// the snapshot. The length must be a multiple of the chunk_size.
	Length *int64 `json:"length"`
	// Granularity of the chunk in bytes. Must be a power of 2 so that each bit in the returned
	// bitmap represents a chunk sized range of bytes.
	ChunkSize *int64 `json:"chunk_size"`
}

// VolumeComputeDifferencesResponse compute snapshot differences in a volume response
type VolumeComputeDifferencesResponse struct {
	// Base64-encoded bitmap with bits set for chunks that are either:
	// Allocated and nonzero when base_snapshot_id not specified, or
	// Unshared with the base snapshot when a base_snapshot_id is specified
	ChunkBitmap *string `json:"chunk_bitmap"`
	// Recommended offset to be used for the next compute_differences invocation
	// A value of -1 will be returned if the end of the object has been reached
	// while scanning for differences or allocations
	NextOffset *int64 `json:"next_offset"`
}

// MetaData returns the metadata headers.
func (vc *VolumeCreate) MetaData() http.Header {
	vc.once.Do(func() {
		vc.metadata = api.NewSafeHeader().GetHeader()
	})
	return vc.metadata
}

// VolumeModify modify volume request
type VolumeModify struct {
	// Unique identifier of the volume instance.
	Name string `json:"name,omitempty"`
	//  Size of the volume in bytes. Minimum volume size is 1MB. Maximum volume size is 256TB.
	//  Size must be a multiple of 8192.
	Size int64 `json:"size,omitempty"`
	// Unique identifier of the protection policy assigned to the volume.
	ProtectionPolicyID string `json:"protection_policy_id"`
	// Unique identifier of the performance policy assigned to the volume.
	PerformancePolicyID string `json:"performance_policy_id,omitempty"`
	// Description of the volume
	Description string `json:"description"`
	// This attribute indicates the intended use of this volume.
	AppType string `json:"app_type,omitempty"`
	// An optional field used to describe application type usage for a volume.
	AppTypeOther string `json:"app_type_other,omitempty"`
	// ExpirationTimestamp provides time at which snapshot will be auto-purged. Valid only for snapshot type.
	ExpirationTimestamp *string `json:"expiration_timestamp,omitempty"`
}

// VolumeClone request for cloning snapshot/volume
type VolumeClone struct {
	// Unique name for the volume to be created.
	Name        *string `json:"name"`
	Description *string `json:"description,omitempty"`
	MetaDataHeader
}

// MetaData returns the metadata headers.
func (vc *VolumeClone) MetaData() http.Header {
	vc.once.Do(func() {
		vc.metadata = api.NewSafeHeader().GetHeader()
	})
	return vc.metadata
}

// SnapshotCreate params for creating 'create snapshot' request
type SnapshotCreate struct {
	// Unique name for the snapshot to be created.
	Name *string `json:"name,omitempty"`
	// Description of the snapshot.
	Description *string `json:"description,omitempty"`
	// Unique identifier of the performance policy assigned to the volume.
	PerformancePolicyID string `json:"performance_policy_id,omitempty"`
	// ExpirationTimestamp provides volume group creation time
	ExpirationTimestamp string `json:"expiration_timestamp,omitempty"`
	// CreatorType provides volume group creation time
	CreatorType StorageCreatorTypeEnum `json:"creator_type,omitempty"`
}

// VolumeDelete body for VolumeDelete request
type VolumeDelete struct {
	ForceInternal *bool `json:"force_internal,omitempty"`
}

// Appliance instance on the array
type ApplianceInstance struct {
	// Unique identifier for the Appliance
	ID string `json:"id"`
	// Name of the Appliance
	Name string `json:"name"`
	// ServiceTag is the service tag attached to the appliance
	ServiceTag string `json:"service_tag,omitempty"`
}

// Volume Details about a volume, including snapshots and clones of volumes.
type Volume struct {
	Description string `json:"description,omitempty"`
	// Unique identifier of the volume instance.
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	//  Size of the volume in bytes. Minimum volume size is 1MB. Maximum volume size is 256TB.
	//  Size must be a multiple of 8192.
	Size int64 `json:"size,omitempty"`
	// state
	State VolumeStateEnum `json:"state,omitempty"`
	// type
	Type VolumeTypeEnum `json:"type,omitempty"`
	// volume topology
	// World wide name of the volume.
	Wwn string `json:"wwn,omitempty"`
	// ApplianceID - Placeholder for appliance ID where the volume resides
	ApplianceID string `json:"appliance_id,omitempty"`
	// ProtectionData provides snapshot details of the volume
	ProtectionData ProtectionData `json:"protection_data,omitempty"`
	// CreationTimeStamp provides volume group creation time
	CreationTimeStamp string `json:"creation_timestamp,omitempty"`
	// Current amount of data (in bytes) host has written to a volume without dedupe, compression or sharing.
	LogicalUsed int64 `json:"logical_used,omitempty"`
	// It shows which node will be advertised as the optimized IO path to the volume
	NodeAffinity NodeAffinityEnum `json:"node_affinity,omitempty"`
	// Unique identifier of the protection policy assigned to the volume. Only applicable to primary and clone volumes.
	ProtectionPolicyID string `json:"protection_policy_id,omitempty"`
	// Unique identifier of the performance policy assigned to the volume.
	PerformancePolicyID string `json:"performance_policy_id,omitempty"`
	// Indicates whether this volume is a replication destination.
	IsReplicationDestination bool `json:"is_replication_destination,omitempty"`
	// This attribute indicates the intended use of this volume. It may be null.
	AppType AppTypeEnum `json:"app_type,omitempty"`
	// An optional field used to describe application type usage for a volume.
	AppTypeOther string `json:"app_type_other,omitempty"`
	// NVMe Namespace unique identifier in the NVME subsystem. Used for volumes attached to NVMEoF hosts.
	Nsid int64 `json:"nsid,omitempty"`
	// NVMe Namespace globally unique identifier. Used for volumes attached to NVMEoF hosts.
	Nguid string `json:"nguid,omitempty"`
	// Appliance defines the properties of the appliance
	Appliance ApplianceInstance `json:"Appliance"`
	// MigrationSessionID is the Unique identifier of the migration session assigned to the volume if it is part of a migration activity.
	MigrationSessionID string `json:"migration_session_id,omitempty"`
	// MetroReplicationSessionID id the Unique identifier of the replication session assigned to the volume if it has been configured as a metro volume between two PowerStore clusters
	MetroReplicationSessionID string `json:"metro_replication_session_id,omitempty"`
	// TypeL10n Localized message string corresponding to type
	TypeL10n string `json:"type_l10n,omitempty"`
	// StateL10n Localized message string corresponding to state
	StateL10n string `json:"state_l10n,omitempty"`
	// NodeAffinityL10n Localized message string corresponding to Node Affinity
	NodeAffinityL10n string `json:"node_affinity_l10n,omitempty"`
	// AppTypeL10n Localized message string corresponding to App type
	AppTypeL10n string `json:"app_type_l10n,omitempty"`
	// LocationHistory contains the storage resource location history.
	LocationHistory []LocationHistory `json:"location_history,omitempty"`
	// ProtectionPolicy defines the properties of a policy.
	ProtectionPolicy ProtectionPolicy `json:"protection_policy,omitempty"`
	// MigrationSession defines the migration session.
	MigrationSession MigrationSession `json:"migration_session,omitempty"`
	// MappedVolumes contains details about a configured host or host group attached to a volume.
	MappedVolumes []MappedVolumes `json:"mapped_volumes,omitempty"`
	// VolumeGroup contains information about a volume group.
	VolumeGroup []VolumeGroup `json:"volume_groups,omitempty"`
	// Datastores defines properties of a datastore.
	Datastores []Datastores `json:"datastores,omitempty"`
}

// ProtectionData is a field that holds meta information about volume creation
type ProtectionData struct {
	SourceID            string `json:"source_id"`
	ExpirationTimeStamp string `json:"expiration_timestamp"`
	CreatorType         string `json:"creator_type"`
	ParentID            string `json:"parent_id"`
}

// LocationHistory of the volume resource
type LocationHistory struct {
	FromApplianceID string `json:"from_appliance_id"`
	ToApplianceID   string `json:"to_appliance_id"`
}

// MigrationSession details of migration session
type MigrationSession struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MappedVolumes provides details about a configured host or host group attached to a volume
type MappedVolumes struct {
	ID string `json:"id"`
}

// Datastores contains properties of datastores.
type Datastores struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	InstanceUUID string `json:"istance_uuid"`
}

type VirtualVolume struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size string `json:"size"`
}

// Fields returns fields which must be requested to fill struct
func (v *Volume) Fields() []string {
	return []string{"*"}
}

// Fields returns fields which must be requested to fill struct
func (n *ApplianceInstance) Fields() []string {
	return []string{"id", "name", "service_tag"}
}

// MetroConfig defines the properties required to configure a metro volume replication session.
type MetroConfig struct {
	// RemoteSystemID is a required parameter specifying the remote PowerStore array/cluster on which
	// the metro volume should be replicated.
	RemoteSystemID string `json:"remote_system_id"`
	// RemoteApplianceID is an optional parameter specifying a specific remote PowerStore appliance
	// on which the metro volume or volume group should be replicated.
	RemoteApplianceID string `json:"remote_appliance_id,omitempty"`
}

// MetroSessionResponse id the Unique identifier of the replication session assigned
// to the volume if it has been configured as a metro volume between two PowerStore clusters.
type MetroSessionResponse struct {
	// ID is a unique identifier of the metro replication session and
	// is included in response to configuring a metro .
	ID string `json:"metro_replication_session_id,omitempty"`
}

// EndMetroVolumeOptions provides options for deleting the remote volume and forcing the deletion.
type EndMetroVolumeOptions struct {
	// DeleteRemoteVolume specifies whether or not to delete the remote volume when ending the metro session.
	DeleteRemoteVolume bool `json:"delete_remote_volume,omitempty"`
	// ForceDelete specifies if the Metro volume should be forcefully deleted.
	// If the force option is specified, any errors returned while attempting to tear down the remote side of the
	// metro session will be ignored and the remote side may be left in an indeterminate state.
	// If any errors occur on the local side the operation can still fail.
	// It is not recommended to use this option unless the remote side is known to be down.
	ForceDelete bool `json:"force,omitempty"`
}
