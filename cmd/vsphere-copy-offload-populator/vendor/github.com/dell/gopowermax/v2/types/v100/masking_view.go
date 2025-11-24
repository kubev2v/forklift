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

// MaskingViewList contains list of masking views
type MaskingViewList struct {
	MaskingViewIDs []string `json:"maskingViewId"`
}

// MaskingView holds masking view fields
type MaskingView struct {
	MaskingViewID  string `json:"maskingViewId"`
	HostID         string `json:"hostId"`
	HostGroupID    string `json:"hostGroupId"`
	PortGroupID    string `json:"portGroupId"`
	StorageGroupID string `json:"storageGroupId"`
}

// EditMaskingViewParam holds values to modify for masking view with execution option
type EditMaskingViewParam struct {
	EditMaskingViewActionParam EditMaskingViewActionParam `json:"editMaskingViewActionParam"`
	ExecutionOption            string                     `json:"executionOption"`
}

// EditMaskingViewActionParam holds values to modify for masking view
type EditMaskingViewActionParam struct {
	RenameMaskingViewParam RenameMaskingViewParam `json:"renameMaskingViewParam"`
}

// RenameMaskingViewParam holds the new name of masking view
type RenameMaskingViewParam struct {
	NewMaskingViewName string `json:"new_masking_view_name"`
}

// HostFlag holds the host flags
type HostFlag struct {
	Enabled  bool `json:"enabled"`
	Override bool `json:"override"`
}

// HostFlags holds additional host flags
type HostFlags struct {
	VolumeSetAddressing *HostFlag `json:"volume_set_addressing,omitempty"`
	DisableQResetOnUA   *HostFlag `json:"disable_q_reset_on_ua,omitempty"`
	EnvironSet          *HostFlag `json:"environ_set,omitempty"`
	AvoidResetBroadcast *HostFlag `json:"avoid_reset_broadcast,omitempty"`
	OpenVMS             *HostFlag `json:"openvms,omitempty"`
	SCSI3               *HostFlag `json:"scsi_3,omitempty"`
	Spc2ProtocolVersion *HostFlag `json:"spc2_protocol_version,omitempty"`
	SCSISupport1        *HostFlag `json:"scsi_support1,omitempty"`
	ConsistentLUN       bool      `json:"consistent_lun"`
}

// UseExistingHostGroupParam contains ID of the
// host group
type UseExistingHostGroupParam struct {
	HostGroupID string `json:"hostGroupId"`
}

// CreateHostParam contains input fields to
// create a host
type CreateHostParam struct {
	HostID          string     `json:"hostId"`
	InitiatorIDs    []string   `json:"initiatorId"`
	HostFlags       *HostFlags `json:"hostFlags,omitempty"`
	ExecutionOption string     `json:"executionOption"`
}

// ChangeInitiatorParam contains initiators
type ChangeInitiatorParam struct {
	Initiators []string `json:"initiator,omitempty"`
}

// RenameHostParam holds the new name
type RenameHostParam struct {
	NewHostName string `json:"new_host_name,omitempty"`
}

// SetHostFlags contains the host flags
type SetHostFlags struct {
	HostFlags *HostFlags `json:"hostFlags,omitempty"`
}

// EditHostParams holds the host flags to modify
type EditHostParams struct {
	SetHostFlags    *SetHostFlags    `json:"setHostFlagsParam,omitempty"`
	RenameHostParam *RenameHostParam `json:"renameHostParam,omitempty"`
}

// AddHostInitiators holds initiator parameter to add
type AddHostInitiators struct {
	AddInitiator *ChangeInitiatorParam `json:"addInitiatorParam,omitempty"`
}

// RemoveHostInitiators holds the initiator parameter to remove
type RemoveHostInitiators struct {
	RemoveInitiator *ChangeInitiatorParam `json:"removeInitiatorParam,omitempty"`
}

// UpdateHostParam contains action and option to update the host
type UpdateHostParam struct {
	EditHostAction  *EditHostParams `json:"editHostActionParam"`
	ExecutionOption string          `json:"executionOption"`
}

// UpdateHostAddInitiatorsParam contains action and option to update
// the host initiators
type UpdateHostAddInitiatorsParam struct {
	EditHostAction  *AddHostInitiators `json:"editHostActionParam"`
	ExecutionOption string             `json:"executionOption"`
}

// UpdateHostRemoveInititorsParam contains action and option to remove
// the host initiators
type UpdateHostRemoveInititorsParam struct {
	EditHostAction  *RemoveHostInitiators `json:"editHostActionParam"`
	ExecutionOption string                `json:"executionOption"`
}

// UseExistingHostParam contains host id to use
type UseExistingHostParam struct {
	HostID string `json:"hostId"`
}

// HostOrHostGroupSelection contains parameters to
// select a host or host group
type HostOrHostGroupSelection struct {
	CreateHostGroupParam      *CreateHostGroupParam      `json:"createHostGroupParam,omitempty"`
	UseExistingHostGroupParam *UseExistingHostGroupParam `json:"useExistingHostGroupParam,omitempty"`
	CreateHostParam           *CreateHostParam           `json:"createHostParam,omitempty"`
	UseExistingHostParam      *UseExistingHostParam      `json:"useExistingHostParam,omitempty"`
}

// SymmetrixPortKeyType contains the director id and port number
type SymmetrixPortKeyType struct {
	DirectorID string `json:"directorId,omitempty"`
	PortID     string `json:"portId,omitempty"`
}

// CreatePortGroupParam contains the port group id and port type
type CreatePortGroupParam struct {
	PortGroupID      string                 `json:"portGroupId,omitempty"`
	SymmetrixPortKey []SymmetrixPortKeyType `json:"symmetrixPortKey,omitempty"`
}

// UseExistingPortGroupParam contains the port group id
type UseExistingPortGroupParam struct {
	PortGroupID string `json:"portGroupId,omitempty"`
}

// PortGroupSelection contains parameters to select the port group
type PortGroupSelection struct {
	CreatePortGroupParam      *CreatePortGroupParam      `json:"createPortGroupParam,omitempty"`
	UseExistingPortGroupParam *UseExistingPortGroupParam `json:"useExistingPortGroupParam,omitempty"`
}

// StorageGroupSelection contains parameters to select storage group
type StorageGroupSelection struct {
	CreateStorageGroupParam      *CreateStorageGroupParam      `json:"createStorageGroupParam,omitempty"`
	UseExistingStorageGroupParam *UseExistingStorageGroupParam `json:"useExistingStorageGroupParam,omitempty"`
}

// MaskingViewCreateParam holds the parameters to create masking views
type MaskingViewCreateParam struct {
	MaskingViewID            string                    `json:"maskingViewId"`
	HostOrHostGroupSelection *HostOrHostGroupSelection `json:"hostOrHostGroupSelection,omitempty"`
	PortGroupSelection       *PortGroupSelection       `json:"portGroupSelection,omitempty"`
	StorageGroupSelection    *StorageGroupSelection    `json:"storageGroupSelection,omitempty"`
	EnableComplianceAlerts   bool                      `json:"enableComplianceAlerts,omitempty"`
	ExecutionOption          string                    `json:"executionOption,omitempty"`
}

// MaskingViewConnection is a connection entry for the massking view associating
// a volume with the HostLUNAddress, the InitiatID and DirectorPort used for the
// path, and other attributes.
type MaskingViewConnection struct {
	VolumeID       string `json:"volumeId"`
	HostLUNAddress string `json:"host_lun_address"`
	CapacityGB     string `json:"cap_gb"`
	InitiatorID    string `json:"initiatorId"`
	Alias          string `json:"alias"`
	DirectorPort   string `json:"dir_port"`
	LoggedIn       bool   `json:"logged_in"`
	OnFabric       bool   `json:"on_fabric"`
}

// MaskingViewConnectionsResult is the result structure for .../maskingview/{id}/connections
type MaskingViewConnectionsResult struct {
	MaskingViewConnections []*MaskingViewConnection `json:"maskingViewConnection"`
}
