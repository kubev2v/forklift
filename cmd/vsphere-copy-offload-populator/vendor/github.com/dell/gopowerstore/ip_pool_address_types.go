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

// IPPurposeTypeEnum Network IP address purpose.
type IPPurposeTypeEnum string

const (
	// IPPurposeTypeEnumMgmtClusterFloating captures enum value "Mgmt_Cluster_Floating"
	IPPurposeTypeEnumMgmtClusterFloating IPPurposeTypeEnum = "Mgmt_Cluster_Floating"
	// IPPurposeTypeEnumMgmtApplianceFloating captures enum value "Mgmt_Appliance_Floating"
	IPPurposeTypeEnumMgmtApplianceFloating IPPurposeTypeEnum = "Mgmt_Appliance_Floating"
	// IPPurposeTypeEnumMgmtNodeCoreOS captures enum value "Mgmt_Node_CoreOS"
	IPPurposeTypeEnumMgmtNodeCoreOS IPPurposeTypeEnum = "Mgmt_Node_CoreOS"
	// IPPurposeTypeEnumMgmtNodeHost captures enum value "Mgmt_Node_Host"
	IPPurposeTypeEnumMgmtNodeHost IPPurposeTypeEnum = "Mgmt_Node_Host"
	// IPPurposeTypeEnumICMClusterFloating captures enum value "ICM_Cluster_Floating"
	IPPurposeTypeEnumICMClusterFloating IPPurposeTypeEnum = "ICM_Cluster_Floating"
	// IPPurposeTypeEnumICMApplianceFloating captures enum value "ICM_Appliance_Floating"
	IPPurposeTypeEnumICMApplianceFloating IPPurposeTypeEnum = "ICM_Appliance_Floating"
	// IPPurposeTypeEnumICMNodeCoreOS captures enum value "ICM_Node_CoreOS"
	IPPurposeTypeEnumICMNodeCoreOS IPPurposeTypeEnum = "ICM_Node_CoreOS"
	// IPPurposeTypeEnumStorageGlobal captures enum value "Storage_Global"
	IPPurposeTypeEnumStorageGlobal IPPurposeTypeEnum = "Storage_Global"
	// IPPurposeTypeEnumStorageIscsiInitiator captures enum value "Storage_Iscsi_Initiator"
	IPPurposeTypeEnumStorageIscsiInitiator IPPurposeTypeEnum = "Storage_Iscsi_Initiator"
	// IPPurposeTypeEnumStorageIscsiTarget captures enum value "Storage_Iscsi_Target"
	IPPurposeTypeEnumStorageIscsiTarget IPPurposeTypeEnum = "Storage_Iscsi_Target"
	// IPPurposeTypeEnumStorageNVMETCPPort captures enum value "Storage_NVMe_TCP_Port"
	IPPurposeTypeEnumStorageNVMETCPPort IPPurposeTypeEnum = "Storage_NVMe_TCP_Port"
	// IPPurposeTypeEnumStorageClusterFloating captures enum value "Storage_Cluster_Floating"
	IPPurposeTypeEnumStorageClusterFloating IPPurposeTypeEnum = "Storage_Cluster_Floating"
	// IPPurposeTypeEnumICDNode captures enum value "ICD_Node"
	IPPurposeTypeEnumICDNode IPPurposeTypeEnum = "ICD_Node"
	// IPPurposeTypeEnumSDNASClusterFloating captures enum value "SDNAS_Cluster_Floating"
	IPPurposeTypeEnumSDNASClusterFloating IPPurposeTypeEnum = "SDNAS_Cluster_Floating"
	// IPPurposeTypeEnumSDNASNode captures enum value "SDNAS_Node"
	IPPurposeTypeEnumSDNASNode IPPurposeTypeEnum = "SDNAS_Node"
	// IPPurposeTypeEnumSDNASNodeServiceability captures enum value "SDNAS_Node_Serviceability"
	IPPurposeTypeEnumSDNASNodeServiceability IPPurposeTypeEnum = "SDNAS_Node_Serviceability"
	// IPPurposeTypeEnumVmotion captures enum value "VMotion"
	IPPurposeTypeEnumVmotion IPPurposeTypeEnum = "VMotion"
	// IPPurposeTypeEnumUnused captures enum value "Unused"
	IPPurposeTypeEnumUnused IPPurposeTypeEnum = "Unused"
)

// IPPoolAddress ip pool address instance
type IPPoolAddress struct {
	// IP address value, in IPv4 or IPv6 format.
	Address string `json:"address,omitempty"`
	// Unique identifier of the appliance to which the IP address belongs.
	ApplianceID string `json:"appliance_id,omitempty"`
	// Unique identifier of the IP address.
	ID string `json:"id,omitempty"`
	// Unique identifier of the port that uses this IP address to provide access to storage network services, such as iSCSI. This attribute can be set only for an IP address used by networks of type Storage.
	IPPortID string `json:"ip_port_id,omitempty"`
	// IPPort instance
	IPPort IPPortInstance `json:"ip_port,omitempty"`
	// Unique identifier of the network to which the IP address belongs.
	NetworkID string `json:"network_id,omitempty"`
	// Unique identifier of the cluster node to which the IP address belongs.
	NodeID string `json:"node_id,omitempty"`
	// purposes
	Purposes []IPPurposeTypeEnum `json:"purposes,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (ip *IPPoolAddress) Fields() []string {
	return []string{
		"address", "appliance_id", "id", "ip_port_id",
		"ip_port(target_iqn, id)", "network_id", "node_id", "purposes",
	}
}

// IPPortInstance ip port instance
type IPPortInstance struct {
	// Unique identifier of the IP port.
	ID string `json:"id,omitempty"`
	// iSCSI qualified name used by the target configured on top of the IP port initially or as a result of network scaling. If the IP port is not used by an iSCSI connection, this attribute should be empty.
	TargetIqn string `json:"target_iqn,omitempty"`
}
