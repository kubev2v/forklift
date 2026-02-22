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
	ID                   string           `json:"id,omitempty"`
	Type                 string           `json:"type,omitempty"`
	System               SystemInfo       `json:"system,omitempty"`
	Identifier           string           `json:"identifier,omitempty"`
	StorageGroups        []StorageGroupID `json:"storage_groups,omitempty"`
	MaskingViews         []MaskingViewID  `json:"masking_views,omitempty"`
	CapCyl               float64          `json:"cap_cyl,omitempty"`
	VolumeHostPaths      []VolumeHostPath `json:"volume_host_paths,omitempty"`
	NumberOfMaskingViews int              `json:"num_of_masking_views,omitempty"`
	SRP                  Srp              `json:"srp,omitempty"`
}

type VolumeHostPath struct {
	ID string `json:"id,omitempty"`
}

type Srp struct {
	ID string `json:"id,omitempty"`
}

// Volumev1 : simplified volume information
type Volumev1 struct {
	Volumes []VolumeEnhanced `json:"volumes,omitempty"`
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
