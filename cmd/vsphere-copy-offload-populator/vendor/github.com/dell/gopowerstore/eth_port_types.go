/*
 *
 * Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// EthPort Ethernet front-end port configuration for all cluster nodes.
// This resource type has queriable associations from appliance, hardware, bond, fsn, eth_port, ip_port.
type EthPort struct {
	// Unique identifier of the Ethernet port instance.
	ID string `json:"id,omitempty"`
	// Ethernet port name.
	Name string `json:"name,omitempty"`
	// The id of the appliance containing the port.
	ApplianceID string `json:"appliance_id,omitempty"`
	// Unique identifier of the hardware instance of type 'Node' containing the port.
	NodeID string `json:"node_id,omitempty"`
	// Unique identifier of the bond containing the port, or null if the port is not in a bond.
	BondID string `json:"bond_id,omitempty"`
	// Identifier of the associated fail-safe network, or null if the port is not in an FSN.
	// Added in version 3.5.0.0.
	FsnID string `json:"fsn_id,omitempty"`
	// Ethernet port current MAC address.
	MacAddress string `json:"mac_address,omitempty"`
	// Ethernet port permanent MAC address assigned at the moment of manufacture.
	// Added in version 3.0.0.0.
	PermanentMacAddress string `json:"permanent_mac_address,omitempty"`
	// Indicates whether the Ethernet port's link is up.
	IsLinkUp bool `json:"is_link_up,omitempty"`
	// Indicates whether the port is in use.
	// Added in version 3.0.0.0.
	IsInUse bool `json:"is_in_use,omitempty"`
	// The list of supported transmission speeds for Ethernet port.
	SupportedSpeeds []EthPortSpeedEnum `json:"supported_speeds,omitempty"`
	// Current transmission speed.
	CurrentSpeed EthPortSpeedEnum `json:"current_speed,omitempty"`
	// User-requested transmission speed.
	RequestedSpeed EthPortSpeedEnum `json:"requested_speed,omitempty"`
	// The Maximum transmission unit (MTU) packet size that the Ethernet port can transmit.
	CurrentMTU int32 `json:"current_mtu,omitempty"`
	// Unique identifier of the hardware instance of type 'SFP' inserted into the port.
	SfpID string `json:"sfp_id,omitempty"`
	// Unique identifier of the hardware instance of type 'IO_Module' handling the port.
	// Deprecated in version 2.0.0.0.
	IoModuleID string `json:"io_module_id,omitempty"`
	// Unique identifier of the parent hardware instance handling the port.
	// Added in version 2.0.0.0.
	HardwareParentID string `json:"hardware_parent_id,omitempty"`
	// The index of the Ethernet port in IO module.
	PortIndex int32 `json:"port_index,omitempty"`
	// The type of connector supported by the port.
	PortConnectorType FrontEndPortConnectorTypeEnum `json:"port_connector_type,omitempty"`
	// Unique identifier of the partner port instance.
	PartnerID string `json:"partner_id,omitempty"`
	// Indicates whether the port is managed by a hypervisor.
	IsHypervisorManaged bool `json:"is_hypervisor_managed,omitempty"`
	// Hypervisor front-end port name.
	HypervisorPortName string `json:"hypervisor_port_name,omitempty"`
	// Name of the virtual switch associated with the hypervisor port.
	HypervisorVswitchName string `json:"hypervisor_vswitch_name,omitempty"`
	// Unique identifier of the virtual switch port associated with the hypervisor port.
	HypervisorPortID int32 `json:"hypervisor_port_id,omitempty"`
	// Unique identifier of the virtual switch associated with the hypervisor port.
	HypervisorVswitchID string `json:"hypervisor_vswitch_id,omitempty"`
	// Link local discovery information received from the uplink port.
	// Added in version 4.1.0.0.
	L2DiscoveryDetails L2DiscoveryDetails `json:"l2_discovery_details,omitempty"`
	// The stale state of the port.
	// Added in version 2.0.0.0.
	StaleState PortStaleStateEnum `json:"stale_state,omitempty"`
}

// L2DiscoveryDetails contains link local discovery information received from the uplink port.
// Added in version 4.1.0.0.
type L2DiscoveryDetails struct {
	// Remote switch MAC address.
	RemoteMac string `json:"remote_mac,omitempty"`
	// Name of the interface of the port.
	RemotePortName string `json:"remote_port_name,omitempty"`
	// Name of the remote switch.
	RemoteName string `json:"remote_name,omitempty"`
	// Description of the remote switch.
	RemoteDescription string `json:"remote_description,omitempty"`
	// Native VLAN of the remote switch.
	RemoteNativeVlan int32 `json:"remote_native_vlan,omitempty"`
	// MTU of the remote switch.
	RemoteMTU int32 `json:"remote_mtu,omitempty"`
}

// EthPortSpeedEnum Supported Ethernet front-end port transmission speeds.
// For the current_speed attribute, these values show the current transmission speed on the port.
// For the requested_speed attribute, these values show the transmission speed set by the user.
// A requested speed of Auto means that the current speed value will be automatically detected.
//
// swagger:model EthPortSpeedEnum
type EthPortSpeedEnum string

const (
	// EthPortSpeedEnumAuto - the speed value is automatically detected
	EthPortSpeedEnumAuto EthPortSpeedEnum = "Auto"

	// EthPortSpeedEnum10Mbps - 10 Megabits per second
	EthPortSpeedEnum10Mbps EthPortSpeedEnum = "10_Mbps"

	// EthPortSpeedEnum100Mbps - 100 Megabits per second
	EthPortSpeedEnum100Mbps EthPortSpeedEnum = "100_Mbps"

	// EthPortSpeedEnum1Gbps - 1 Gigabits per second
	EthPortSpeedEnum1Gbps EthPortSpeedEnum = "1_Gbps"

	// EthPortSpeedEnum10Gbps - 10 Gigabits per second
	EthPortSpeedEnum10Gbps EthPortSpeedEnum = "10_Gbps"

	// EthPortSpeedEnum25Gbps - 25 Gigabits per second
	EthPortSpeedEnum25Gbps EthPortSpeedEnum = "25_Gbps"

	// EthPortSpeedEnum40Gbps - 40 Gigabits per second
	EthPortSpeedEnum40Gbps EthPortSpeedEnum = "40_Gbps"

	// EthPortSpeedEnum100Gbps - 100 Gigabits per second (added in 3.0.0.0)
	EthPortSpeedEnum100Gbps EthPortSpeedEnum = "100_Gbps"
)

// FrontEndPortConnectorTypeEnum represents the type of connector supported by the port.
//
// swagger:model FrontEndPortConnectorTypeEnum
type FrontEndPortConnectorTypeEnum string

const (
	// FrontEndPortConnectorTypeEnumUnknown - Unknown Connector
	FrontEndPortConnectorTypeEnumUnknown FrontEndPortConnectorTypeEnum = "Unknown"

	// FrontEndPortConnectorTypeEnumLC - Lucent Connector
	FrontEndPortConnectorTypeEnumLC FrontEndPortConnectorTypeEnum = "LC"

	// FrontEndPortConnectorTypeEnumRJ45 - RJ45 Connector
	FrontEndPortConnectorTypeEnumRJ45 FrontEndPortConnectorTypeEnum = "RJ45"

	// FrontEndPortConnectorTypeEnumCopperPigtail - Copper Pigtail Connector
	FrontEndPortConnectorTypeEnumCopperPigtail FrontEndPortConnectorTypeEnum = "Copper_Pigtail"

	// FrontEndPortConnectorTypeEnumOpticalPigtail - Optical Pigtail Connector
	FrontEndPortConnectorTypeEnumOpticalPigtail FrontEndPortConnectorTypeEnum = "Optical_Pigtail"

	// FrontEndPortConnectorTypeEnumNoSeparable - No Separable Connector
	FrontEndPortConnectorTypeEnumNoSeparable FrontEndPortConnectorTypeEnum = "No_Separable"
)

// PortStaleStateEnum represents the stale state of the port.
// Added in version 2.0.0.0.
//
// swagger:model PortStaleStateEnum
type PortStaleStateEnum string

const (
	// PortStaleStateEnumNotStale - Not stale
	PortStaleStateEnumNotStale PortStaleStateEnum = "Not_Stale"

	// PortStaleStateEnumDisconnected - The IO_Module hardware handling this port has a disconnected lifecycle state
	PortStaleStateEnumDisconnected PortStaleStateEnum = "Disconnected"
)

// EthPortModify represents the parameters for modifying an Ethernet port.
type EthPortModify struct {
	// The requested transmission speed for the port.
	// Required: true
	RequestedSpeed *EthPortSpeedEnum `json:"requested_speed"`
}

// Fields returns fields which must be requested to fill struct
func (e *EthPort) Fields() []string {
	return []string{
		"id", "name", "appliance_id", "node_id", "bond_id",
		"mac_address", "is_link_up", "supported_speeds",
		"current_speed", "requested_speed", "current_mtu",
		"sfp_id", "io_module_id", "hardware_parent_id",
		"port_index", "port_connector_type", "partner_id",
		"is_hypervisor_managed", "hypervisor_port_name",
		"hypervisor_vswitch_name", "hypervisor_port_id",
		"hypervisor_vswitch_id", "stale_state",
	}
}
