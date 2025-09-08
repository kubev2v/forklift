/*
 *
 * Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

type StorageContainerStorageProtocolEnum string

const (
	StorageContainerStorageProtocolEnumSCSI StorageContainerStorageProtocolEnum = "SCSI"
	StorageContainerStorageProtocolEnumNVME StorageContainerStorageProtocolEnum = "NVMe"
)

type StorageContainer struct {
	ID              string                              `json:"id,omitempty"`
	Name            string                              `json:"name,omitempty"`
	Quota           int64                               `json:"quota,omitempty"`
	StorageProtocol StorageContainerStorageProtocolEnum `json:"storage_protocol,omitempty"`
	HighWaterMark   int16                               `json:"high_water_mark,omitempty"`
}

func (s StorageContainer) Fields() []string {
	return []string{"id", "name", "quota", "storage_protocol", "high_water_mark"}
}
