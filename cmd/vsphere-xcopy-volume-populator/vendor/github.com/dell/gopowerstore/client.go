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

package gopowerstore

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/dell/gopowerstore/api"
)

// Env variables
const (
	APIURLEnv                 = "GOPOWERSTORE_APIURL"
	UsernameEnv               = "GOPOWERSTORE_USERNAME"
	PasswordEnv               = "GOPOWERSTORE_PASSWORD"
	InsecureEnv               = "GOPOWERSTORE_INSECURE"
	HTTPTimeoutEnv            = "GOPOWERSTORE_HTTP_TIMEOUT"
	DebugEnv                  = "GOPOWERSTORE_DEBUG"
	paginationDefaultPageSize = 1000
)

// ApiClient defines gopowerstore client interface
type Client interface {
	APIClient() api.Client
	SetTraceID(ctx context.Context, value string) context.Context
	SetCustomHTTPHeaders(headers http.Header)
	GetCustomHTTPHeaders() http.Header
	GetVolume(ctx context.Context, id string) (Volume, error)
	GetVolumeByName(ctx context.Context, name string) (Volume, error)
	GetVolumes(ctx context.Context) ([]Volume, error)
	CreateVolume(ctx context.Context, createParams *VolumeCreate) (CreateResponse, error)
	DeleteVolume(ctx context.Context, deleteParams *VolumeDelete, id string) (EmptyResponse, error)
	GetSnapshotRules(ctx context.Context) ([]SnapshotRule, error)
	GetSnapshotRule(ctx context.Context, id string) (SnapshotRule, error)
	GetSnapshotRuleByName(ctx context.Context, name string) (SnapshotRule, error)
	CreateSnapshotRule(ctx context.Context, createParams *SnapshotRuleCreate) (CreateResponse, error)
	ModifySnapshotRule(ctx context.Context, modifyParams *SnapshotRuleCreate, id string) (resp EmptyResponse, err error)
	DeleteSnapshotRule(ctx context.Context, deleteParams *SnapshotRuleDelete, id string) (EmptyResponse, error)
	DeleteReplicationRule(ctx context.Context, id string) (resp EmptyResponse, err error)
	DeleteProtectionPolicy(ctx context.Context, id string) (resp EmptyResponse, err error)
	DeleteVolumeGroup(ctx context.Context, id string) (resp EmptyResponse, err error)
	GetAppliance(ctx context.Context, id string) (ApplianceInstance, error)
	GetApplianceByName(ctx context.Context, name string) (ApplianceInstance, error)
	GetHost(ctx context.Context, id string) (Host, error)
	GetHostByName(ctx context.Context, name string) (Host, error)
	GetHosts(ctx context.Context) ([]Host, error)
	CreateHost(ctx context.Context, createParams *HostCreate) (CreateResponse, error)
	DeleteHost(ctx context.Context, deleteParams *HostDelete, id string) (EmptyResponse, error)
	ModifyHost(ctx context.Context, modifyParams *HostModify, id string) (CreateResponse, error)
	GetHostVolumeMappings(ctx context.Context) ([]HostVolumeMapping, error)
	GetHostVolumeMapping(ctx context.Context, id string) (HostVolumeMapping, error)
	GetHostVolumeMappingByVolumeID(ctx context.Context, volumeID string) ([]HostVolumeMapping, error)
	AttachVolumeToHost(ctx context.Context, hostID string, attachParams *HostVolumeAttach) (EmptyResponse, error)
	AttachVolumeToHostGroup(ctx context.Context, hostGroupID string, attachParams *HostVolumeAttach) (EmptyResponse, error)
	DetachVolumeFromHost(ctx context.Context, hostID string, detachParams *HostVolumeDetach) (EmptyResponse, error)
	DetachVolumeFromHostGroup(ctx context.Context, hostGroupID string, detachParams *HostVolumeDetach) (EmptyResponse, error)
	GetStorageISCSITargetAddresses(ctx context.Context) ([]IPPoolAddress, error)
	GetStorageNVMETCPTargetAddresses(ctx context.Context) ([]IPPoolAddress, error)
	GetCapacity(ctx context.Context) (int64, error)
	GetFCPorts(ctx context.Context) (resp []FcPort, err error)
	GetFCPort(ctx context.Context, id string) (resp FcPort, err error)
	GetSoftwareInstalled(ctx context.Context) (resp []SoftwareInstalled, err error)
	GetSoftwareMajorMinorVersion(ctx context.Context) (majorVersion float32, err error)
	SetLogger(logger Logger)
	CreateSnapshot(ctx context.Context, createSnapParams *SnapshotCreate, id string) (CreateResponse, error)
	DeleteSnapshot(ctx context.Context, deleteParams *VolumeDelete, id string) (EmptyResponse, error)
	GetSnapshotsByVolumeID(ctx context.Context, volID string) ([]Volume, error)
	GetSnapshots(ctx context.Context) ([]Volume, error)
	GetSnapshot(ctx context.Context, snapID string) (Volume, error)
	GetSnapshotByName(ctx context.Context, snapName string) (Volume, error)
	ComputeDifferences(ctx context.Context, snapdiffParams *VolumeComputeDifferences, volID string) (VolumeComputeDifferencesResponse, error)
	CreateVolumeFromSnapshot(ctx context.Context, createParams *VolumeClone, snapID string) (CreateResponse, error)
	GetNASServers(ctx context.Context) ([]NAS, error)
	GetNAS(ctx context.Context, id string) (NAS, error)
	GetNASByName(ctx context.Context, name string) (NAS, error)
	GetNfsServer(ctx context.Context, id string) (NFSServerInstance, error)
	GetFSByName(ctx context.Context, name string) (FileSystem, error)
	GetFS(ctx context.Context, id string) (FileSystem, error)
	GetFileInterface(ctx context.Context, id string) (FileInterface, error)
	GetNFSExport(ctx context.Context, id string) (resp NFSExport, err error)
	GetNFSExportByFilter(ctx context.Context, filter map[string]string) ([]NFSExport, error)
	GetNFSExportByName(ctx context.Context, name string) (NFSExport, error)
	GetNFSExportByFileSystemID(ctx context.Context, fsID string) (NFSExport, error)
	CreateNAS(ctx context.Context, createParams *NASCreate) (CreateResponse, error)
	DeleteNAS(ctx context.Context, id string) (EmptyResponse, error)
	CreateFS(ctx context.Context, createParams *FsCreate) (CreateResponse, error)
	DeleteFS(ctx context.Context, id string) (EmptyResponse, error)
	CreateNFSExport(ctx context.Context, createParams *NFSExportCreate) (CreateResponse, error)
	DeleteNFSExport(ctx context.Context, id string) (EmptyResponse, error)
	ModifyNFSExport(ctx context.Context, modifyParams *NFSExportModify, id string) (CreateResponse, error)
	CreateNFSServer(ctx context.Context, createParams *NFSServerCreate) (CreateResponse, error)
	CreateFsSnapshot(ctx context.Context, createSnapParams *SnapshotFSCreate, id string) (CreateResponse, error)
	DeleteFsSnapshot(ctx context.Context, id string) (EmptyResponse, error)
	GetFsSnapshotsByVolumeID(ctx context.Context, volID string) ([]FileSystem, error)
	GetFsSnapshots(ctx context.Context) ([]FileSystem, error)
	GetFsSnapshot(ctx context.Context, snapID string) (FileSystem, error)
	CreateFsFromSnapshot(ctx context.Context, createParams *FsClone, snapID string) (CreateResponse, error)
	GetFsByFilter(ctx context.Context, filter map[string]string) ([]FileSystem, error)
	CloneVolume(ctx context.Context, createParams *VolumeClone, volID string) (CreateResponse, error)
	ModifyVolume(ctx context.Context, modifyParams *VolumeModify, volID string) (EmptyResponse, error)
	ModifyFS(ctx context.Context, modifyParams *FSModify, volID string) (EmptyResponse, error)
	CloneFS(ctx context.Context, createParams *FsClone, fsID string) (CreateResponse, error)
	CreateReplicationRule(ctx context.Context, createParams *ReplicationRuleCreate) (CreateResponse, error)
	ModifyReplicationRule(ctx context.Context, modifyParams *ReplicationRuleModify, id string) (EmptyResponse, error)
	GetReplicationRule(ctx context.Context, id string) (resp ReplicationRule, err error)
	GetReplicationRuleByName(ctx context.Context, name string) (ReplicationRule, error)
	GetReplicationRules(ctx context.Context) ([]ReplicationRule, error)
	CreateProtectionPolicy(ctx context.Context, createParams *ProtectionPolicyCreate) (CreateResponse, error)
	ModifyVolumeGroup(ctx context.Context, modifyParams *VolumeGroupModify, id string) (resp EmptyResponse, err error)
	GetProtectionPolicy(ctx context.Context, id string) (ProtectionPolicy, error)
	GetProtectionPolicyByName(ctx context.Context, name string) (ProtectionPolicy, error)
	ModifyProtectionPolicy(ctx context.Context, modifyParams *ProtectionPolicyCreate, id string) (resp EmptyResponse, err error)
	GetProtectionPolicies(ctx context.Context) ([]ProtectionPolicy, error)
	GetRemoteSystem(ctx context.Context, id string) (RemoteSystem, error)
	GetRemoteSystemByName(ctx context.Context, name string) (RemoteSystem, error)
	GetVolumeGroup(ctx context.Context, id string) (VolumeGroup, error)
	GetVolumeGroupByName(ctx context.Context, name string) (VolumeGroup, error)
	GetVolumeGroupsByVolumeID(ctx context.Context, id string) (VolumeGroups, error)
	GetVolumeGroups(ctx context.Context) ([]VolumeGroup, error)
	CreateVolumeGroup(ctx context.Context, createParams *VolumeGroupCreate) (CreateResponse, error)
	CreateVolumeGroupSnapshot(ctx context.Context, volumeGroupID string, createParams *VolumeGroupSnapshotCreate) (resp CreateResponse, err error)
	ModifyVolumeGroupSnapshot(ctx context.Context, modifyParams *VolumeGroupSnapshotModify, id string) (resp EmptyResponse, err error)
	UpdateVolumeGroupProtectionPolicy(ctx context.Context, id string, params *VolumeGroupChangePolicy) (resp EmptyResponse, err error)
	RemoveMembersFromVolumeGroup(ctx context.Context, params *VolumeGroupMembers, id string) (EmptyResponse, error)
	AddMembersToVolumeGroup(ctx context.Context, params *VolumeGroupMembers, id string) (EmptyResponse, error)
	GetReplicationSessionByLocalResourceID(ctx context.Context, id string) (ReplicationSession, error)
	GetAllRemoteSystems(ctx context.Context) (resp []RemoteSystem, err error)
	GetRemoteSystems(ctx context.Context, filters map[string]string) (resp []RemoteSystem, err error)
	GetCluster(ctx context.Context) (Cluster, error)
	PerformanceMetricsByAppliance(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByApplianceResponse, error)
	PerformanceMetricsByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNodeResponse, error)
	PerformanceMetricsByVolume(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByVolumeResponse, error)
	VolumeMirrorTransferRate(ctx context.Context, entityID string) ([]VolumeMirrorTransferRateResponse, error)
	PerformanceMetricsByCluster(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByClusterResponse, error)
	PerformanceMetricsByVM(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByVMResponse, error)
	PerformanceMetricsByVg(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByVgResponse, error)
	PerformanceMetricsByFeFcPort(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeFcPortResponse, error)
	PerformanceMetricsByFeEthPort(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeEthPortResponse, error)
	PerformanceMetricsByFeEthNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeEthNodeResponse, error)
	PerformanceMetricsByFeFcNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeFcNodeResponse, error)
	PerformanceMetricsByFileSystem(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFileSystemResponse, error)
	WearMetricsByDrive(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]WearMetricsByDriveResponse, error)
	SpaceMetricsByCluster(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByClusterResponse, error)
	SpaceMetricsByAppliance(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByApplianceResponse, error)
	SpaceMetricsByVolume(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVolumeResponse, error)
	SpaceMetricsByVolumeFamily(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVolumeFamilyResponse, error)
	SpaceMetricsByVM(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVMResponse, error)
	SpaceMetricsByStorageContainer(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByStorageContainerResponse, error)
	SpaceMetricsByVolumeGroup(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVolumeGroupResponse, error)
	CopyMetricsByAppliance(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByApplianceResponse, error)
	CopyMetricsByCluster(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByClusterResponse, error)
	CopyMetricsByVolumeGroup(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByVolumeGroupResponse, error)
	CopyMetricsByRemoteSystem(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByRemoteSystemResponse, error)
	CopyMetricsByVolume(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByVolumeResponse, error)
	PerformanceMetricsSmbByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbNodeResponse, error)
	PerformanceMetricsSmbBuiltinclientByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbClientResponse, error)
	PerformanceMetricsSmbBranchCacheByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbCacheResponse, error)
	PerformanceMetricsSmb1ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV1NodeResponse, error)
	PerformanceMetricsSmb1BuiltinclientByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV1BuiltinClientResponse, error)
	PerformanceMetricsSmb2ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV2NodeResponse, error)
	PerformanceMetricsSmb2BuiltinclientByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV2BuiltinClientResponse, error)
	PerformanceMetricsNfsByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNfsResponse, error)
	PerformanceMetricsNfsv3ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNfsv3Response, error)
	PerformanceMetricsNfsv4ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNfsv4Response, error)
	ExecuteActionOnReplicationSession(ctx context.Context, id string, actionType ActionType, params *FailoverParams) (resp EmptyResponse, err error)
	GetReplicationSessionByID(ctx context.Context, id string) (resp ReplicationSession, err error)
	CreateStorageContainer(ctx context.Context, createParams *StorageContainer) (CreateResponse, error)
	DeleteStorageContainer(ctx context.Context, id string) (EmptyResponse, error)
	GetStorageContainer(ctx context.Context, id string) (StorageContainer, error)
	ModifyStorageContainer(ctx context.Context, modifyParams *StorageContainer, id string) (EmptyResponse, error)
	CreateHostGroup(ctx context.Context, createParams *HostGroupCreate) (CreateResponse, error)
	GetHostGroup(ctx context.Context, id string) (HostGroup, error)
	GetHostGroups(ctx context.Context) ([]HostGroup, error)
	GetHostGroupByName(ctx context.Context, name string) (HostGroup, error)
	DeleteHostGroup(ctx context.Context, id string) (EmptyResponse, error)
	ModifyHostGroup(ctx context.Context, modifyParams *HostGroupModify, id string) (EmptyResponse, error)
	GetVolumeGroupSnapshot(ctx context.Context, snapID string) (VolumeGroup, error)
	GetVolumeGroupSnapshots(ctx context.Context) ([]VolumeGroup, error)
	GetVolumeGroupSnapshotByName(ctx context.Context, snapName string) (VolumeGroup, error)
	GetMaxVolumeSize(ctx context.Context) (int64, error)
	ConfigureMetroVolume(ctx context.Context, id string, config *MetroConfig) (resp MetroSessionResponse, err error)
	ConfigureMetroVolumeGroup(ctx context.Context, id string, config *MetroConfig) (resp MetroSessionResponse, err error)
	EndMetroVolume(ctx context.Context, id string, options *EndMetroVolumeOptions) (resp EmptyResponse, err error)
	EndMetroVolumeGroup(ctx context.Context, id string, options *EndMetroVolumeGroupOptions) (resp EmptyResponse, err error)
	CreateSMBShare(ctx context.Context, createParams *SMBShareCreate) (resp CreateResponse, err error)
	ModifySMBShare(ctx context.Context, id string, modifyParams *SMBShareModify) (resp EmptyResponse, err error)
	DeleteSMBShare(ctx context.Context, id string) (resp EmptyResponse, err error)
	GetSMBShare(ctx context.Context, id string) (resp SMBShare, err error)
	GetSMBShares(ctx context.Context, args map[string]string) (resp []SMBShare, err error)
	SetSMBShareACL(ctx context.Context, id string, acl *ModifySMBShareACL) (resp EmptyResponse, err error)
	GetSMBShareACL(ctx context.Context, id string) (resp SMBShareACL, err error)
}

// ClientIMPL provides basic API client implementation
type ClientIMPL struct {
	API api.Client
}

// SetTraceID method allows to set tracing ID to context which will be used in log messages
func (c *ClientIMPL) SetTraceID(ctx context.Context, value string) context.Context {
	return c.API.SetTraceID(ctx, value)
}

// SetCustomHTTPHeaders method register headers which will be sent with every request
func (c *ClientIMPL) SetCustomHTTPHeaders(headers http.Header) {
	c.API.SetCustomHTTPHeaders(headers)
}

func (c *ClientIMPL) GetCustomHTTPHeaders() http.Header {
	return c.API.GetCustomHTTPHeaders()
}

// Logger is interface required for gopowerstore custom logger
type Logger api.Logger

// SetLogger set logger which will be used by client
func (c *ClientIMPL) SetLogger(logger Logger) {
	c.API.SetLogger(api.Logger(logger))
}

// APIClient method returns powerstore API client may be useful for doing raw API requests
func (c *ClientIMPL) APIClient() api.Client {
	return c.API
}

// method allow to read paginated data from backend
func (c *ClientIMPL) readPaginatedData(f func(int) (api.RespMeta, error)) error {
	var err error
	var meta api.RespMeta
	meta, err = f(0)
	if err != nil {
		return err
	}
	if meta.Pagination.IsPaginate {
		for {
			nextOffset := meta.Pagination.Last + 1
			if nextOffset >= meta.Pagination.Total {
				break
			}
			meta, err = f(nextOffset)
			err = WrapErr(err)
			if err != nil {
				apiError, ok := err.(*APIError)
				if !ok {
					return err
				}
				if apiError.BadRange() {
					// could happen if some instances was deleted during pagination
					break
				}
			}
		}
	}
	return nil
}

// NewClient returns new PowerStore API client initialized from env vars
func NewClient() (Client, error) {
	options := NewClientOptions()
	insecure, err := strconv.ParseBool(os.Getenv(InsecureEnv))
	if err == nil {
		options.SetInsecure(insecure)
	}
	httpTimeout, err := strconv.ParseInt(os.Getenv(HTTPTimeoutEnv), 10, 64)

	if err == nil {
		options.SetDefaultTimeout(httpTimeout)
	}
	return NewClientWithArgs(
		os.Getenv(APIURLEnv),
		os.Getenv(UsernameEnv),
		os.Getenv(PasswordEnv),
		options)
}

// NewClientWithArgs returns new PowerStore API client initialized from args
func NewClientWithArgs(
	apiURL string,
	username, password string, options *ClientOptions,
) (Client, error) {
	client, err := api.New(apiURL, username, password,
		options.Insecure(), options.DefaultTimeout(), options.RateLimit(), options.RequestIDKey())
	if err != nil {
		return nil, err
	}

	return &ClientIMPL{client}, nil
}
