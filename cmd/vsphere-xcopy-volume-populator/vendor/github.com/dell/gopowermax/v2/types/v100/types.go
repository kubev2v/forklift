/*
 Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

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

import (
	"strings"
)

// Error : contains fields to report rest interface errors
type Error struct {
	Message        string `json:"message"`
	HTTPStatusCode int    `json:"httpStatusCode"`
	ErrorCode      int    `json:"errorCode"`
}

func (e Error) Error() string {
	return e.Message
}

// Version : /unixmax/restapi/system/version
type Version struct {
	Version string `json:"version"`
}

// SymmetrixIDList : contains list of symIDs
type SymmetrixIDList struct {
	SymmetrixIDs []string `json:"symmetrixId"`
}

// Symmetrix : information about a Symmetrix system
type Symmetrix struct {
	SymmetrixID              string                `json:"symmetrixId"`
	DellServiceTag           string                `json:"dell_service_tag"`
	DeviceCount              int                   `json:"device_count"`
	Ucode                    string                `json:"ucode"`
	UcodeDate                string                `json:"ucode_date"`
	Model                    string                `json:"model"`
	Local                    bool                  `json:"local"`
	AllFlash                 bool                  `json:"all_flash"`
	DisplayName              string                `json:"display_name"`
	DiskCount                int                   `json:"disk_count"`
	CacheSizeMB              int                   `json:"cache_size_mb"`
	DataEncryption           string                `json:"data_encryption"`
	FEDirCount               int                   `json:"fe_dir_count"`
	BEDirCount               int                   `json:"be_dir_count"`
	RDFDirCount              int                   `json:"rdf_dir_count"`
	MaxHyperPerDisk          int                   `json:"max_hyper_per_disk"`
	VCMState                 string                `json:"vcm_state"`
	VCMDBState               string                `json:"vcmdb_state"`
	ReliabilityState         string                `json:"reliability_state"`
	UcodeRegisteredBuild     int                   `json:"ucode_registered_build"`
	Microcode                string                `json:"microcode"`
	MicrocodeDate            string                `json:"microcode_date"`
	MicrocodeRegisteredBuild int                   `json:"microcode_registered_build"`
	MicrocodePackageVersion  string                `json:"microcode_package_version"`
	SystemSizedProperty      []SystemSizedProperty `json:"system_sized_property"`
}

// SystemSizedProperty contains information about size data
type SystemSizedProperty struct {
	SRPName                    string `json:"srp_name"`
	SizedFBADataReductionRatio string `json:"sized_fba_data_reduction_ratio"`
	SizedCKDDataReductionRatio string `json:"sized_ckd_data_reduction_ratio"`
	SizedFBACapacityTB         int    `json:"sized_fba_capacity_tb"`
	SizedCKDCapacityTB         int    `json:"sized_ckd_capacity_tb"`
	SizedFBAReduciblePercent   int    `json:"sized_fba_reducible_percent"`
	SizedCKDReduciblePercent   int    `json:"sized_ckd_reducible_percent"`
}

// StoragePoolList : list of storage pools in the system
type StoragePoolList struct {
	StoragePoolIDs []string `json:"srpID"`
}

// StoragePool : information about a storage pool
type StoragePool struct {
	StoragePoolID        string         `json:"srpId"`
	DiskGrouCount        int            `json:"num_of_disk_groups"`
	Description          string         `json:"description"`
	Emulation            string         `json:"emulation"`
	CompressionState     string         `json:"compression_state"`
	EffectiveUsedCapPerc int            `json:"effective_used_capacity_percent"`
	ReservedCapPerc      int            `json:"reserved_cap_percent"`
	SrdfDseAllocCap      float64        `json:"total_srdf_dse_allocated_cap_gb"`
	RdfaDse              bool           `json:"rdfa_dse"`
	ReliabilityState     string         `json:"reliability_state"`
	DiskGroupIDs         []string       `json:"diskGroupId"`
	ExternalCap          float64        `json:"external_capacity_gb"`
	SrpCap               *SrpCap        `json:"srp_capacity,omitempty"`
	FbaCap               *FbaCap        `json:"fba_srp_capacity,omitempty"`
	CkdCap               *CkdCap        `json:"ckd_srp_capacity,omitempty"`
	SrpEfficiency        *SrpEfficiency `json:"srp_efficiency"`
	ServiceLevels        []string       `json:"service_levels"`
}

// FbaCap FBA storage pool capacity
type FbaCap struct {
	Provisioned *Provisioned `json:"provisioned"`
}

// CkdCap CKD storage pool capacity
type CkdCap struct {
	Provisioned *Provisioned `json:"provisioned"`
}

type Provisioned struct {
	UsableUsedInTB float64 `json:"used_tb"`
	UsableTotInTB  float64 `json:"effective_capacity_tb"`
	//	EffectiveUsedCapacityPercent float64 `json:"provisioned_percent"`
}

// SrpCap : capacity of an SRP
type SrpCap struct {
	SubAllocCapInTB              float64 `json:"subscribed_allocated_tb"`
	SubTotInTB                   float64 `json:"subscribed_total_tb"`
	SnapModInTB                  float64 `json:"snapshot_modified_tb"`
	SnapTotInTB                  float64 `json:"snapshot_total_tb"`
	UsableUsedInTB               float64 `json:"usable_used_tb"`
	UsableTotInTB                float64 `json:"usable_total_tb"`
	EffectiveUsedCapacityPercent int     `json:"effective_used_capacity_percent"`
}

// SrpEfficiency : efficiency attributes of an SRP
type SrpEfficiency struct {
	EfficiencyRatioToOne     float32 `json:"overall_efficiency_ratio_to_one"`
	DataReductionRatioToOne  float32 `json:"data_reduction_ratio_to_one"`
	DataReductionEnabledPerc float32 `json:"data_reduction_enabled_percent"`
	VirtProvSavingRatioToOne float32 `json:"virtual_provisioning_savings_ratio_to_one"`
	SanpSavingRatioToOne     float32 `json:"snapshot_savings_ratio_to_one"`
}

// constants of storage units
const (
	CapacityUnitTb  = "TB"
	CapacityUnitGb  = "GB"
	CapacityUnitMb  = "MB"
	CapacityUnitCyl = "CYL"
)

// VolumeIdentifierType : volume identifier
type VolumeIdentifierType struct {
	VolumeIdentifierChoice string `json:"volumeIdentifierChoice,omitempty"`
	IdentifierName         string `json:"identifier_name,omitempty"`
	AppendNumber           string `json:"append_number,omitempty"`
}

// Link : key and URI
type Link struct {
	Key string   `json:"key"`
	URI []string `json:"uris"`
}

// Task : holds execution order with a description
type Task struct {
	ExecutionOrder int    `json:"execution_order"`
	Description    string `json:"description"`
}

// constants
const (
	JobStatusUnscheduled = "UNSCHEDULED"
	JobStatusScheduled   = "SCHEDULED"
	JobStatusSucceeded   = "SUCCEEDED"
	JobStatusFailed      = "FAILED"
	JobStatusRunning     = "RUNNING"
)

// JobIDList : list of Job ids
type JobIDList struct {
	JobIDs []string `json:"jobId"`
}

// Job : information about a job
type Job struct {
	JobID                        string `json:"jobId"`
	Name                         string `json:"name"`
	SymmetrixID                  string `json:"symmetrixId"`
	Status                       string `json:"status"`
	Username                     string `json:"username"`
	LastModifiedDate             string `json:"last_modified_date"`
	LastModifiedDateMilliseconds int64  `json:"last_modified_date_milliseconds"`
	ScheduledDate                string `json:"scheduled_date"`
	ScheduledDateMilliseconds    int64  `json:"scheduled_date_milliseconds"`
	CompletedDate                string `json:"completed_date"`
	CompletedDateMilliseconds    int64  `json:"completed_date_milliseconds"`
	Tasks                        []Task `json:"task"`
	ResourceLink                 string `json:"resourceLink"`
	Result                       string `json:"result"`
	Links                        []Link `json:"links"`
}

// GetJobResource parses the Resource link and returns three things:
// The 1) the symmetrixID, 2) the resource type (e.g.) volume, and 3) the resourceID
// If the Resource Link cannot be parsed, empty strings are returned.
func (j *Job) GetJobResource() (string, string, string) {
	if j.ResourceLink == "" {
		return "", "", ""
	}
	parts := strings.Split(j.ResourceLink, "/")
	nparts := len(parts)
	if nparts < 3 {
		return "", "", ""
	}
	return parts[nparts-3], parts[nparts-2], parts[nparts-1]
}

// PortGroupList : list of port groups
type PortGroupList struct {
	PortGroupIDs []string `json:"portGroupId"`
}

// PortKey : combination of a port and a key
type PortKey struct {
	DirectorID string `json:"directorId"`
	PortID     string `json:"portId"`
}

// PortGroup : Information about a port group
type PortGroup struct {
	PortGroupID        string    `json:"portGroupId"`
	SymmetrixPortKey   []PortKey `json:"symmetrixPortKey"`
	NumberPorts        int64     `json:"num_of_ports"`
	NumberMaskingViews int64     `json:"num_of_masking_views"`
	PortGroupType      string    `json:"type"`
	MaskingView        []string  `json:"maskingview"`
	TestID             string    `json:"testId"`
	PortGroupProtocol  string    `json:"port_group_protocol"`
}

// CreatePortGroupParams - Input params for creating port groups
type CreatePortGroupParams struct {
	PortGroupID       string    `json:"portGroupId"`
	SymmetrixPortKey  []PortKey `json:"symmetrixPortKey"`
	ExecutionOption   string    `json:"executionOption"`
	PortGroupProtocol string    `json:"port_group_protocol,omitempty"`
}

// InitiatorList : list of initiators
type InitiatorList struct {
	InitiatorIDs []string `json:"initiatorId"`
}

// Initiator : Information about an initiator
type Initiator struct {
	InitiatorID          string    `json:"initiatorId"`
	SymmetrixPortKey     []PortKey `json:"symmetrixPortKey"`
	Alias                string    `json:"alias"`
	InitiatorType        string    `json:"type"`
	FCID                 string    `json:"fcid,omitempty"`
	FCIDValue            string    `json:"fcid_value"`
	FCIDLockdown         string    `json:"fcid_lockdown"`
	IPAddress            string    `json:"ip_address,omitempty"`
	Host                 string    `json:"host,omitempty"`
	HostGroupIDs         []string  `json:"hostGroup,omitempty"`
	LoggedIn             bool      `json:"logged_in"`
	OnFabric             bool      `json:"on_fabric"`
	FabricName           string    `json:"fabric_name"`
	PortFlagsOverride    bool      `json:"port_flags_override"`
	EnabledFlags         string    `json:"enabled_flags"`
	DisabledFlags        string    `json:"disabled_flags"`
	FlagsInEffect        string    `json:"flags_in_effect"`
	NumberVols           int64     `json:"num_of_vols"`
	NumberHostGroups     int64     `json:"num_of_host_groups"`
	NumberMaskingViews   int64     `json:"num_of_masking_views"`
	MaskingView          []string  `json:"maskingview"`
	PowerPathHosts       []string  `json:"powerpathhosts"`
	NumberPowerPathHosts int64     `json:"num_of_powerpath_hosts"`
	HostID               string    `json:"host_id"`
}

// HostList : list of hosts
type HostList struct {
	HostIDs []string `json:"hostId"`
}

// Host : Information about a host
type Host struct {
	HostID             string   `json:"hostId"`
	NumberMaskingViews int64    `json:"num_of_masking_views"`
	NumberInitiators   int64    `json:"num_of_initiators"`
	NumberHostGroups   int64    `json:"num_of_host_groups"`
	PortFlagsOverride  bool     `json:"port_flags_override"`
	ConsistentLun      bool     `json:"consistent_lun"`
	EnabledFlags       string   `json:"enabled_flags"`
	DisabledFlags      string   `json:"disabled_flags"`
	HostType           string   `json:"type"`
	Initiators         []string `json:"initiator"`
	MaskingviewIDs     []string `json:"maskingview"`
	PowerPathHosts     []string `json:"powerpathhosts"`
	NumPowerPathHosts  int64    `json:"num_of_powerpath_hosts"`
	BWLimit            int      `json:"bw_limit"`
}

// DirectorIDList : list of directors
type DirectorIDList struct {
	DirectorIDs []string `json:"directorId"`
}

// PortList : list of ports
type PortList struct {
	ExecutionOption  string    `json:"executionOption,omitempty"`
	SymmetrixPortKey []PortKey `json:"symmetrixPortKey"`
}

// SymmetrixPortType : type of symmetrix port
type SymmetrixPortType struct {
	SymmetrixPortKey                       PortKey  `json:"symmetrixPortKey"`
	PortStatus                             string   `json:"port_status"`
	DirectorStatus                         string   `json:"director_status"`
	Type                                   string   `json:"type,omitempty"`
	NumberOfCores                          string   `json:"number_of_cores"`
	Identifier                             string   `json:"identifier,omitempty"`
	PortGroups                             []string `json:"portgroup"`
	MaskingViews                           []string `json:"maskingview"`
	PortInterface                          string   `json:"port_interface"`
	ISCSITarget                            bool     `json:"iscsi_target,omitempty"`
	IPAddresses                            []string `json:"ip_addresses,omitempty"`
	NegotiatedSpeed                        string   `json:"negotiated_speed,omitempty"`
	MacAddress                             string   `json:"mac_address,omitempty"`
	NumOfPortGroups                        int64    `json:"num_of_port_groups,omitempty"`
	NumOfMaskingViews                      int64    `json:"num_of_masking_views,omitempty"`
	NumOfMappedVols                        int64    `json:"num_of_mapped_vols,omitempty"`
	VcmState                               string   `json:"vcm_state,omitempty"`
	Aclx                                   bool     `json:"aclx,omitempty"`
	CommonSerialNumber                     bool     `json:"common_serial_number,omitempty"`
	UniqueWwn                              bool     `json:"unique_wwn,omitempty"`
	InitPointToPoint                       bool     `json:"init_point_to_point,omitempty"`
	VolumeSetAddressing                    bool     `json:"volume_set_addressing,omitempty"`
	VnxAttached                            bool     `json:"vnx_attached,omitempty"`
	AvoidResetBroadcast                    bool     `json:"avoid_reset_broadcast,omitempty"`
	NegotiateReset                         bool     `json:"negotiate_reset,omitempty"`
	EnableAutoNegotiate                    bool     `json:"enable_auto_negotiate,omitempty"`
	EnvironSet                             bool     `json:"environ_set,omitempty"`
	DisableQResetOnUA                      bool     `json:"disable_q_reset_on_ua,omitempty"`
	SoftReset                              bool     `json:"soft_reset,omitempty"`
	Scsi3                                  bool     `json:"scsi_3,omitempty"`
	ScsiSupport1                           bool     `json:"scsi_support1,omitempty"`
	NoParticipating                        bool     `json:"no_participating,omitempty"`
	Spc2ProtocolVersion                    bool     `json:"spc2_protocol_version,omitempty"`
	Hp3000Mode                             bool     `json:"hp_3000_mode,omitempty"`
	Sunapee                                bool     `json:"sunapee,omitempty"`
	Siemens                                bool     `json:"siemens,omitempty"`
	RxPowerLevelMw                         float64  `json:"rx_power_level_mw,omitempty"`
	TxPowerLevelMw                         float64  `json:"tx_power_level_mw,omitempty"`
	PowerLevelsLastSampledDataMilliseconds int64    `json:"power_levels_last_sampled_data_milliseconds,omitempty"`
	NumOfHypers                            int64    `json:"num_of_hypers,omitempty"`
	RdfRaGroupAttributesFarpoint           bool     `json:"rdf_ra_group_attributes_farpoint,omitempty"`
	PreventAutomaticRdfLinkRecovery        string   `json:"prevent_automatic_rdf_link_recovery,omitempty"`
	PreventRaOnlineOnPowerUp               string   `json:"prevent_ra_online_on_power_up,omitempty"`
	RdfSoftwareCompressionSupported        string   `json:"rdf_software_compression_supported,omitempty"`
	RdfSoftwareCompression                 string   `json:"rdf_software_compression,omitempty"`
	Ipv4Address                            string   `json:"ipv4_address,omitempty"`
	Ipv6Adderess                           string   `json:"ipv6_address,omitempty"`
	Ipv6Prefix                             string   `json:"ipv6_prefix,omitempty"`
	Ipv4DefaultGateway                     string   `json:"ipv4_default_gateway,omitempty"`
	Ipv4DomainName                         string   `json:"ipv4_domain_name,omitempty"`
	Ipv4Netmask                            string   `json:"ipv4_netmask,omitempty"`
	MaxSpeed                               string   `json:"max_speed,omitempty"`
	WwnNode                                string   `json:"wwn_node,omitempty"`
	ISCSIEndpoint                          bool     `json:"iscsi_endpoint,omitempty"`
	NvmetcpEndpoint                        bool     `json:"nvmetcp_endpoint,omitempty"`
	NetworkID                              int64    `json:"network_id,omitempty"`
	TCPPort                                int32    `json:"tcp_port,omitempty"`
	EnabledProtocols                       []string `json:"enabled_protocols,omitempty"`
	CapableProtocols                       []string `json:"capable_protocols,omitempty"`
}

// Port is a minimal represation of a Symmetrix Port for iSCSI target purpose
type Port struct {
	ExecutionOption string            `json:"executionOption,omitempty"`
	SymmetrixPort   SymmetrixPortType `json:"symmetrixPort"`
}

type VersionDetails struct {
	Version    string `json:"version"`
	APIVersion string `json:"api_version"`
}

type PortGroupListResult struct {
	Results []PortGroupListv1 `json:"results,omitempty"`
}

// PortGroupListByID : list of port groups by ID
type PortGroupListv1 struct {
	ID       string       `json:"id,omitempty"`
	Protocol string       `json:"protocol,omitempty"`
	Ports    []PortValues `json:"ports,omitempty"`
}

// PortValues : list of port values
type PortValues struct {
	PortID   string     `json:"id,omitempty"`
	Type     string     `json:"type,omitempty"`
	Director DirectorID `json:"director,omitempty"`
}

// DirectorID : list of director IDs
type DirectorID struct {
	ID string `json:"id,omitempty"`
}

type PortV1 struct {
	ID             string         `json:"id,omitempty"`
	ResourceType   string         `json:"resource_type,omitempty"`
	PortNumber     string         `json:"port_number,omitempty"`
	PortIdentifier string         `json:"port_identifier,omitempty"`
	PortStatus     string         `json:"port_status"`
	DirectorStatus string         `json:"director_status"`
	Director       []PortDirector `json:"director,omitempty"`
}
type PortDirector struct {
	ID string `json:"id,omitempty"`
}
