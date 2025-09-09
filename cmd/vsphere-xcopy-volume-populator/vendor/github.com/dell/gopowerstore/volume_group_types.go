/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *      http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

// VGPlacementRuleEnum - This is set during creation, and determines resource balancer recommendations.
type VGPlacementRuleEnum string

const (
	// VGPlacementRuleEnumSameAppliance - All the members of the group should be on the same appliance in the cluster.
	VGPlacementRuleEnumSameAppliance VolumeStateEnum = "Same_Appliance"
	// VGPlacementRuleEnumNoPreference - The volumes can be placed on any of the appliances in the cluster.
	VGPlacementRuleEnumNoPreference VolumeStateEnum = "No_Preference"
)

// VolumeGroupCreate create volume group request
type VolumeGroupCreate struct {
	// Unique name for the volume group.
	// The name should contain no special HTTP characters and no unprintable characters.
	// Although the case of the name provided is reserved, uniqueness check is case-insensitive,
	// so the same name in two different cases is not considered unique.
	Name string `json:"name"`
	// Description for the volume group. The description should not be more than 256
	// characters long and should not have any unprintable characters.
	Description string `json:"description,omitempty"`
	// Unique identifier of an optional protection policy to assign to the volume group.
	ProtectionPolicyID string `json:"protection_policy_id,omitempty"`
	// For a primary or a clone volume group, this property determines whether snapshot sets of the group will be write order consistent.
	// If not specified, this parameter defaults to true in PowerStore API.
	IsWriteOrderConsistent *bool `json:"is_write_order_consistent,omitempty"`
	// A list of identifiers of existing volumes that should be added to the volume group.
	// All the volumes must be on the same Cyclone appliance and should not be part of another volume group.
	// If a list of volumes is not specified or if the specified list is empty, an
	// empty volume group of type Volume will be created.
	VolumeIDs []string `json:"volume_ids,omitempty"`
}

// VolumeGroup details about a volume groups.
type VolumeGroup struct {
	// Unique identifier of the volume group.
	ID string `json:"id,omitempty"`
	// Name of the volume group.
	// This property supports case-insensitive filtering
	Name string `json:"name,omitempty"`
	// Description for the volume group.
	Description string `json:"description,omitempty"`
	// Unique identifier of the protection policy assigned to the volume.
	ProtectionPolicyID string `json:"protection_policy_id,omitempty"`
	// For a primary or a clone volume group, this property determines whether snapshot sets of the group will be write order consistent.
	IsWriteOrderConsistent bool `json:"is_write_order_consistent,omitempty"`
	// Volumes provides list of volumes associated to the volume group
	Volumes []Volume `json:"volumes"`
	// ProtectionPolicy provides snapshot details of the volume or volumeGroup
	ProtectionPolicy ProtectionPolicy `json:"protection_policy"`
	// CreationTimeStamp provides volume group creation time
	CreationTimeStamp string `json:"creation_timestamp,omitempty"`
	// IsReplicationDestination indicates whether this volume group is a replication destination.
	IsReplicationDestination bool `json:"is_replication_destination,omitempty"`
	// is_importing indicates whether the volume group is being imported.
	IsImporting bool `json:"is_importing,omitempty"`
	// TypeL10 localized message string corresponding to type
	TypeL10 string `json:"type_l10n,omitempty"`
	// IsProtectable is a derived field that is set internally.
	IsProtectable bool `json:"is_protectable,omitempty"`
	// Unique identifier of the migration session assigned to the volume group when it is part of a migration activity.
	MigrationSessionID string `json:"migration_session_id,omitempty"`
	// This is set during creation, and determines resource balancer recommendations.
	PlacementRule VGPlacementRuleEnum `json:"placement_rule,omitempty"`
	// Type of volume.
	Type VolumeTypeEnum `json:"type,omitempty"`
	// Protection data associated with a resource.
	ProtectionData ProtectionData `json:"protection_data,omitempty"`
	// A list of locations. The list of locations includes the move to the current appliance.
	LocationHistory []LocationHistory `json:"location_history,omitempty"`
	//  This resource type has queriable associations from virtual_volume, volume, volume_group, replication_session
	MigrationSession MigrationSession `json:"migration_session,omitempty"`
	// Unique identifier of the replication session assigned to the volume group if it has been configured as a metro volume group between two PowerStore clusters.
	MetroReplicationSessionID string `json:"metro_replication_session_id,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (v *VolumeGroup) Fields() []string {
	return []string{"*", "volumes(*)", "protection_policy(*)", "protection_data", "location_history", "migration_session(*)"}
}

type VolumeGroups struct {
	VolumeGroup []VolumeGroup `json:"volume_groups,omitempty"`
}

type VolumeGroupMembers struct {
	VolumeIDs []string `json:"volume_ids"`
}

// VolumeGroupModify modifies existing Volume Group
type VolumeGroupModify struct {
	ProtectionPolicyID     string  `json:"protection_policy_id"` // empty to unassign
	Description            string  `json:"description"`
	Name                   string  `json:"name,omitempty"`
	IsWriteOrderConsistent *bool   `json:"is_write_order_consistent,omitempty"`
	ExpirationTimestamp    *string `json:"expiration_timestamp,omitempty"`
}

// VolumeGroupSnapshotModify modifies existing Volume Group Snapshot Similar to volume group modify without protection policy since this is an invalid field for VolumeGroupSnapshot
type VolumeGroupSnapshotModify struct {
	Description            string  `json:"description"`
	Name                   string  `json:"name,omitempty"`
	IsWriteOrderConsistent *bool   `json:"is_write_order_consistent,omitempty"`
	ExpirationTimestamp    *string `json:"expiration_timestamp,omitempty"`
}

type VolumeGroupChangePolicy struct {
	ProtectionPolicyID string `json:"protection_policy_id"`
}

// VolumeGroupSnapshotCreate create volume group snapshot request
type VolumeGroupSnapshotCreate struct {
	// Unique name for the volume group.
	Name string `json:"name"`
	// Optional description
	Description string `json:"description,omitempty"`
	// ExpirationTimestamp provides volume group creation time
	ExpirationTimestamp string `json:"expiration_timestamp,omitempty"`
}

// EndMetroVolumeGroupOptions provides options for deleting the remote volume group and forcing the deletion.
type EndMetroVolumeGroupOptions struct {
	// DeleteRemoteVolumeGroup specifies whether or not to delete the remote volume group when ending the metro session.
	DeleteRemoteVolumeGroup bool `json:"delete_remote_volume_group,omitempty"`
	// ForceDelete specifies if the Metro volume group should be forcefully deleted.
	// If the force option is specified, any errors returned while attempting to tear down the remote side of the
	// metro session will be ignored and the remote side may be left in an indeterminate state.
	// If any errors occur on the local side the operation can still fail.
	// It is not recommended to use this option unless the remote side is known to be down.
	ForceDelete bool `json:"force,omitempty"`
}
