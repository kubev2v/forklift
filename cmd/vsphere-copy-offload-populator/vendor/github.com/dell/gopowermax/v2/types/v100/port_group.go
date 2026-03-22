/*
 Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

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

// AddPortParam ...
type AddPortParam struct {
	Ports []SymmetrixPortKeyType `json:"port"`
}

// RemovePortParam ...
type RemovePortParam struct {
	Ports []SymmetrixPortKeyType `json:"port"`
}

// RenamePortGroupParam ...
type RenamePortGroupParam struct {
	NewPortGroupName string `json:"new_port_group_name"`
}

// EditPortGroupActionParam ...
type EditPortGroupActionParam struct {
	AddPortParam         *AddPortParam         `json:"addPortParam,omitempty"`
	RemovePortParam      *RemovePortParam      `json:"removePortParam,omitempty"`
	RenamePortGroupParam *RenamePortGroupParam `json:"renamePortGroupParam,omitempty"`
}

// EditPortGroup ...
type EditPortGroup struct {
	ExecutionOption          string                    `json:"executionOption"`
	EditPortGroupActionParam *EditPortGroupActionParam `json:"editPortGroupActionParam"`
}
