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
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net/http"
)

const (
	TRACE_LEVEL = 2
	DEBUG_LEVEL = 1
	INFO_LEVEL  = 0
)

var ErrMappingExists = errors.New("mapping already exists")
var ErrNotFound = errors.New("item not found")

var ERROR_CODE_HOST_NOT_FOUND = "HOST_NOT_FOUND"

const (
	RESOURCE_NOT_FOUND    = 1
	PARAMETER_APPROVED    = "approved"
	PARAMETER_VALUE_TRUE  = "true"
	PARAMETER_VALUE_FALSE = "false"
	CONTENT_TYPE          = "Content-Type"
	JSON_CONTENT_TYPE     = "application/json; charset=UTF-8"
	PARAMETER_PAGE_SIZE   = "page_size"
	PARAMETER_PAGE        = "page"
)

/**
type APIError struct {
	Code int
	Err  error
}

func (r *APIError) Error() string {
	return fmt.Sprintf("iboxapi error code %d: err %v", r.Code, r.Err)
}
*/

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
	GetPoolByName(ctx context.Context, name string) (*PoolResult, error)
	GetPoolByID(ctx context.Context, id int) (*PoolResult, error)

	// volumes
	DeleteVolume(ctx context.Context, volumeID int) (*DeleteVolumeResponse, error)
	GetVolumeByName(ctx context.Context, volumeName string) (*Volume, error)
	GetVolume(ctx context.Context, volumeID int) (*Volume, error)
	UpdateVolume(ctx context.Context, volumeID int, volume Volume) (*Volume, error)
	CreateSnapshotVolume(ctx context.Context, snapshotParam CreateSnapshotVolumeRequest) (*Snapshot, error)
	PromoteSnapshot(ctx context.Context, snapshotID int) (*Volume, error)
	GetVolumesByParentID(ctx context.Context, parentID int) ([]Volume, error)

	// network spaces
	GetNetworkSpaceByName(ctx context.Context, networkSpaceName string) (nspace *NetworkSpace, err error)

	// datasets
	GetAllSnapshots(ctx context.Context) ([]Volume, error)

	// hosts
	GetAllHosts(ctx context.Context) (host []Host, err error)
	GetHostByName(ctx context.Context, hostName string) (host *Host, err error)
	CreateHost(ctx context.Context, hostName string) (host *Host, err error)
	DeleteHost(ctx context.Context, hostID int) (resp *Host, err error)
	AddHostSecurity(ctx context.Context, chapCreds map[string]string, hostID int) (host *AddHostSecurityResponse, err error)
	AddHostPort(ctx context.Context, portType, portAddress string, hostID int) (addPortResponse *AddPortResponse, err error)
	GetHostPort(ctx context.Context, hostID int, portAddress string) (hostPort *HostPort, err error)
	MapVolumeToHost(ctx context.Context, hostID, volumeID, lun int) (lunInfo *LunInfo, err error)
	GetAllLunByHost(ctx context.Context, hostID int) (luninfo []LunInfo, err error)
	GetLunByHostVolume(ctx context.Context, hostID, volumeID int) (lun *LunInfo, err error)
	UnMapVolumeFromHost(ctx context.Context, hostID, volumeID int) (resp *UnMapVolumeFromHostResponse, err error)

	// volumes
	GetLunsByVolume(ctx context.Context, volumeID int) (resp []LunInfo, err error)
	CreateVolume(ctx context.Context, request CreateVolumeRequest) (*Volume, error)

	// config
	GetMaxTreeqPerFs(ctx context.Context) (int, error)
	GetMaxFileSystems(ctx context.Context) (int, error)

	// components
	GetFCPorts(ctx context.Context) (fcNodes []FCNode, err error)

	// consistency group (volume group)
	GetConsistencyGroup(ctx context.Context, cgID int) (*ConsistencyGroupInfo, error)
	DeleteConsistencyGroup(ctx context.Context, cgID int) error
	GetConsistencyGroupByName(ctx context.Context, name string) (*ConsistencyGroupInfo, error)
	CreateSnapshotGroup(ctx context.Context, req CreateSnapshotGroupRequest) (*ConsistencyGroupInfo, error)
	GetMembersByCGID(ctx context.Context, cgID int) ([]MemberInfo, error)
	AddMemberToSnapshotGroup(ctx context.Context, volumeID, cgID int) error
	CreateConsistencyGroup(ctx context.Context, req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error)

	// for nfs
	GetFileSystemByName(ctx context.Context, name string) (*FileSystem, error)
	GetFileSystemsByPool(ctx context.Context, poolID int, fsPrefix string) ([]FileSystem, error)
	GetFileSystemsByParentID(ctx context.Context, parentID int) ([]FileSystem, error)
	GetFileSystemByID(ctx context.Context, fileSystemID int) (*FileSystem, error)
	CreateFileSystem(ctx context.Context, request CreateFileSystemRequest) (*FileSystem, error)
	DeleteFileSystem(ctx context.Context, fileSystemID int) error
	UpdateFileSystem(ctx context.Context, fileSystemID int, fileSystem FileSystem) (*FileSystem, error)
	CreateFileSystemSnapshot(ctx context.Context, snapshotParam FileSystemSnapshot) (*FileSystemSnapshotResponse, error)

	// treeq
	GetTreeqsByFileSystem(ctx context.Context, filesystemID int) ([]Treeq, error)
	UpdateTreeq(ctx context.Context, fileSystemID, treeqID int, body UpdateTreeqRequest) (*Treeq, error)
	CreateTreeq(ctx context.Context, filesystemID int, treeqParameter CreateTreeqRequest) (*Treeq, error)
	DeleteTreeq(ctx context.Context, fileSystemID, treeqID int) (*Treeq, error)
	GetTreeq(ctx context.Context, fileSystemID, treeqID int) (*Treeq, error)
	GetTreeqByName(ctx context.Context, fileSystemID int, treeqName string) (*Treeq, error)

	// exports
	GetExportsByFileSystemID(ctx context.Context, filesystemID int) ([]Export, error)
	GetExportByID(ctx context.Context, exportID int) (*Export, error)
	DeleteExport(ctx context.Context, exportID int) (*Export, error)
	CreateExport(ctx context.Context, request CreateExportRequest) (*Export, error)
	UpdateExportPermissions(ctx context.Context, export Export, exportPathRef ExportPathRef) (*Export, error)

	// metadata
	PutMetadata(ctx context.Context, objectID int, metadata map[string]any) (*PutMetadataResponse, error)
	GetMetadata(ctx context.Context, objectID int) ([]GetMetadataResult, error)
	DeleteMetadata(ctx context.Context, objectID int) (*DeleteMetadataResponse, error)

	// links and replication
	GetLink(ctx context.Context, linkID int) (*Link, error)
	GetLinks(ctx context.Context) ([]Link, error)
	CreateReplica(ctx context.Context, request CreateReplicaRequest) (*Replica, error)
	GetReplicas(ctx context.Context) ([]Replica, error)
	DeleteReplica(ctx context.Context, id int) error
	GetReplica(ctx context.Context, id int) (*Replica, error)

	// system
	GetSystem(ctx context.Context) (*SystemDetails, error)
	GetNtpStatus(ctx context.Context) ([]NtpStatus, error)

	CreateEvent(ctx context.Context, request EventRequest) error
}

type Credentials struct {
	Username string
	Password string
	URL      string
}

type IboxClient struct {
	Creds      Credentials
	HTTPClient *http.Client
}

func NewIboxClient(creds Credentials) (cl *IboxClient) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: tr}

	return &IboxClient{
		Creds:      creds,
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
