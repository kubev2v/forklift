/*
 *
 * Copyright © 2020-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// FcPort This resource type has queriable associations from appliance, hardware, hardware, hardware, fc_port
type FcPort struct {
	ApplianceID string `json:"appliance_id,omitempty"`
	// current speed
	CurrentSpeed FcPortSpeedEnum `json:"current_speed,omitempty"`
	// Localized message string corresponding to <b>current_speed</b>
	// Unique identifier of the port.
	ID string `json:"id,omitempty"`
	// This is the embeddable reference form of io_module_id attribute.
	IoModuleID string `json:"io_module_id,omitempty"`
	// Indicates whether the port's link is up. Values are:
	// * true - Link is up.
	// * false - Link is down.
	IsLinkUp bool `json:"is_link_up,omitempty"`
	// Name of the port.
	Name string `json:"name,omitempty"`
	// This is the embeddable reference form of node_id attribute.
	NodeID string `json:"node_id,omitempty"`
	// This is the embeddable reference form of partner_id attribute.
	PartnerID string `json:"partner_id,omitempty"`
	// port connector type
	PortIndex int64 `json:"port_index,omitempty"`
	// requested speed
	RequestedSpeed FcPortSpeedEnum `json:"requested_speed,omitempty"`
	// Localized message string corresponding to <b>requested_speed</b>
	SfpID string `json:"sfp_id,omitempty"`
	// List of supported transmission speeds for the port.
	SupportedSpeeds []FcPortSpeedEnum `json:"supported_speeds"`
	// World Wide Name (WWN) of the port.
	Wwn string `json:"wwn,omitempty"`
	// World Wide Name (WWN) of NVME port
	WwnNVMe string `json:"wwn_nvme,omitempty"`
	// World Wide Name (WWN) of the Node of the port.
	WwnNode string `json:"wwn_node,omitempty"`
}

// FcPortSpeedEnum Possible Fibre Channel port speeds. For the current_speed attribute, these values show the current transmission speed on the port.
// For the requested_speed attribute, these values show the transmission speed set by the user. A requested speed of Auto means that the current speed value will be automatically detected.
// If this file is updated, also update FrontEndPortSpeedEnum.yaml
// * Auto - the speed value is automatically detected
// * 4_Gbps - 4 Gigabits per second
// * 8_Gbps - 8 Gigabits per second
// * 16_Gbps - 16 Gigabits per second
// * 32_Gbps - 32 Gigabits per second
//
// swagger:model FcPortSpeedEnum
type FcPortSpeedEnum string

const (
	// FcPortSpeedEnumAuto captures enum value "Auto"
	FcPortSpeedEnumAuto FcPortSpeedEnum = "Auto"

	// FcPortSpeedEnumNr4Gbps captures enum value "4_Gbps"
	FcPortSpeedEnumNr4Gbps FcPortSpeedEnum = "4_Gbps"

	// FcPortSpeedEnumNr8Gbps captures enum value "8_Gbps"
	FcPortSpeedEnumNr8Gbps FcPortSpeedEnum = "8_Gbps"

	// FcPortSpeedEnumNr16Gbps captures enum value "16_Gbps"
	FcPortSpeedEnumNr16Gbps FcPortSpeedEnum = "16_Gbps"

	// FcPortSpeedEnumNr32Gbps captures enum value "32_Gbps"
	FcPortSpeedEnumNr32Gbps FcPortSpeedEnum = "32_Gbps"
)

// FcPortModify fc port modify
type FcPortModify struct {
	// requested speed
	// Required: true
	RequestedSpeed *FcPortSpeedEnum `json:"requested_speed"`
}

// Fields returns fields which must be requested to fill struct
func (h *FcPort) Fields() []string {
	return []string{
		"appliance_id", "current_speed", "id",
		"io_module_id", "is_link_up", "name", "node_id", "partner_id",
		"port_index", "requested_speed", "sfp_id", "supported_speeds", "wwn",
	}
}
