/*
 *
 * Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// HostGroup hostgroup instance
type HostGroup struct {
	// A description for the hostgroup.
	Description string `json:"description,omitempty"`
	// Unique id of the hostgroup.
	ID string `json:"id,omitempty"`
	// The hostgroup name.
	Name string `json:"name,omitempty"`
	// Properties of a host.
	Hosts []Host `json:"hosts,omitempty"`
	// Connectivity type for hosts and host groups.
	HostConnectivity HostConnectivityEnum `json:"host_connectivity,omitempty"`
	// HostConnectivityL10n Localized message string corresponding to host_connectivity
	HostConnectivityL10n string `json:"host_connectivity_l10n,omitempty"`
	// MappedHostGroups Details about a configured host or host group attached to a volume.
	MappedHostGroups []MappedHostGroup `json:"mapped_host_groups,omitempty"`
	// HostVirtualVolumeMappings Virtual volume mapping details.
	HostVirtualVolumeMappings []HostVirtualVolumeMapping `json:"host_virtual_volume_mappings,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (h *HostGroup) Fields() []string {
	return []string{"*", "hosts(*)", "mapped_host_groups(*,volume(*))", "host_virtual_volume_mappings(*,virtual_volume(*))"}
}

// HostGroupCreate create hostgroup request
type HostGroupCreate struct {
	// The hostgroup name.
	Name string `json:"name,omitempty"`
	// A description for the hostgroup.
	Description string `json:"description,omitempty"`
	// The list of hosts to include in the host group.
	HostIDs []string `json:"host_ids,omitempty"`
}

// HostGroupModify modifies existing hostgroup
type HostGroupModify struct {
	// The hostgroup name.
	Name string `json:"name,omitempty"`
	// A description for the hostgroup.
	Description string `json:"description,omitempty"`
	// Connectivity type for hosts and host groups.
	HostConnectivity string `json:"host_connectivity,omitempty"`
	// List of hosts to be removed from the host group.
	RemoveHostIDs []string `json:"remove_host_ids,omitempty"`
	// List of hosts to be added to host group.
	AddHostIDs []string `json:"add_host_ids,omitempty"`
}

type MappedHostGroup struct {
	// Unique identifier of a mapping between a host and a volume.
	ID string `json:"id,omitempty"`
	// Unique identifier of a host attached to a volume.
	HostID string `json:"host_id,omitempty"`
	// Unique identifier of a host group attached to a volume.
	HostGroupID string `json:"host_group_id,omitempty"`
	// Unique identifier of the volume to which the host is attached.
	VolumeID string `json:"volume_id,omitempty"`
	// Details about a volume, including snapshots and clones of volumes.
	Volume Volume `json:"volume,omitempty"`
}

type HostVirtualVolumeMapping struct {
	// Unique identifier of a mapping between a host and a virtual volume.
	ID string `json:"id,omitempty"`
	// Unique identifier of a host attached to a volume.
	HostID string `json:"host_id,omitempty"`
	// Unique identifier of the virtual volume to which the host is attached.
	VirtualVolumeID string `json:"virtual_volume_id,omitempty"`
	// A virtual volume.
	VirtualVolume VirtualVolume `json:"virtual_volume,omitempty"`
}
