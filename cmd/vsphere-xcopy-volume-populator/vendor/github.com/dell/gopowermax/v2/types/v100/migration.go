/*
 *
 * Copyright © 2021-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package v100

// MigrationEnv related data types
type MigrationEnv struct {
	ArrayID               string `json:"arrayId"`
	StorageGroupCount     int    `json:"storageGroupCount"`
	MigrationSessionCount int    `json:"migrationSessionCount"`
	Local                 bool   `json:"local"`
}

// MigrationStorageGroups contains list of storage group for migration
type MigrationStorageGroups struct {
	StorageGroupIDList []string `json:"name"`
	MigratingNameList  []string `json:"migratingName"`
}

// MigrationSession contains information about device pairs, source and target masking views
type MigrationSession struct {
	SourceArray       string                 `json:"sourceArray"`
	TargetArray       string                 `json:"targetArray"`
	StorageGroup      string                 `json:"storageGroup"`
	State             string                 `json:"state"`
	TotalCapacity     float64                `json:"totalCapacity"`
	RemainingCapacity float64                `json:"remainingCapacity"`
	DevicePairs       []MigrationDevicePairs `json:"devicePairs"`
	SourceMaskingView []SourceMaskingView    `json:"sourceMaskingView"`
	TargetMaskingView []TargetMaskingView    `json:"targetMaskingView"`
	Offline           bool                   `json:"offline"`
	Type              string                 `json:"type"`
}

// ModifyMigrationSessionRequest contains param to modify a migration session
type ModifyMigrationSessionRequest struct {
	Action          string `json:"action"`
	ExecutionOption string `json:"executionOption"`
}

// CreateMigrationEnv param creates migration environment
type CreateMigrationEnv struct {
	OtherArrayID    string `json:"otherArrayId"`
	ExecutionOption string `json:"executionOption"`
}

// MigrationDevicePairs contains device pair information amidst migration
type MigrationDevicePairs struct {
	SrcVolumeName string `json:"srcVolumeName"`
	InvalidSrc    bool   `json:"invalidSrc"`
	MissingSrc    bool   `json:"missingSrc"`
	TgtVolumeName string `json:"tgtVolumeName"`
	InvalidTgt    bool   `json:"invalidTgt"`
	MissingTgt    bool   `json:"missingTgt"`
}

// SourceMaskingView contains source masking view information
type SourceMaskingView struct {
	Name           string         `json:"name"`
	Invalid        bool           `json:"invalid"`
	ChildInvalid   bool           `json:"childInvalid"`
	Missing        bool           `json:"missing"`
	InitiatorGroup InitiatorGroup `json:"initiatorGroup"`
	PortGroup      PortGroups     `json:"portGroup"`
}

// TargetMaskingView contains target masking view information
type TargetMaskingView struct {
	Name           string         `json:"name"`
	Invalid        bool           `json:"invalid"`
	ChildInvalid   bool           `json:"childInvalid"`
	Missing        bool           `json:"missing"`
	InitiatorGroup InitiatorGroup `json:"initiatorGroup"`
	PortGroup      PortGroups     `json:"portGroup"`
}

// InitiatorGroup contains initiator group information
type InitiatorGroup struct {
	Name         string       `json:"name"`
	Invalid      bool         `json:"invalid"`
	ChildInvalid bool         `json:"childInvalid"`
	Missing      bool         `json:"missing"`
	Initiator    []Initiators `json:"initiator"`
}

// Initiators contains initiator group information
type Initiators struct {
	Name    string `json:"name"`
	WWN     string `json:"wwn"`
	Invalid bool   `json:"invalid"`
}

// PortGroups contains port group information
type PortGroups struct {
	Name         string  `json:"name"`
	Invalid      bool    `json:"invalid"`
	ChildInvalid bool    `json:"childInvalid"`
	Missing      bool    `json:"missing"`
	Ports        []Ports `json:"ports"`
}

// Ports contains port information
type Ports struct {
	Name    string `json:"name"`
	Invalid bool   `json:"invalid"`
}
