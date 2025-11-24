/*
Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.

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

// SnapshotPolicy holds all the fields of a Snapshot Policy
type SnapshotPolicy struct {
	// The System where the snapshot policy is located
	SymmetrixID string `json:"symmetrixID"`
	// The name of the snapshot policy on this System
	SnapshotPolicyName string `json:"snapshot_policy_name"`
	// The number of snapshots that will be taken before the oldest ones are no longer required
	SnapshotCount int64 `json:"snapshot_count"`
	// The number of minutes between each policy execution
	IntervalMinutes int64 `json:"interval_minutes"`
	// The number of minutes after 00:00 on Monday morning that the policy will execute
	OffsetMinutes int64 `json:"offset_minutes"`
	// The name of the cloud provider associated with this policy. Only applies to cloud policies.
	ProviderName string `json:"provider_name"`
	// The number of days that snapshots will be retained in the cloud for. Only applies to cloud policies.
	RetentionDays int64 `json:"retention_days"`
	// Set if the snapshot policy has been suspended
	Suspended bool `json:"suspended"`
	// Set if the snapshot policy creates secure snapshots
	Secure bool `json:"secure"`
	// The last time that the snapshot policy was run
	LastTimeUsed string `json:"last_time_used"`
	// The total number of storage groups that this snapshot policy is associated with
	StorageGroupCount int32 `json:"storage_group_count"`
	// The threshold of good snapshots which are not failed/bad for compliance to change from normal to warning.
	ComplianceCountWarning int64 `json:"compliance_count_warning"`
	// The threshold of good snapshots which are not failed/bad for compliance to change from warning to critical.
	ComplianceCountCritical int64 `json:"compliance_count_critical"`
	// The type of Snapshots that are created with the policy, local or cloud.
	Type string `json:"type"`
}

// CreateSnapshotPolicyParam Parameters for creating a new snapshot policy
type CreateSnapshotPolicyParam struct {
	// The name of the new snapshot policy.
	SnapshotPolicyName string `json:"snapshot_policy_name"`
	// The interval between snapshots Valid values:(10 Minutes, 12 Minutes, 15 Minutes, 20 Minutes, 30 Minutes,
	// 1 Hour, 2 Hours, 3 Hours, 4 Hours, 6 Hours, 8 Hours, 12 Hours, 1 Day, 7 Days)
	Interval string `json:"interval,omitempty"`
	// The number of minutes from 00:00 on a Monday morning when the policy should run. Default is 0 if not specified.
	OffsetMins int32 `json:"offset_mins,omitempty"`
	// The number of snapshots which are not failed or bad when compliance changes to warning.
	ComplianceCountWarning int64 `json:"compliance_count_warning,omitempty"`
	// The number of snapshots which are not failed or bad when compliance changes to critical.
	ComplianceCountCritical int64 `json:"compliance_count_critical,omitempty"`
	// Cloud Snapshot Policy Details
	CloudSnapshotPolicyDetails *CloudSnapshotPolicyDetails `json:"cloud_snapshot_policy_details,omitempty"`
	// Local Snapshot Policy Details
	LocalSnapshotPolicyDetails *LocalSnapshotPolicyDetails `json:"local_snapshot_policy_details,omitempty"`
	ExecutionOption            string                      `json:"executionOption"`
}

// CloudSnapshotPolicyDetails holds the cloud snapshot policy details
type CloudSnapshotPolicyDetails struct {
	// The number of cloud retention days. Has to be a minimum of 3 days and maximum of 5110 days
	CloudRetentionDays int32 `json:"cloud_retention_days,omitempty"`
	// The name of the Cloud Provider
	CloudProviderName string `json:"cloud_provider_name,omitempty"`
}

// LocalSnapshotPolicyDetails holds the local snapshot policy details
type LocalSnapshotPolicyDetails struct {
	// The snapshot policy will create secure snapshots
	Secure bool `json:"secure,omitempty"`
	// The number of the snapshots that will be maintained by the snapshot policy
	SnapshotCount int32 `json:"snapshot_count,omitempty"`
}

// ModifySnapshotPolicyParam Parameters for modifying basic Snapshot Policy fields
type ModifySnapshotPolicyParam struct {
	// The name to change the snapshot policy to.
	SnapshotPolicyName string `json:"snapshot_policy_name,omitempty"`
	// The number of minutes between when the snapshot policy runs. For valid values convert the allowed interval values((10 Minutes, 12 Minutes,
	// 15 Minutes,20 Minutes, 30 Minutes, 1 Hour, 2 Hours, 3 Hours, 4 Hours, 6 Hours, 8 Hours, 12 Hours, 1 Day, 7 Days)) to minutes.
	// Ex: 7 Days would be 10080 minutes.
	IntervalMinutes int64 `json:"interval_mins,omitempty"`
	// The number of minutes from 00:00 on a Monday morning when the SP should run first.
	OffsetMins int32 `json:"offset_mins,omitempty"`
	// The number of snapshots which are not failed or bad when compliance changes to warning. Set to -1 to disable this compliance threshold.
	ComplianceCountWarning int64 `json:"compliance_count_warning,omitempty"`
	// The number of snapshots which are not failed or bad when compliance changes to critical. Set to -1 to disable this compliance threshold.
	ComplianceCountCritical int64 `json:"compliance_count_critical,omitempty"`
	// The number of the snapshots that will be maintained by the snapshot policy
	SnapshotCount int32 `json:"snapshot_count,omitempty"`
}

// AssociateStorageGroupParam defines storage group ids that you want to add to the Snapshot Policy
type AssociateStorageGroupParam struct {
	// The storage group to add to the snapshot policy
	StorageGroupName []string `json:"storage_group_name,omitempty"`
}

// DisassociateStorageGroupParam defines storage group ids that you want to remove from the Snapshot Policy
type DisassociateStorageGroupParam struct {
	// The storage group to remove to the snapshot policy
	StorageGroupName []string `json:"storage_group_name,omitempty"`
}

// UpdateSnapshotPolicyParam Parameters for update Snapshot Policy
type UpdateSnapshotPolicyParam struct {
	// The action to be performed. Enumeration values: a)Modify - Modify the attributes of a snapshot policy
	// b)Suspend - Suspend a snapshot policy from running c)Resume - Resume a snapshot policy to running
	// d)AssociateToStorageGroups - Associate the snapshot policy to storage groups
	// e)DisassociateFromStorageGroups - Disassociate the snapshot policy from storage groups
	Action                        string                         `json:"action"`
	ModifySnapshotPolicyParam     *ModifySnapshotPolicyParam     `json:"modify,omitempty"`
	AssociateStorageGroupParam    *AssociateStorageGroupParam    `json:"associate_to_storage_group,omitempty"`
	DisassociateStorageGroupParam *DisassociateStorageGroupParam `json:"disassociate_from_storage_group,omitempty"`
	ExecutionOption               string                         `json:"executionOption"`
}

// SnapshotPolicyList contains list of snapshot Policiy ids
type SnapshotPolicyList struct {
	SnapshotPolicyIDs []string `json:"name"`
}
