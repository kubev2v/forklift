/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// VTreeDetails defines struct for VTrees
type VTreeDetails struct {
	CompressionMethod  string             `json:"compressionMethod"`
	DataLayout         string             `json:"dataLayout"`
	ID                 string             `json:"id"`
	InDeletion         bool               `json:"inDeletion"`
	Name               string             `json:"name"`
	RootVolumes        []string           `json:"rootVolumes"`
	StoragePoolID      string             `json:"storagePoolId"`
	Links              []*Link            `json:"links"`
	VtreeMigrationInfo VTreeMigrationInfo `json:"vtreeMigrationInfo"`
}

// VTreeMigrationInfo defines struct for VTree migration
type VTreeMigrationInfo struct {
	DestinationStoragePoolID string `json:"destinationStoragePoolId"`
	MigrationPauseReason     string `json:"migrationPauseReason"`
	MigrationQueuePosition   int64  `json:"migrationQueuePosition"`
	MigrationStatus          string `json:"migrationStatus"`
	SourceStoragePoolID      string `json:"sourceStoragePoolId"`
	ThicknessConversionType  string `json:"thicknessConversionType"`
}

// VTreeQueryBySelectedIDsParam defines struct for specifying Vtree IDs
type VTreeQueryBySelectedIDsParam struct {
	IDs []string `json:"ids"`
}
