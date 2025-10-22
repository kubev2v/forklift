/*
 *
 * Copyright © 2020-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package goscaleio

// PauseMode states in which the ConsistencyGroup can be set to when Paused.
type PauseMode string

// List of pause modes.
const (
	StopDataTransfer PauseMode = "StopDataTransfer"
	OnlyTrackChanges PauseMode = "OnlyTrackChanges"
)

// PauseReplicationConsistencyGroup defines struct for PauseReplicationConsistencyGroup.
type PauseReplicationConsistencyGroup struct {
	PauseMode string `json:"pauseMode"`
}

// SetRPOReplicationConsistencyGroup defines struct for SetRPOReplicationConsistencyGroup.
type SetRPOReplicationConsistencyGroup struct {
	RpoInSeconds string `json:"rpoInSeconds"`
}

// SetTargetVolumeAccessModeOnReplicationGroup defines struct for SetTargetVolumeAccessModeOnReplicationGroup.
type SetTargetVolumeAccessModeOnReplicationGroup struct {
	TargetVolumeAccessMode string `json:"targetVolumeAccessMode"`
}

// SetNewNameOnReplicationGroup defines struct for SetNewNameOnReplicationGroup.
type SetNewNameOnReplicationGroup struct {
	NewName string `json:"newName"`
}

// SynchronizationResponse defines struct for SynchronizationResponse.
type SynchronizationResponse struct {
	SyncNowKey string `json:"syncNowKey"`
}

// QuerySyncNowRequest defines struct for QuerySyncNowRequest.
type QuerySyncNowRequest struct {
	SyncNowKey string `json:"syncNowKey"`
}
