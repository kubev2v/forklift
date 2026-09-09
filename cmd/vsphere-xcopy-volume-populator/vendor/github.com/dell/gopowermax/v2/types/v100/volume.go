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

// Following structures are to in/out cast the Unisphere rest payload

// VolumeIDList : list of volume ids
type VolumeIDList struct {
	VolumeIDs string `json:"volumeId"`
}

// VolumeResultList : volume list resulted
type VolumeResultList struct {
	VolumeList []VolumeIDList `json:"result"`
	From       int            `json:"from"`
	To         int            `json:"to"`
}

// VolumeIterator : holds the iterator of resultant volume list
type VolumeIterator struct {
	ResultList     VolumeResultList `json:"resultList"`
	ID             string           `json:"id"`
	Count          int              `json:"count"`
	ExpirationTime int64            `json:"expirationTime"` // What units is ExpirationTime in?
	MaxPageSize    int              `json:"maxPageSize"`
	WarningMessage string           `json:"warningMessage"`
}

// Volume : information about a volume
type Volume struct {
	VolumeID              string                 `json:"volumeId"`
	Type                  string                 `json:"type"`
	Emulation             string                 `json:"emulation"`
	SSID                  string                 `json:"ssid"`
	AllocatedPercent      int                    `json:"allocated_percent"`
	CapacityGB            float64                `json:"cap_gb"`
	FloatCapacityMB       float64                `json:"cap_mb"`
	CapacityCYL           int                    `json:"cap_cyl"`
	Status                string                 `json:"status"`
	Reserved              bool                   `json:"reserved"`
	Pinned                bool                   `json:"pinned"`
	PhysicalName          string                 `json:"physical_name"`
	VolumeIdentifier      string                 `json:"volume_identifier"`
	WWN                   string                 `json:"wwn"`
	Encapsulated          bool                   `json:"encapsulated"`
	NumberOfStorageGroups int                    `json:"num_of_storage_groups"`
	NumberOfFrontEndPaths int                    `json:"num_of_front_end_paths"`
	StorageGroupIDList    []string               `json:"storageGroupId"`
	RDFGroupIDList        []RDFGroupID           `json:"rdfGroupId"`
	SymmetrixPortKey      []SymmetrixPortKeyType `json:"symmetrixPortKey"`
	SnapSource            bool                   `json:"snapvx_source"`
	SnapTarget            bool                   `json:"snapvx_target"`
	CUImageBaseAddress    string                 `json:"cu_image_base_address"`
	HasEffectiveWWN       bool                   `json:"has_effective_wwn"`
	EffectiveWWN          string                 `json:"effective_wwn"`
	EncapsulatedWWN       string                 `json:"encapsulated_wwn"`
	OracleInstanceName    string                 `json:"oracle_instance_name"`
	MobilityIDEnabled     bool                   `json:"mobility_id_enabled"`
	StorageGroups         []StorageGroupName     `json:"storage_groups"`
	UnreducibleDataGB     float64                `json:"unreducible_data_gb"`
	NGUID                 string                 `json:"nguid"`
}

// SystemInfo : simplified system information
type SystemInfo struct {
	ID string `json:"id"`
}

// VolumeEnhanced : simplified volume information
type VolumeEnhanced struct {
	ID                      string           `json:"id,omitempty"`
	Type                    string           `json:"type,omitempty"`
	System                  SystemInfo       `json:"system,omitempty"`
	Identifier              string           `json:"identifier,omitempty"`
	StorageGroups           []StorageGroupID `json:"storage_groups,omitempty"`
	MaskingViews            []MaskingViewID  `json:"masking_views,omitempty"`
	CapCyl                  float64          `json:"cap_cyl,omitempty"`
	CapGB                   float64          `json:"cap_gb,omitempty"`
	EffectiveUsedCapacityGB float64          `json:"effective_used_capacity_gb,omitempty"`
	VolumeHostPaths         []VolumeHostPath `json:"volume_host_paths,omitempty"`
	NumberOfMaskingViews    int              `json:"num_of_masking_views,omitempty"`
	SRP                     Srp              `json:"srp,omitempty"`
}

type VolumeHostPath struct {
	ID string `json:"id,omitempty"`
}

type Srp struct {
	ID string `json:"id,omitempty"`
}

// Volumev1 : simplified volume information
type Volumev1 struct {
	Volumes      []VolumeEnhanced `json:"volumes,omitempty"`
	VolumePaging VolumePaging     `json:"paging,omitempty"`
}

type VolumePaging struct {
	ResumeToken        string `json:"resume_token,omitempty"`
	TotalInstances     int    `json:"total_instances,omitempty"`
	RemainingInstances int    `json:"remaining_instances,omitempty"`
}

// StorageGroupName holds group name in which volume exists
type StorageGroupID struct {
	StorageGroupID string `json:"id"`
}

// MaskingViewID holds group name in which volume exists
type MaskingViewID struct {
	MaskingViewID string `json:"id"`
}

// StorageGroupName holds group name in which volume exists
type StorageGroupName struct {
	StorageGroupName       string `json:"storage_group_name"`
	ParentStorageGroupName string `json:"parent_storage_group_name"`
}

// RDFGroupID contains the group number and label
type RDFGroupID struct {
	RDFGroupNumber int    `json:"rdf_group_number"`
	Label          string `json:"label"`
}

// FreeVolumeParam : boolean value representing data to be freed
type FreeVolumeParam struct {
	FreeVolume bool `json:"free_volume"`
}

// ExpandVolumeParam : attributes to expand a volume
type ExpandVolumeParam struct {
	VolumeAttribute VolumeAttributeType `json:"volumeAttribute"`
	RDFGroupNumber  int                 `json:"rdfGroupNumber,omitempty"`
}

// ModifyVolumeIdentifierParam : volume identifier to modify the volume information
type ModifyVolumeIdentifierParam struct {
	VolumeIdentifier VolumeIdentifierType `json:"volumeIdentifier"`
}

// EnableMobilityIDParam has mobility ID for a volume
type EnableMobilityIDParam struct {
	EnableMobilityID bool `json:"enable_mobility_id"`
}

// EditVolumeActionParam : action information to edit volume
type EditVolumeActionParam struct {
	EnableMobilityIDParam       *EnableMobilityIDParam       `json:"enable_mobility_id_param"`
	FreeVolumeParam             *FreeVolumeParam             `json:"freeVolumeParam,omitempty"`
	ExpandVolumeParam           *ExpandVolumeParam           `json:"expandVolumeParam,omitempty"`
	ModifyVolumeIdentifierParam *ModifyVolumeIdentifierParam `json:"modifyVolumeIdentifierParam,omitempty"`
}

// EditVolumeParam : parameters required to edit volume information
type EditVolumeParam struct {
	EditVolumeActionParam EditVolumeActionParam `json:"editVolumeActionParam"`
	ExecutionOption       string                `json:"executionOption"`
}

type CreateVolumesRequest struct {
	Volumes         []VolumeRequestParam `json:"volumes"`
	RequestID       string               `json:"request_id,omitempty"`
	ResponseSelect  string               `json:"response_select,omitempty"`
	ExecutionOption string               `json:"executionOption,omitempty"` // SYNCHRONOUS / ASYNCHRONOUS
}

// VolumeRequestParam represents per-volume request element in "volumes"
type VolumeRequestParam struct {
	// Existing volume selector (optional): provide either id or identifier
	Volume *ExistingVolumeRequestParam `json:"volume,omitempty"`
	// Create new selector (optional): choose exactly one method inside CreateVolumeParam
	CreateNew *CreateVolumeParam `json:"create_new,omitempty"`
	// Actions to apply to the resolved volume (existing or newly created)
	Actions *VolumeRequestParamActions `json:"actions,omitempty"`
	// Per-item selection of returned attributes
	ResponseSelect string `json:"response_select,omitempty"`
	// Optional per-item request identifier
	RequestID string `json:"request_id,omitempty"`
}

// ExistingVolumeRequestParam identifies an existing volume by ID or identifier.
type ExistingVolumeRequestParam struct {
	Identifier string `json:"identifier,omitempty"`
	ID         string `json:"id,omitempty"`
}

// CreateVolumeParam defines how to create a new volume.
type CreateVolumeParam struct {
	CreateNewFromSnapshot   *CreateNewFromSnapshot   `json:"create_new_from_snapshot,omitempty"`
	CreateNewFromAttributes *CreateNewFromAttributes `json:"create_new_from_attributes,omitempty"`
	PrecheckSrpCapacity     *ValidationSrpAction     `json:"precheck_srp_capacity,omitempty"`
}

// CreateNewFromSnapshot creates a new volume from a snapshot.
// When NewVolumeAttributes is set, the new volume is created at the specified
// size instead of inheriting the snapshot source volume size.
type CreateNewFromSnapshot struct {
	Snapshot            SnapshotRequestParam     `json:"snapshot"`
	NewVolumeAttributes *CreateNewFromAttributes `json:"new_volume_attributes,omitempty"`
}

// SnapshotRequestParam identifies a snapshot.
type SnapshotRequestParam struct {
	ID string `json:"id"`
}

// CreateNewFromAttributes creates a new volume with a requested capacity.
type CreateNewFromAttributes struct {
	CapacityUnit string  `json:"capacity_unit"` // CYL, MB, GB, TB
	VolumeSize   float64 `json:"volume_size"`   // number (double) in spec
}

// ValidationSrpAction validates SRP capacity before creation.
type ValidationSrpAction struct {
	SRP VolumeSrpParam `json:"srp"`
}

// VolumeSrpParam identifies an SRP.
type VolumeSrpParam struct {
	ID string `json:"id"`
}

// VolumeRequestParamActions contains actions applied to new/existing volumes.
type VolumeRequestParamActions struct {
	ManageVolumeStorageGroup *ManageVolumeStorageGroupAction `json:"manage_volume_storage_group,omitempty"`
	ManageIdentifier         *ManageIdentifierAction         `json:"manage_identifier,omitempty"`
	ManageReplication        *ManageReplicationAction        `json:"manage_replication,omitempty"`
}

// ManageVolumeStorageGroupAction adds/removes the volume to/from a storage group.
type ManageVolumeStorageGroupAction struct {
	Action           string                  `json:"action"` // ADD/REMOVE (spec) but some systems accept Add/Remove
	StorageGroup     VolumeStorageGroupParam `json:"storage_group"`
	EnableMobilityID *bool                   `json:"enable_mobility_id,omitempty"`
}

// VolumeStorageGroupParam identifies a storage group (and optional create-only fields).
type VolumeStorageGroupParam struct {
	ID              string                   `json:"id"`
	SRP             *VolumeSrpParam          `json:"srp,omitempty"`
	ServiceLevel    *VolumeServiceLevelParam `json:"service_level,omitempty"`
	HostIOLimitInfo *HostIOLimitInfo         `json:"host_io_limit_info,omitempty"`
}

// VolumeServiceLevelParam identifies a service level.
type VolumeServiceLevelParam struct {
	ID string `json:"id"`
}

// ManageIdentifierAction sets/unsets an identifier on the volume.
type ManageIdentifierAction struct {
	Action             string `json:"action"` // Set / Unset
	Identifier         string `json:"identifier,omitempty"`
	SkipDuplicateCheck *bool  `json:"skip_duplicate_check,omitempty"`
}

// ManageReplicationAction models local replication operations (CopyFrom/CopyTo).
type ManageReplicationAction struct {
	Local *LocalReplicationAction `json:"local,omitempty"`
}

// LocalReplicationAction defines local replication operation and source/target volume.
type LocalReplicationAction struct {
	Action             string                     `json:"action"` // CopyFrom / CopyTo
	Volume             ExistingVolumeRequestParam `json:"volume"`
	Copy               *bool                      `json:"copy,omitempty"`
	Establish          *bool                      `json:"establish,omitempty"`
	EstablishTerminate *bool                      `json:"establish_terminate,omitempty"`
}

// CreateVolumesResponse represents the response for the enhanced Create Volume API.
type CreateVolumesResponse struct {
	HTTPStatusCode int                  `json:"http_status_code"`
	Summary        ResponseSummary      `json:"summary"`
	Results        CreateVolumesResults `json:"results"`
}

// ResponseSummary provides a high-level summary of the request outcome.
type ResponseSummary struct {
	Total              int `json:"total"`
	PartiallySucceeded int `json:"partially_succeeded"`
	Succeeded          int `json:"succeeded"`
	Failed             int `json:"failed"`
	NotRun             int `json:"not_run"`
	Rejected           int `json:"rejected"`
}
type CreateVolumesResults struct {
	Result []CreateVolumeResponseItem `json:"result"`
}

// CreateVolumeResponseItem represents the response for a single volume request.
// In failure cases, Volume and StorageGroup may be absent.
type CreateVolumeResponseItem struct {
	Volume       *VolumeRefResponse       `json:"volume,omitempty"`
	StorageGroup *StorageGroupRefResponse `json:"storage_group,omitempty"`
	Status       string                   `json:"status"` // success | failed
	Messages     *ResponseMessages        `json:"messages,omitempty"`
	Steps        []ResponseStep           `json:"steps,omitempty"`
	RequestID    string                   `json:"request_id,omitempty"`
	ResourceID   string                   `json:"resource_id,omitempty"`
}
type VolumeRefResponse struct {
	ID            string           `json:"id,omitempty"`
	Identifier    string           `json:"identifier,omitempty"`
	CapCyl        float64          `json:"cap_cyl,omitempty"`
	StorageGroups []StorageGroupID `json:"storage_groups,omitempty"`
}
type StorageGroupRefResponse struct {
	ID           string `json:"id,omitempty"`
	NumOfVolumes int    `json:"num_of_volumes,omitempty"`
}

// ResponseStep represents a system execution step.
type ResponseStep struct {
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
	Result      string `json:"result,omitempty"`
}

// ResponseMessages wraps messages returned by the system.
// JSON shape: { "messages": { "message": [ ... ] } }
type ResponseMessages struct {
	Message []ResponseMessage `json:"message,omitempty"`
}

// ResponseMessage represents an error, warning, or informational message.
type ResponseMessage struct {
	Code        string `json:"code,omitempty"`
	TimestampMS int64  `json:"timestamp_ms,omitempty"`
	Severity    string `json:"severity,omitempty"` // Error | Warning | Info
	Message     string `json:"message,omitempty"`
}
