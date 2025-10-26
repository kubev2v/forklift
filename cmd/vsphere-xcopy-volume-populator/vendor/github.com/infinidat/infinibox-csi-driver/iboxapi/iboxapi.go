/*
Copyright 2025 Infinidat
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
package iboxapi

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
)

const (
	TRACE_LEVEL = 2
	DEBUG_LEVEL = 1
	INFO_LEVEL  = 0
)

var ERROR_CODE_HOST_NOT_FOUND = "HOST_NOT_FOUND"

const (
	IBOXAPI_RESOURCE_NOT_FOUND_ERROR = 1
	PARAMETER_APPROVED               = "approved"
	PARAMETER_VALUE_TRUE             = "true"
	PARAMETER_VALUE_FALSE            = "false"
	CONTENT_TYPE                     = "Content-Type"
	JSON_CONTENT_TYPE                = "application/json; charset=UTF-8"
	PARAMETER_PAGE_SIZE              = "page_size"
	PARAMETER_PAGE                   = "page"
)

type APIError struct {
	Code int
	Err  error
}

func (r *APIError) Error() string {
	return fmt.Sprintf("iboxapi error code %d: err %v", r.Code, r.Err)
}

type Metadata struct {
	Ready           bool `json:"ready"`
	NumberOfObjects int  `json:"number_of_objects"`
	PageSize        int  `json:"page_size"`
	PagesTotal      int  `json:"pages_total"`
	Page            int  `json:"page"`
}

type Error struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Reasons  []any  `json:"reasons"`
	Severity string `json:"severity"`
	IsRemote bool   `json:"is_remote"`
	Data     any    `json:"data"`
}

type Client interface {
	// pools
	GetPoolByName(name string) (*PoolResult, error)
	GetPoolByID(id int) (*PoolResult, error)

	// volumes
	DeleteVolume(volumeID int) (*DeleteVolumeResponse, error)
	GetVolumeByName(volumeName string) (*Volume, error)
	GetVolume(volumeID int) (*Volume, error)
	UpdateVolume(volumeID int, volume Volume) (*Volume, error)
	CreateSnapshotVolume(snapshotParam CreateSnapshotVolumeRequest) (*Snapshot, error)
	GetVolumesByParentID(parentID int) ([]Volume, error)

	// network spaces
	GetNetworkSpaceByName(networkSpaceName string) (nspace *NetworkSpace, err error)

	// datasets
	GetAllSnapshots() ([]Volume, error)

	// hosts
	GetAllHosts() (host []Host, err error)
	GetHostByName(hostName string) (host *Host, err error)
	CreateHost(hostName string) (host *Host, err error)
	DeleteHost(hostID int) (resp *Host, err error)
	AddHostSecurity(chapCreds map[string]string, hostID int) (host *AddHostSecurityResponse, err error)
	AddHostPort(portType, portAddress string, hostID int) (addPortResponse *AddPortResponse, err error)
	GetHostPort(hostID int, portAddress string) (hostPort *HostPort, err error)
	MapVolumeToHost(hostID, volumeID, lun int) (lunInfo *LunInfo, err error)
	GetAllLunByHost(hostID int) (luninfo []LunInfo, err error)
	GetLunByHostVolume(hostID, volumeID int) (lun *LunInfo, err error)
	UnMapVolumeFromHost(hostID, volumeID int) (resp *UnMapVolumeFromHostResponse, err error)

	// volumes
	GetLunsByVolume(volumeID int) (resp []LunInfo, err error)
	CreateVolume(request CreateVolumeRequest) (*Volume, error)

	// config
	GetMaxTreeqPerFs() (int, error)
	GetMaxFileSystems() (int, error)

	// components
	GetFCPorts() (fcNodes []FCNode, err error)

	// consistency group (volume group)
	GetConsistencyGroup(cgID int) (*ConsistencyGroupInfo, error)
	DeleteConsistencyGroup(cgID int) error
	GetConsistencyGroupByName(name string) (*ConsistencyGroupInfo, error)
	CreateSnapshotGroup(req CreateSnapshotGroupRequest) (*ConsistencyGroupInfo, error)
	GetMembersByCGID(cgID int) ([]MemberInfo, error)
	AddMemberToSnapshotGroup(volumeID, cgID int) error
	CreateConsistencyGroup(req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error)

	// for nfs
	GetFileSystemByName(name string) (*FileSystem, error)
	GetFileSystemsByPool(poolID int, fsPrefix string) ([]FileSystem, error)
	GetFileSystemsByParentID(parentID int) ([]FileSystem, error)
	GetFileSystemByID(fileSystemID int) (*FileSystem, error)
	CreateFileSystem(request CreateFileSystemRequest) (*FileSystem, error)
	DeleteFileSystem(fileSystemID int) error
	UpdateFileSystem(fileSystemID int, fileSystem FileSystem) (*FileSystem, error)
	CreateFileSystemSnapshot(snapshotParam FileSystemSnapshot) (*FileSystemSnapshotResponse, error)

	// treeq
	GetTreeqsByFileSystem(filesystemID int) ([]Treeq, error)
	UpdateTreeq(fileSystemID, treeqID int, body UpdateTreeqRequest) (*Treeq, error)
	CreateTreeq(filesystemID int, treeqParameter CreateTreeqRequest) (*Treeq, error)
	DeleteTreeq(fileSystemID, treeqID int) (*Treeq, error)
	GetTreeq(fileSystemID, treeqID int) (*Treeq, error)
	GetTreeqByName(fileSystemID int, treeqName string) (*Treeq, error)

	// exports
	GetExportsByFileSystemID(filesystemID int) ([]Export, error)
	DeleteExport(exportID int) (*Export, error)
	CreateExport(request CreateExportRequest) (*Export, error)
	UpdateExportPermissions(export Export, exportPathRef ExportPathRef) (*Export, error)

	// metadata
	PutMetadata(objectID int, metadata map[string]interface{}) (*PutMetadataResponse, error)
	GetMetadata(objectID int) ([]GetMetadataResult, error)
	DeleteMetadata(objectID int) (*DeleteMetadataResponse, error)

	// links and replication
	GetLink(linkID int) (*Link, error)
	GetLinks() ([]Link, error)
	CreateReplica(request CreateReplicaRequest) (*Replica, error)
	GetReplicas() ([]Replica, error)
	DeleteReplica(id int) error
	GetReplica(id int) (*Replica, error)

	// system
	GetSystem() (*SystemDetails, error)
	GetNtpStatus() ([]NtpStatus, error)

	CreateEvent(request EventRequest) error
}

type Credentials struct {
	Username string
	Password string
	URL      string
}

type IboxClient struct {
	Creds      Credentials
	Log        logr.Logger
	HTTPClient *http.Client
}

func NewIboxClient(log logr.Logger, creds Credentials) (cl *IboxClient) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: tr}

	return &IboxClient{
		Creds:      creds,
		Log:        log,
		HTTPClient: httpClient,
	}
}

func SetAuthHeader(req *http.Request, creds Credentials) {
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	// Set the Basic Auth header
	auth := creds.Username + ":" + creds.Password
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", basicAuth)
}
