/*
 Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package v100

// StorageGroupIDList : list of sg's
type StorageGroupIDList struct {
	StorageGroupIDs []string `json:"storageGroupId"`
}

// StorageGroup holds all the fields of an SG
type StorageGroup struct {
	StorageGroupID        string                `json:"storageGroupId"`
	SLO                   string                `json:"slo"`
	ServiceLevel          string                `json:"service_level"`
	BaseSLOName           string                `json:"base_slo_name"`
	SRP                   string                `json:"srp"`
	Workload              string                `json:"workload"`
	SLOCompliance         string                `json:"slo_compliance"`
	NumOfVolumes          int                   `json:"num_of_vols"`
	NumOfChildSGs         int                   `json:"num_of_child_sgs"`
	NumOfParentSGs        int                   `json:"num_of_parent_sgs"`
	NumOfMaskingViews     int                   `json:"num_of_masking_views"`
	NumOfSnapshots        int                   `json:"num_of_snapshots"`
	NumOfSnapshotPolicies int                   `json:"num_of_snapshot_policies"`
	CapacityGB            float64               `json:"cap_gb"`
	DeviceEmulation       string                `json:"device_emulation"`
	Type                  string                `type:"type"`
	Unprotected           bool                  `type:"unprotected"`
	ChildStorageGroup     []string              `json:"child_storage_group"`
	ParentStorageGroup    []string              `json:"parent_storage_group"`
	MaskingView           []string              `json:"maskingview"`
	SnapshotPolicies      []string              `json:"snapshot_policies"`
	HostIOLimit           *SetHostIOLimitsParam `json:"hostIOLimit"`
	Compression           bool                  `json:"compression"`
	CompressionRatio      string                `json:"compressionRatio"`
	CompressionRatioToOne float64               `json:"compression_ratio_to_one"`
	VPSavedPercent        float64               `json:"vp_saved_percent"`
	Tags                  string                `json:"tags"`
	UUID                  string                `json:"uuid"`
	UnreducibleDataGB     float64               `json:"unreducible_data_gb"`
}

// StorageGroupResult holds result of an operation
type StorageGroupResult struct {
	StorageGroup []StorageGroup `json:"storageGroup"`
	Success      bool           `json:"success"`
	Message      string         `json:"message"`
}

// CreateStorageGroupParam : Payload for creating Storage Group
type CreateStorageGroupParam struct {
	ExecutionOption           string                      `json:"executionOption,omitempty"`
	StorageGroupID            string                      `json:"storageGroupId"`
	SnapshotPolicies          []string                    `json:"snapshot_policies"`
	SRPID                     string                      `json:"srpId,omitempty"`
	SLOBasedStorageGroupParam []SLOBasedStorageGroupParam `json:"sloBasedStorageGroupParam,omitempty"`
	Emulation                 string                      `json:"emulation,omitempty"`
}

// MergeStorageGroupParam : Payloads for updating Storage Group
type MergeStorageGroupParam struct {
	StorageGroupID string `json:"storageGroupId,omitempty"`
}

// SplitStorageGroupVolumesParam holds parameters to split
type SplitStorageGroupVolumesParam struct {
	VolumeIDs      []string `json:"volumeId,omitempty"`
	StorageGroupID string   `json:"storageGroupId,omitempty"`
	MaskingViewID  string   `json:"maskingViewId,omitempty"`
}

// SplitChildStorageGroupParam holds param to split
// child SG
type SplitChildStorageGroupParam struct {
	StorageGroupID string `json:"storageGroupId,omitempty"`
	MaskingViewID  string `json:"maskingViewId,omitempty"`
}

// MoveVolumeToStorageGroupParam stores parameters to
// move volumes to SG
type MoveVolumeToStorageGroupParam struct {
	VolumeIDs      []string `json:"volumeId,omitempty"`
	StorageGroupID string   `json:"storageGroupId,omitempty"`
	Force          bool     `json:"force,omitempty"`
}

// EditCompressionParam hold param to edit compression
// attribute with an SG
type EditCompressionParam struct {
	Compression *bool `json:"compression,omitempty"`
}

// SetHostIOLimitsParam holds param to set host IO limit
type SetHostIOLimitsParam struct {
	HostIOLimitMBSec    string `json:"host_io_limit_mb_sec,omitempty"`
	HostIOLimitIOSec    string `json:"host_io_limit_io_sec,omitempty"`
	DynamicDistribution string `json:"dynamicDistribution,omitempty"`
}

// RemoveVolumeParam holds volume ids to remove from SG
type RemoveVolumeParam struct {
	VolumeIDs             []string              `json:"volumeId,omitempty"`
	RemoteSymmSGInfoParam RemoteSymmSGInfoParam `json:"remoteSymmSGInfoParam"`
}

// AddExistingStorageGroupParam contains SG ids and compliance alert flag
type AddExistingStorageGroupParam struct {
	StorageGroupIDs        []string `json:"storageGroupId,omitempty"`
	EnableComplianceAlerts bool     `json:"enableComplianceAlerts,omitempty"`
}

// VolumeAttributeType : volume attributes for 9.1
type VolumeAttributeType struct {
	NumberOfVolumes  int                   `json:"num_of_vols,omitempty"`
	VolumeIdentifier *VolumeIdentifierType `json:"volumeIdentifier,omitempty"`
	CapacityUnit     string                `json:"capacityUnit"` // CAPACITY_UNIT_{TB,GB,MB,CYL}
	VolumeSize       string                `json:"volume_size"`
}

// SLOBasedStorageGroupParam holds parameters related to an SG and SLO
type SLOBasedStorageGroupParam struct {
	CustomCascadedStorageGroupID                   string                `json:"custom_cascaded_storageGroupId"`
	SnapshotPolicies                               []string              `json:"snapshot_policies"`
	SLOID                                          string                `json:"sloId,omitempty"`
	WorkloadSelection                              string                `json:"workloadSelection,omitempty"`
	VolumeAttributes                               []VolumeAttributeType `json:"volumeAttributes,omitempty"`
	AllocateCapacityForEachVol                     bool                  `json:"allocate_capacity_for_each_vol,omitempty"`
	PersistPrealloctedCapacityThroughReclaimOrCopy bool                  `json:"persist_preallocated_capacity_through_reclaim_or_copy,omitempty"`
	NoCompression                                  bool                  `json:"noCompression,omitempty"`
	EnableMobilityID                               bool                  `json:"enable_mobility_id"`
	SetHostIOLimitsParam                           *SetHostIOLimitsParam `json:"setHostIOLimitsParam,omitempty"`
}

// AddNewStorageGroupParam contains parameters required to add a
// new storage group
type AddNewStorageGroupParam struct {
	SRPID                     string                      `json:"srpId,omitempty"`
	SLOBasedStorageGroupParam []SLOBasedStorageGroupParam `json:"sloBasedStorageGroupParam,omitempty"`
	Emulation                 string                      `json:"emulation,omitempty"`
	EnableComplianceAlerts    bool                        `json:"enableComplianceAlerts,omitempty"`
}

// SpecificVolumeParam holds volume ids, volume attributes and RDF group num
type SpecificVolumeParam struct {
	VolumeIDs       []string            `json:"volumeId,omitempty"`
	VolumeAttribute VolumeAttributeType `json:"volumeAttribute,omitempty"`
	RDFGroupNumber  int                 `json:"rdfGroupNumber,omitempty"`
}

// AllVolumeParam contains volume attributes and RDF group number
type AllVolumeParam struct {
	VolumeAttribute VolumeAttributeType `json:"volumeAttribute,omitempty"`
	RDFGroupNumber  int                 `json:"rdfGroupNumber,omitempty"`
}

// ExpandVolumesParam holds parameters to expand volumes
type ExpandVolumesParam struct {
	SpecificVolumeParam SpecificVolumeParam `json:"specificVolumeParam,omitempty"`
	AllVolumeParam      AllVolumeParam      `json:"allVolumeParam,omitempty"`
}

// AddSpecificVolumeParam holds volume ids
type AddSpecificVolumeParam struct {
	VolumeIDs             []string              `json:"volumeId,omitempty"`
	RemoteSymmetrixSGInfo RemoteSymmSGInfoParam `json:"remoteSymmSGInfoParam"`
}

// AddVolumeParam holds number volumes to add and related param
type AddVolumeParam struct {
	VolumeAttributes      []VolumeAttributeType `json:"volumeAttributes,omitempty"`
	CreateNewVolumes      bool                  `json:"create_new_volumes"`
	Emulation             string                `json:"emulation,omitempty"`
	EnableMobilityID      bool                  `json:"enable_mobility_id"`
	VolumeIdentifier      *VolumeIdentifierType `json:"volumeIdentifier,omitempty"`
	RemoteSymmetrixSGInfo RemoteSymmSGInfoParam `json:"remoteSymmSGInfoParam"`
}

// ExpandStorageGroupParam holds params related to expanding size of an SG
type ExpandStorageGroupParam struct {
	AddExistingStorageGroupParam *AddExistingStorageGroupParam `json:"addExistingStorageGroupParam,omitempty"`
	AddNewStorageGroupParam      *AddNewStorageGroupParam      `json:"addNewStorageGroupParam,omitempty"`
	ExpandVolumesParams          *ExpandVolumesParam           `json:"expandVolumesParam,omitempty"`
	AddSpecificVolumeParam       *AddSpecificVolumeParam       `json:"addSpecificVolumeParam,omitempty"`
	AddVolumeParam               *AddVolumeParam               `json:"addVolumeParam,omitempty"`
}

// EditStorageGroupWorkloadParam holds selected work load
type EditStorageGroupWorkloadParam struct {
	WorkloadSelection string `json:"workloadSelection,omitempty,omitempty"`
}

// EditStorageGroupSLOParam hold param to change SLOs
type EditStorageGroupSLOParam struct {
	SLOID string `json:"sloId,omitempty"`
}

// EditStorageGroupSRPParam holds param to change SRPs
type EditStorageGroupSRPParam struct {
	SRPID string `json:"srpId,omitempty"`
}

// RemoveStorageGroupParam holds parameters to remove an SG
type RemoveStorageGroupParam struct {
	StorageGroupIDs []string `json:"storageGroupId,omitempty"`
	Force           bool     `json:"force,omitempty"`
}

// RenameStorageGroupParam holds new name of a storage group
type RenameStorageGroupParam struct {
	NewStorageGroupName string `json:"new_storage_Group_name,omitempty"`
}

// EditSnapshotPoliciesParam holds the updates for snapshotpolicies of the storageGroup
type EditSnapshotPoliciesParam struct {
	ResumeSnapshotPolicyParam       *SnapshotPolicies `json:"resume_snapshot_policy_param,omitempty"`
	SuspendSnapshotPolicyParam      *SnapshotPolicies `json:"suspend_snapshot_policy_param,omitempty"`
	DisassociateSnapshotPolicyParam *SnapshotPolicies `json:"disassociate_snapshot_policy_param,omitempty"`
	AssociateSnapshotPolicyParam    *SnapshotPolicies `json:"associate_snapshot_policy_param,omitempty"`
}

// SnapshotPolicies holds the list of snapshot policy names
type SnapshotPolicies struct {
	SnapshotPolicies []string `json:"snapshot_policies,omitempty"`
}

// EditStorageGroupActionParam holds parameters to modify an SG
type EditStorageGroupActionParam struct {
	MergeStorageGroupParam        *MergeStorageGroupParam        `json:"mergeStorageGroupParam,omitempty"`
	SplitStorageGroupVolumesParam *SplitStorageGroupVolumesParam `json:"splitStorageGroupVolumesParam,omitempty"`
	SplitChildStorageGroupParam   *SplitChildStorageGroupParam   `json:"splitChildStorageGroupParam,omitempty"`
	MoveVolumeToStorageGroupParam *MoveVolumeToStorageGroupParam `json:"moveVolumeToStorageGroupParam,omitempty"`
	EditCompressionParam          *EditCompressionParam          `json:"editCompressionParam,omitempty"`
	SetHostIOLimitsParam          *SetHostIOLimitsParam          `json:"setHostIOLimitsParam,omitempty"`
	RemoveVolumeParam             *RemoveVolumeParam             `json:"removeVolumeParam,omitempty"`
	ExpandStorageGroupParam       *ExpandStorageGroupParam       `json:"expandStorageGroupParam,omitempty"`
	EditStorageGroupWorkloadParam *EditStorageGroupWorkloadParam `json:"editStorageGroupWorkloadParam,omitempty"`
	EditStorageGroupSLOParam      *EditStorageGroupSLOParam      `json:"editStorageGroupSLOParam,omitempty"`
	EditStorageGroupSRPParam      *EditStorageGroupSRPParam      `json:"editStorageGroupSRPParam,omitempty"`
	RemoveStorageGroupParam       *RemoveStorageGroupParam       `json:"removeStorageGroupParam,omitempty"`
	RenameStorageGroupParam       *RenameStorageGroupParam       `json:"renameStorageGroupParam,omitempty"`
	EditSnapshotPoliciesParam     *EditSnapshotPoliciesParam     `json:"edit_snapshot_policies_param,omitempty"`
}

// ExecutionOptionSynchronous : execute tasks synchronously
const ExecutionOptionSynchronous = "SYNCHRONOUS"

// ExecutionOptionAsynchronous : execute tasks asynchronously
const ExecutionOptionAsynchronous = "ASYNCHRONOUS"

// UpdateStorageGroupPayload : updates SG rest paylod
type UpdateStorageGroupPayload struct {
	EditStorageGroupActionParam EditStorageGroupActionParam `json:"editStorageGroupActionParam"`
	// ExecutionOption "SYNCHRONOUS" or "ASYNCHRONOUS"
	ExecutionOption string `json:"executionOption"`
}

// UseExistingStorageGroupParam : use this sg ID
type UseExistingStorageGroupParam struct {
	StorageGroupID string `json:"storageGroupId,omitempty"`
}

// RemoveTagsParam holds array of tags to be removed
type RemoveTagsParam struct {
	TagName []string `json:"tag_name,omitempty"`
}

// AddTagsParam holds array of tags to be added
type AddTagsParam struct {
	TagName []string `json:"tag_name,omitempty"`
}

// TagManagementParam holds parameters to remove or add tags
type TagManagementParam struct {
	RemoveTagsParam *RemoveTagsParam `json:"removeTagsParam,omitempty"`
	AddTagsParam    *AddTagsParam    `json:"addTagsParam,omitempty"`
}

// RemoteSymmSGInfoParam have info about remote symmetrix Id's and storage groups
type RemoteSymmSGInfoParam struct {
	RemoteSymmetrix1ID  string   `json:"remote_symmetrix_1_id,omitempty"`
	RemoteSymmetrix1SGs []string `json:"remote_symmetrix_1_sgs,omitempty"`
	RemoteSymmetrix2ID  string   `json:"remote_symmetrix_2_id,omitempty"`
	RemoteSymmetrix2SGs []string `json:"remote_symmetrix_2_sgs,omitempty"`
	Force               bool     `json:"force,omitempty"`
}

// StorageGroupSnapshotPolicy holds storage group snapshot policy
type StorageGroupSnapshotPolicy struct {
	SymmetrixID           string `json:"symmetrixID,omitempty"`
	SnapshotPolicyID      string `json:"snapshot_policy_id,omitempty"`
	StorageGroupID        string `json:"storage_group_id,omitempty"`
	Compliance            string `json:"compliance,omitempty"`
	SnapshotsInTimeWindow int    `json:"snapshots_in_time_window,omitempty"`
	TotalSnapshots        int    `json:"total_snapshots,omitempty"`
	Suspended             bool   `json:"suspended,omitempty"`
}
