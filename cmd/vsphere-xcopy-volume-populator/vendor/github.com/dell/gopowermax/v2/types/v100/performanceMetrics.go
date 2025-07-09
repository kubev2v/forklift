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

// StorageGroupMetricsParam parameters for query
type StorageGroupMetricsParam struct {
	SymmetrixID    string   `json:"symmetrixId"`
	StartDate      int64    `json:"startDate"`
	EndDate        int64    `json:"endDate"`
	DataFormat     string   `json:"dataFormat"`
	StorageGroupID string   `json:"storageGroupId"`
	Metrics        []string `json:"metrics"`
}

// StorageGroupMetricsIterator contains the result of query
type StorageGroupMetricsIterator struct {
	ResultList     StorageGroupMetricsResultList `json:"resultList"`
	ID             string                        `json:"id"`
	Count          int                           `json:"count"`
	ExpirationTime int64                         `json:"expirationTime"`
	MaxPageSize    int                           `json:"maxPageSize"`
}

// StorageGroupMetricsResultList contains the list of storage group metrics
type StorageGroupMetricsResultList struct {
	Result []StorageGroupMetric `json:"result"`
	From   int                  `json:"from"`
	To     int                  `json:"to"`
}

// StorageGroupMetric is the struct of metric
type StorageGroupMetric struct {
	HostReads         float64 `json:"HostReads"`
	HostWrites        float64 `json:"HostWrites"`
	HostMBReads       float64 `json:"HostMBReads"`
	HostMBWritten     float64 `json:"HostMBWritten"`
	ReadResponseTime  float64 `json:"ReadResponseTime"`
	WriteResponseTime float64 `json:"WriteResponseTime"`
	AllocatedCapacity float64 `json:"AllocatedCapacity"`
	AvgIOSize         float64 `json:"AvgIOSize"`
	Timestamp         int64   `json:"timestamp"`
}

// VolumeMetricsParam parameters for query
type VolumeMetricsParam struct {
	SystemID                       string   `json:"systemId"`
	StartDate                      int64    `json:"startDate"`
	EndDate                        int64    `json:"endDate"`
	VolumeStartRange               string   `json:"volumeStartRange"`
	VolumeEndRange                 string   `json:"volumeEndRange"`
	DataFormat                     string   `json:"dataFormat"`
	CommaSeparatedStorageGroupList string   `json:"commaSeparatedStorageGroupList"`
	Metrics                        []string `json:"metrics"`
}

// VolumeMetricsIterator contains the result of query
type VolumeMetricsIterator struct {
	ResultList     VolumeMetricsResultList `json:"resultList"`
	ID             string                  `json:"id"`
	Count          int                     `json:"count"`
	ExpirationTime int64                   `json:"expirationTime"`
	MaxPageSize    int                     `json:"maxPageSize"`
}

// VolumeMetricsResultList contains the list of volume result
type VolumeMetricsResultList struct {
	Result []VolumeResult `json:"result"`
	From   int            `json:"from"`
	To     int            `json:"to"`
}

// VolumeResult contains the list of volume metrics and ID of volume
type VolumeResult struct {
	VolumeResult  []VolumeMetric `json:"volumeResult"`
	VolumeID      string         `json:"volumeId"`
	StorageGroups string         `json:"storageGroups"`
}

// VolumeMetric is the struct of metric
type VolumeMetric struct {
	MBRead            float64 `json:"MBRead"`
	MBWritten         float64 `json:"MBWritten"`
	Reads             float64 `json:"Reads"`
	Writes            float64 `json:"Writes"`
	ReadResponseTime  float64 `json:"ReadResponseTime"`
	WriteResponseTime float64 `json:"WriteResponseTime"`
	IoRate            float64 `json:"IoRate"`
	Timestamp         int64   `json:"timestamp"`
}

// StorageGroupKeysParam is the parameter of keys query
type StorageGroupKeysParam struct {
	SymmetrixID string `json:"symmetrixId"`
}

// StorageGroupKeysResult is the list of storage group info
type StorageGroupKeysResult struct {
	StorageGroupInfos []StorageGroupInfo `json:"storageGroupInfo"`
}

// StorageGroupInfo is the information of the storage group key
type StorageGroupInfo struct {
	StorageGroupID     string `json:"storageGroupId"`
	FirstAvailableDate int64  `json:"firstAvailableDate"`
	LastAvailableDate  int64  `json:"lastAvailableDate"`
}

// ArrayKeysResult is the list of array info
type ArrayKeysResult struct {
	ArrayInfos []ArrayInfo `json:"arrayInfo"`
}

// ArrayInfo is the information of the array key
type ArrayInfo struct {
	SymmetrixID        string `json:"symmetrixId"`
	FirstAvailableDate int64  `json:"firstAvailableDate"`
	LastAvailableDate  int64  `json:"lastAvailableDate"`
}

// FileSystemMetricsParam contains req param for filesystem metric
type FileSystemMetricsParam struct {
	SystemID     string   `json:"systemId"`
	EndDate      int64    `json:"endDate"`
	FileSystemID string   `json:"fileSystemID"`
	DataFormat   string   `json:"dataFormat"`
	Metrics      []string `json:"metrics"`
	StartDate    int64    `json:"startDate"`
}

// FileSystemMetricsIterator contains the result of query
type FileSystemMetricsIterator struct {
	ResultList     FileSystemMetricsResultList `json:"resultList"`
	ID             string                      `json:"id"`
	Count          int                         `json:"count"`
	ExpirationTime int64                       `json:"expirationTime"`
	MaxPageSize    int                         `json:"maxPageSize"`
}

// FileSystemMetricsResultList contains the list of volume result
type FileSystemMetricsResultList struct {
	Result []FileSystemResult `json:"result"`
	From   int                `json:"from"`
	To     int                `json:"to"`
}

// FileSystemResult contains the list of volume metrics and ID of volume
type FileSystemResult struct {
	PercentBusy float64 `json:"PercentBusy"`
	Timestamp   int64   `json:"timestamp"`
}
