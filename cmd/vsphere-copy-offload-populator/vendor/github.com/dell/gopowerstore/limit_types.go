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

// LimitIDEnum - ID of limits returned by the /limit endpoint
type LimitIDEnum string

const (
	// MaxVolumeSize - Max size of a volume
	MaxVolumeSize LimitIDEnum = "Max_Volume_Size"
	// Max_VirtualVolume_Size - Max size of a virtual volume
	MaxVirtualVolumeSize LimitIDEnum = "Max_VirtualVolume_Size"
	// Max_Folder_Size - Max size of a folder
	MaxFolderSize LimitIDEnum = "Max_Folder_Size"
)

// Limit - Response /limit endpoint
type Limit struct {
	ID    string `json:"id"`
	Limit int64  `json:"limit"`
}

// Fields - Returns fields which must be requested to fill struct
func (l *Limit) Fields() []string {
	return []string{"id", "limit"}
}
