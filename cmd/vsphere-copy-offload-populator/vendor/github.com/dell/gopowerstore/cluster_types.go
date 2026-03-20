/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *      http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

// RemoteSystem details about a remote system
type RemoteSystem struct {
	// Unique identifier of the remote system instance.
	ID string `json:"id,omitempty"`
	// User-specified name of the remote system instance.
	// This property supports case-insensitive filtering
	Name string `json:"name,omitempty"`
	// User-specified description of the remote system instance.
	Description string `json:"description,omitempty"`
	// Serial number of the remote system instance
	SerialNumber string `json:"serial_number,omitempty"`
	// Type of the remote system instance
	Type string `json:"type,omitempty"`
	// Management IP address of the remote system instance
	ManagementAddress string `json:"management_address,omitempty"`
	// Possible data connection states of a remote system
	DataConnectionState string `json:"data_connection_state,omitempty"`
	// Data Network Latency
	DataNetworkLatency string `json:"data_network_latency,omitempty"`
	// List of supported remote protection capabilities
	Capabilities []string `json:"capabilities,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (r *RemoteSystem) Fields() []string {
	return []string{"id", "name", "description", "serial_number", "type", "management_address", "data_connection_state", "data_network_latency", "capabilities"}
}

type DataConnectStateEnum string

// List of possible data connection states for a RemoteSystem
const (
	ConnStateOK                   DataConnectStateEnum = "OK"
	ConnStatePartialDataConnLoss  DataConnectStateEnum = "Partial_Data_Connection_Loss"
	ConnStateCompleteDataConnLoss DataConnectStateEnum = "Complete_Data_Connection_Loss"
	ConnStateNotAvailable         DataConnectStateEnum = "Status_Not_Available"
	ConnStateNoTargetsDiscovered  DataConnectStateEnum = "No_Targets_Discovered"
	ConnStateInitializing         DataConnectStateEnum = "Initializing"
	ConnStateUnstable             DataConnectStateEnum = "Data_Connection_Unstable"
)

type RemoteCapabilitiesEnum string

// List of possible remote protection capabilities for a remote system
const (
	AsyncBlock               RemoteCapabilitiesEnum = "Asynchronous_Block_Replication"
	SyncBlock                RemoteCapabilitiesEnum = "Synchronous_Block_Replication"
	AsyncFile                RemoteCapabilitiesEnum = "Asynchronous_File_Replication"
	AsyncVvol                RemoteCapabilitiesEnum = "Asynchronous_Vvol_Replication"
	BlockNonDisruptiveImport RemoteCapabilitiesEnum = "Block_Nondisruptive_Import"
	BlockAgentlessImport     RemoteCapabilitiesEnum = "Block_Agentless_Import"
	FileImport               RemoteCapabilitiesEnum = "File_Import"
	BlockMetro               RemoteCapabilitiesEnum = "Block_Metro_Active_Active"
	RemoteBackup             RemoteCapabilitiesEnum = "Remote_Backup"
	SyncFile                 RemoteCapabilitiesEnum = "Synchronous_File_Replication"
)

// Cluster details about the cluster
type Cluster struct {
	// Unique identifier of the cluster.
	ID string `json:"id,omitempty"`
	// User-specified name of the cluster
	Name string `json:"name,omitempty"`
	// Management IP address of the remote system instance
	ManagementAddress string `json:"management_address,omitempty"`
	// Current state of the cluster
	State string `json:"state,omitempty"`
	// NVMe Subsystem NQN for cluster
	NVMeNQN string `json:"nvm_subsystem_nqn,omitempty"`
	// Current clock time for the system in UTC format.
	SystemTime string `json:"system_time,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (r *Cluster) Fields() []string {
	return []string{"id", "name", "management_address", "state", "system_time"}
}
