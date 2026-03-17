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

// CreateHostGroupParam contains parameters required
// to create host group
type CreateHostGroupParam struct {
	HostGroupID     string            `json:"hostGroupId"`
	HostIDs         []string          `json:"hostId"`
	HostFlags       *HostFlags        `json:"hostFlags,omitempty"`
	ExecutionOption string            `json:"executionOption"`
	NewHosts        []CreateHostParam `json:"new_hosts,omitempty"`
}

// HostGroup holds the information about a hostgroup
type HostGroup struct {
	HostGroupID        string        `json:"hostGroupId"`
	NumOfHosts         int64         `json:"num_of_hosts"`
	NumberMaskingViews int64         `json:"num_of_masking_views"`
	NumberInitiators   int64         `json:"num_of_initiators"`
	PortFlagsOverride  bool          `json:"port_flags_override"`
	ConsistentLun      bool          `json:"consistent_lun"`
	EnabledFlags       string        `json:"enabled_flags"`
	DisabledFlags      string        `json:"disabled_flags"`
	HostGroupType      string        `json:"type"`
	MaskingviewIDs     []string      `json:"maskingview"`
	Hosts              []HostSummary `json:"host"`
}

// HostSummary holds the information about hostIDs and its corresponding initiators associated with the hostgroup
type HostSummary struct {
	HostID     string   `json:"hostId"`
	Initiators []string `json:"initiator"`
}

// UpdateHostGroupParam contains action and option to update the hostGroup
type UpdateHostGroupParam struct {
	EditHostGroupAction *EditHostGroupActionParams `json:"editHostGroupActionParam"`
	ExecutionOption     string                     `json:"executionOption"`
}

// EditHostGroupActionParams holds the parameters of the hostGroup to modify
type EditHostGroupActionParams struct {
	SetHostGroupFlags    *SetHostFlags         `json:"setHostGroupFlagsParam,omitempty"`
	RenameHostGroupParam *RenameHostGroupParam `json:"renameHostGroupParam,omitempty"`
	RemoveHostParam      *EditHostsParam       `json:"removeHostParam,omitempty"`
	AddHostParam         *EditHostsParam       `json:"addHostParam,omitempty"`
}

// RenameHostGroupParam holds the new name for the host group
type RenameHostGroupParam struct {
	NewHostGroupName string `json:"new_host_group_name,omitempty"`
}

// EditHostsParam holds the list of hosts to be updated in the host group
type EditHostsParam struct {
	Host []string `json:"host"`
}

// HostGroupList : list of hostgroups
type HostGroupList struct {
	HostGroupIDs []string `json:"hostGroupId"`
}
