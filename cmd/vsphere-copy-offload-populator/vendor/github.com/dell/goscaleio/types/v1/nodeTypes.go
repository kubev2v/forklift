// Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goscaleio

// NodeDetails defines struct for Node
type NodeDetails struct {
	RefID               string          `json:"refId"`
	IPAddress           string          `json:"ipAddress"`
	CurrentIPAddress    string          `json:"currentIpAddress"`
	ServiceTag          string          `json:"serviceTag"`
	Model               string          `json:"model"`
	DeviceType          string          `json:"deviceType"`
	DiscoverDeviceType  string          `json:"discoverDeviceType"`
	DisplayName         string          `json:"displayName"`
	ManagedState        string          `json:"managedState"`
	State               string          `json:"state"`
	InUse               bool            `json:"inUse"`
	CustomFirmware      bool            `json:"customFirmware"`
	NeedsAttention      bool            `json:"needsAttention"`
	Manufacturer        string          `json:"manufacturer"`
	SystemID            string          `json:"systemId"`
	Health              string          `json:"health"`
	HealthMessage       string          `json:"healthMessage"`
	OperatingSystem     string          `json:"operatingSystem"`
	NumberOfCPUs        int             `json:"numberOfCPUs"`
	Nics                int             `json:"nics"`
	MemoryInGB          int             `json:"memoryInGB"`
	ComplianceCheckDate string          `json:"complianceCheckDate"`
	DiscoveredDate      string          `json:"discoveredDate"`
	DeviceGroupList     DeviceGroupList `json:"deviceGroupList"`
	DetailLink          DetailLink      `json:"detailLink"`
	CredID              string          `json:"credId"`
	Compliance          string          `json:"compliance"`
	FailuresCount       int             `json:"failuresCount"`
	Facts               string          `json:"facts"`
	PuppetCertName      string          `json:"puppetCertName"`
	FlexosMaintMode     int             `json:"flexosMaintMode"`
	EsxiMaintMode       int             `json:"esxiMaintMode"`
}

// DeviceGroupList defines struct for devices
type DeviceGroupList struct {
	DeviceGroup    []DeviceGroup `json:"deviceGroup"`
	ManagedDevices []NodeDetails `json:"managedDevices"`
}

// DeviceGroup defines struct for nodepool
type DeviceGroup struct {
	GroupSeqID       int           `json:"groupSeqId"`
	GroupName        string        `json:"groupName"`
	GroupDescription string        `json:"groupDescription"`
	CreatedDate      string        `json:"createdDate"`
	CreatedBy        string        `json:"createdBy"`
	UpdatedDate      string        `json:"updatedDate"`
	UpdatedBy        string        `json:"updatedBy"`
	GroupUserList    GroupUserList `json:"groupUserList"`
}

// GroupUserList defines struct for group users
type GroupUserList struct {
	TotalRecords int          `json:"totalRecords"`
	GroupUsers   []GroupUsers `json:"groupUsers"`
}

// GroupUsers defines struct for group user
type GroupUsers struct {
	UserSeqID string `json:"userSeqId"`
	UserName  string `json:"userName"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role"`
	Enabled   bool   `json:"enabled"`
}

// DetailLink defines struct for links
type DetailLink struct {
	Title string `json:"title"`
	Href  string `json:"href"`
	Rel   string `json:"rel"`
}

// ManagedDeviceList defines struct for managed devices
type ManagedDeviceList struct {
	ManagedDevices []NodeDetails `json:"managedDevices"`
}

// NodePoolDetails defines struct for nodepools
type NodePoolDetails struct {
	DeviceGroup       DeviceGroup       `json:"deviceGroup"`
	ManagedDeviceList ManagedDeviceList `json:"managedDeviceList"`
	GroupUserList     GroupUserList     `json:"groupUserList"`
}

// NodePoolDetailsFilter defines struct for nodepools
type NodePoolDetailsFilter struct {
	NodePoolDetails []DeviceGroup `json:"deviceGroup"`
}
