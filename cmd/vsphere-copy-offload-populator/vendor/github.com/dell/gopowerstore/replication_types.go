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

import "errors"

type (
	RPOEnum     string
	RSStateEnum string
)

const (
	RpoFiveMinutes                    RPOEnum     = "Five_Minutes"
	RpoFifteenMinutes                 RPOEnum     = "Fifteen_Minutes"
	RpoThirtyMinutes                  RPOEnum     = "Thirty_Minutes"
	RpoOneHour                        RPOEnum     = "One_Hour"
	RpoSixHours                       RPOEnum     = "Six_Hours"
	RpoTwelveHours                    RPOEnum     = "Twelve_Hours"
	RpoOneDay                         RPOEnum     = "One_Day"
	RpoZero                           RPOEnum     = "Zero"
	RsStateInitializing               RSStateEnum = "Initializing"
	RsStateOk                         RSStateEnum = "OK"
	RsStateSynchronizing              RSStateEnum = "Synchronizing"
	RsStateSystemPaused               RSStateEnum = "System_Paused"
	RsStatePaused                     RSStateEnum = "Paused"
	RsStatePausedForMigration         RSStateEnum = "Paused_For_Migration"
	RsStatePausedForNdu               RSStateEnum = "Paused_For_NDU"
	RsStateFractured                  RSStateEnum = "Fractured"
	RsStateResuming                   RSStateEnum = "Resuming"
	RsStateFailingOver                RSStateEnum = "Failing_Over"
	RsStateFailingOverForDR           RSStateEnum = "Failing_Over_For_DR"
	RsStateFailedOver                 RSStateEnum = "Failed_Over"
	RsStateReprotecting               RSStateEnum = "Reprotecting"
	RsStatePartialCutoverForMigration RSStateEnum = "Partial_Cutover_For_Migration"
	RsStateSwitchingToMetroSync       RSStateEnum = "Switching_To_Metro_Sync"
	RsStateError                      RSStateEnum = "Error"
)

func (rpo RPOEnum) IsValid() error {
	switch rpo {
	case RpoFiveMinutes, RpoFifteenMinutes, RpoThirtyMinutes, RpoOneHour, RpoSixHours, RpoTwelveHours, RpoOneDay, RpoZero:
		return nil
	}
	return errors.New("invalid rpo type")
}

// ReplicationRuleCreate create replication rule request
type ReplicationRuleCreate struct {
	// Name of the replication rule.
	Name string `json:"name"`
	// Recovery point objective (RPO), which is the acceptable amount of data, measured in units of time, that may be lost in case of a failure.
	// If RPO is Zero, it indicates the replication_type is 'sync'.
	Rpo RPOEnum `json:"rpo"`
	// Unique identifier of the remote system to which this rule will replicate the associated resources
	RemoteSystemID string `json:"remote_system_id"`
	AlertThreshold int    `json:"alert_threshold,omitempty"`
	IsReadOnly     bool   `json:"is_read_only,omitempty"`
}

type ReplicationRule struct {
	// ID of replication rule
	ID string `json:"id"`
	// Name of replication rule
	Name string `json:"name"`
	// Rpo (Recovery point objective), which is the acceptable amount of data, measured in units of time, that may be lost in case of a failure.
	// If RPO is Zero, it indicates the replication_type is 'sync'.
	Rpo RPOEnum `json:"rpo"`
	// RemoteSystemID - unique identifier of the remote system to which this rule will replicate the associated resources.
	RemoteSystemID     string               `json:"remote_system_id"`
	ProtectionPolicies []ProtectionPolicy   `json:"policies"`
	AlertThreshold     int                  `json:"alert_threshold"`
	IsReadOnly         bool                 `json:"is_read_only,omitempty"`
	IsReplica          bool                 `json:"is_replica,omitempty"`
	ManagedBy          string               `json:"managed_by,omitempty"`
	ManagedByID        string               `json:"managed_by_id,omitempty"`
	RemoteSystem       RemoteSystem         `json:"remote_system,omitempty"`
	ReplicationSession []ReplicationSession `json:"replication_sessions,omitempty"`
}

func (rule *ReplicationRule) Fields() []string {
	return []string{"id", "name", "rpo", "remote_system_id", "alert_threshold", "is_read_only", "is_replica", "managed_by", "managed_by_id", "policies(id,name)", "remote_system(id,name)", "replication_sessions(id,state)"}
}

// VirtualMachines - Details of virtual machine
type VirtualMachines struct {
	ID           string `json:"id"`
	InstanceUUID string `json:"instance_uuid"`
	Name         string `json:"name"`
}

// FileSystems - Details of file system
type FileSystems struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PerformanceRules - Details of performance rule
type PerformanceRules struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IoPriority string `json:"io_priority"`
}

// ProtectionPolicyCreate create protection policy request
type ProtectionPolicyCreate struct {
	// Policy name.
	Name string `json:"name"`
	// Policy description.
	Description string `json:"description,omitempty"`
	// IDs of replication rules
	ReplicationRuleIDs []string `json:"replication_rule_ids"`
	// IDs of snapshot rules
	SnapshotRuleIDs []string `json:"snapshot_rule_ids"`
}

// failover params create failover request
type FailoverParams struct {
	// For DR failover.
	IsPlanned bool `json:"is_planned"`
	// Reverse replication direction
	Reverse bool `json:"reverse,omitempty"`
	// Force for DR
	Force bool `json:"force,omitempty"`
}

type ProtectionPolicy struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Type             string             `json:"type"`
	ManagedBy        string             `json:"managed_by,omitempty"`
	ManagedByID      string             `json:"managed_by_id"`
	IsReadOnly       bool               `json:"is_read_only"`
	IsReplica        bool               `json:"is_replica"`
	TypeL10          string             `json:"type_l10"`
	ManagedByL10     string             `json:"managed_by_l10n"`
	VirtualMachines  []VirtualMachines  `json:"virtual_machines"`
	FileSystems      []FileSystems      `json:"file_systems"`
	PerformanceRules []PerformanceRules `json:"performance_rules"`
	ReplicationRules []ReplicationRule  `json:"replication_rules"`
	SnapshotRules    []SnapshotRule     `json:"snapshot_rules"`
	Volumes          []Volume           `json:"volumes"`
	VolumeGroups     []VolumeGroup      `json:"volume_groups"`
}

func (policy *ProtectionPolicy) Fields() []string {
	return []string{"*", "replication_rules(*)", "snapshot_rules(*)", "virtual_machines(*)", "file_systems(*)", "performance_rules(*)", "volumes(*)", "volume_groups(*)"}
}

type StorageElementPair struct {
	LocalStorageElementID  string `json:"local_storage_element_id,omitempty"`
	RemoteStorageElementID string `json:"remote_storage_element_id,omitempty"`
	StorageElementType     string `json:"storage_element_type,omitempty"`
	ReplicationShadowID    string `json:"replication_shadow_id,omitempty"`
}

type ReplicationSession struct {
	ID               string      `json:"id,omitempty"`
	State            RSStateEnum `json:"state,omitempty"`
	Role             string      `json:"role,omitempty"`
	ResourceType     string      `json:"resource_type,omitempty"`
	LocalResourceID  string      `json:"local_resource_id,omitempty"`
	RemoteResourceID string      `json:"remote_resource_id,omitempty"`
	RemoteSystemID   string      `json:"remote_system_id,omitempty"` // todo: maybe name?
	// Type of replication session. One of Synchronous, Asynchronous, or Metro_Active_Active.
	Type string `json:"type,omitempty"`

	StorageElementPairs []StorageElementPair `json:"storage_element_pairs,omitempty"`
}

func (r *ReplicationSession) Fields() []string {
	return []string{"id", "state", "role", "resource_type", "local_resource_id", "remote_resource_id", "remote_system_id", "type", "storage_element_pairs"}
}

// ReplicationRoleEnum - List of replication role types associated with a replication session
type ReplicationRoleEnum string

const (
	ReplicationRoleSource            ReplicationRoleEnum = "Source"
	ReplicationRoleDestination       ReplicationRoleEnum = "Destination"
	ReplicationRoleMetroPreferred    ReplicationRoleEnum = "Metro_Preferred"
	ReplicationRoleMetroNonPreferred ReplicationRoleEnum = "Metro_Non_Preferred"
)

// ReplicationRuleModify modifies replication rule
type ReplicationRuleModify struct {
	// Name of the replication rule.
	Name string `json:"name,omitempty"`
	// Recovery point objective (RPO), which is the acceptable amount of data, measured in units of time, that may be lost in case of a failure.
	// If RPO is Zero, it indicates the replication_type is 'sync'.
	Rpo RPOEnum `json:"rpo,omitempty"`
	// Unique identifier of the remote system to which this rule will replicate the associated resources
	RemoteSystemID string `json:"remote_system_id,omitempty"`
	AlertThreshold int    `json:"alert_threshold,omitempty"`
}
