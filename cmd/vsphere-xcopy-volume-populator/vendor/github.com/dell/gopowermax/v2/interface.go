/*
 Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package pmax

import (
	"context"
	"net/http"

	types "github.com/dell/gopowermax/v2/types/v100"
)

// Debug is a boolean, when enabled, that enables logging of send payloads, and other debug information. Default to false.
// It is set true by unit testing.
var Debug = false

// ConfigConnect is an argument structure that can be passed to Authenticate.
// It contains the Endpoint, API Version (which should not be used), Username, and Password.
type ConfigConnect struct {
	Endpoint string
	Version  string
	Username string
	Password string
}

// ISCSITarget is a structure representing a target IQN and associated IP addresses
type ISCSITarget struct {
	IQN       string
	PortalIPs []string
}

// NVMeTCPTarget is a structure representing a target NQN and associated IP addresses
type NVMeTCPTarget struct {
	NQN       string
	PortalIPs []string
}

const (
	// DefaultAPIVersion is the default API version you will get if not specified to NewClientWithArgs.
	// The other supported versions are listed here.
	DefaultAPIVersion = "100"
	// APIVersion91 is the API version corresponding to 91
	APIVersion91 = "91"
)

// Pmax interface has all the externally available functions provided by the pmax client library for the Powermax accessed through Unisphere.
type Pmax interface {
	GetHTTPClient() *http.Client

	// Authenticate causes authentication and tests the connection
	Authenticate(ctx context.Context, configConnect *ConfigConnect) error

	// WithSymmetrixID set a default symmetrix ID for the admin client,
	// for it to be added to the request header.
	WithSymmetrixID(symmetrixID string) Pmax

	// SLO provisioning are the methods for SLO provisioning. All the methods requre a
	// symID to identify the Symmetrix.

	// GetVolumeIDsIterator generates a VolumeIterator containing the ids of either all or a selected set volumes.
	// The volumeIdentifierMatch string can be used to find a specific volume, or if the like bool is set, all the
	// volumes containing match as part of their VolumeIdentifier.
	GetVolumeIDsIterator(ctx context.Context, symID string, volumeIdentifierMatch string, like bool) (*types.VolumeIterator, error)

	// GetVolumesInStorageGroupIterator returns a list of volumes for a given StorageGroup
	GetVolumesInStorageGroupIterator(ctx context.Context, symID string, storageGroupID string) (*types.VolumeIterator, error)

	// GetVolumeIDsIteratorWithParams returns an iterator of a list of volumes with query parameters
	GetVolumeIDsIteratorWithParams(ctx context.Context, symID string, queryParams map[string]string) (*types.VolumeIterator, error)

	// GetVolumeIDsIteratorPage gets a page of volume ids from a Volume iterator.
	GetVolumeIDsIteratorPage(ctx context.Context, iter *types.VolumeIterator, from, to int) ([]string, error)

	// DeleteVolumeIDsIterator deletes a Volume iterator.
	DeleteVolumeIDsIterator(ctx context.Context, iter *types.VolumeIterator) error

	// GetVolumeIDList provides a simpler interface that returns a []string of volume ids
	// of volumes matching the volumeIdentifierMatch (and like) criteria. It is
	// implemented in terms of GetVolumeIDsIterator, GetVolumeIDsIteratorPage, and DeleteVolumeIDsIterator
	// and handles all the details of the iteration for you.
	GetVolumeIDList(ctx context.Context, symID string, volumeIdentifierMatch string, like bool) ([]string, error)

	// GetVolumeIDListInStorageGroup returns a list of volume IDs that are associated with the StorageGroup
	GetVolumeIDListInStorageGroup(ctx context.Context, symID string, storageGroupID string) ([]string, error)

	// GetVolumeIDListWithParams - Gets a list of volume ids with parameters
	GetVolumeIDListWithParams(ctx context.Context, symID string, queryParams map[string]string) ([]string, error)

	// GetVolumeByID returns a Volume given the volumeID.
	GetVolumeByID(ctx context.Context, symID string, volumeID string) (*types.Volume, error)

	// GetStorageGroupIDList returns a list of all the StorageGroup ids.
	GetStorageGroupIDList(ctx context.Context, symID, storageGroupIDMatch string, like bool) (*types.StorageGroupIDList, error)

	// GetStorageGroup returns a storage group given the StorageGroup id.
	GetStorageGroup(ctx context.Context, symID string, storageGroupID string) (*types.StorageGroup, error)

	// GetStorageGroupSnapshotPolicy returns a storage group snapshot policy details.
	GetStorageGroupSnapshotPolicy(ctx context.Context, symID, snapshotPolicyID, storageGroupID string) (*types.StorageGroupSnapshotPolicy, error)

	// GetStoragePool returns a storage pool given the GetStoragePoolID and SymID.
	GetStoragePool(ctx context.Context, symID string, storagePoolID string) (*types.StoragePool, error)

	// CreateStorageGroup creates a storage group given the Storage group id
	// and returns the storage group object. The storage group can be configured for thick volumes as an option.
	// This is a blocking call and will only return after the storage group has been created
	CreateStorageGroup(ctx context.Context, symID string, storageGroupID string, srpID string, serviceLevel string, thickVolumes bool, optionalPayload map[string]interface{}) (*types.StorageGroup, error)
	// UpdateStorageGroup updates a storage group (i.e. a PUT operation) and should support all the defined
	// operations (but many have not been tested).
	// This is done asynchronously and returns back a job
	UpdateStorageGroup(ctx context.Context, symID string, storageGroupID string, payload interface{}) (*types.Job, error)

	// UpdateStorageGroupS updates a storage group (i.e. a PUT operation) and should support all the defined
	// operations (but many have not been tested).
	// This is done synchronously and doesn't create any jobs
	UpdateStorageGroupS(ctx context.Context, symID string, storageGroupID string, payload interface{}) error

	// CreateVolumeInStorageGroup takes simplified input arguments to create a volume of a give name and size in a particular storage group.
	// This method creates a job and waits on the job to complete.
	CreateVolumeInStorageGroup(ctx context.Context, symID string, storageGroupID string, volumeName string, volumeSize interface{}, volOpts map[string]interface{}) (*types.Volume, error)

	// CreateVolumeInStorageGroupS takes simplified input arguments to create a volume of a give name and size in a particular storage group.
	// This is done synchronously and no jobs are created. HTTP header argument is optional
	CreateVolumeInStorageGroupS(ctx context.Context, symID, storageGroupID string, volumeName string, volumeSize interface{}, volOpts map[string]interface{}, opts ...http.Header) (*types.Volume, error)

	// CreateVolumeInProtectedStorageGroupS takes simplified input arguments to create a volume of a give name and size in a protected storage group.
	// This will add volume in both Local and Remote Storage group
	// This is done synchronously and no jobs are created. HTTP header argument is optional
	CreateVolumeInProtectedStorageGroupS(ctx context.Context, symID, remoteSymID, storageGroupID string, remoteStorageGroupID string, volumeName string, volumeSize interface{}, volOpts map[string]interface{}, opts ...http.Header) (*types.Volume, error)

	// GetStorageGroupSnapshots Gets All Storage Group Snapshots
	GetStorageGroupSnapshots(ctx context.Context, symID string, storageGroupID string, excludeManualSnaps bool, excludeSlSnaps bool) (*types.StorageGroupSnapshot, error)

	// GetStorageGroupSnapshotSnapIDs Gets a list of SnapIDs for a particular snapshot
	GetStorageGroupSnapshotSnapIDs(ctx context.Context, symID string, storageGroupID string, snapshotID string) (*types.SnapID, error)

	// GetStorageGroupSnapshotSnap Gets the details of a storage group snapshot snap
	GetStorageGroupSnapshotSnap(ctx context.Context, symID string, storageGroupID string, snapshotID, snapID string) (*types.StorageGroupSnap, error)

	// CreateStorageGroupSnapshot Creates a Storage Group Snapshot
	CreateStorageGroupSnapshot(ctx context.Context, symID string, storageGroupID string, payload *types.CreateStorageGroupSnapshot) (*types.StorageGroupSnap, error)

	// ModifyStorageGroupSnapshot Modify a Storage Group Snapshot snap
	ModifyStorageGroupSnapshot(ctx context.Context, symID string, storageGroupID string, snapshotID string, snapID string, payload *types.ModifyStorageGroupSnapshot) (*types.StorageGroupSnap, error)

	// DeleteStorageGroupSnapshot Deletes a Storage Group Snapshot snap
	DeleteStorageGroupSnapshot(ctx context.Context, symID string, storageGroupID string, snapshotID string, snapID string) error

	// DeleteStorageGroup deletes a storage group given a storage group id
	DeleteStorageGroup(ctx context.Context, symID string, storageGroupID string) error

	// DeleteMaskingView deletes a masking view given a masking view id
	DeleteMaskingView(ctx context.Context, symID string, maskingViewID string) error

	// RenameMaskingView renames masking view given its identifier (which is the name)
	RenameMaskingView(ctx context.Context, symID string, maskingViewID string, newName string) (*types.MaskingView, error)

	// GetStoragePoolList Gets the list of Storage Pools
	GetStoragePoolList(ctx context.Context, symID string) (*types.StoragePoolList, error)

	// RenameVolume Rename a Volume given the volumeID
	RenameVolume(ctx context.Context, symID string, volumeID string, newName string) (*types.Volume, error)

	// AddVolumesToStorageGroup Add volume(s) asynchronously to a StorageGroup
	AddVolumesToStorageGroup(ctx context.Context, symID, storageGroupID string, force bool, volumeIDs ...string) error
	// AddVolumesToStorageGroupS Add volume(s) synchronously to a StorageGroup
	// This is a blocking call and will only return once the volumes have been added to storage group
	AddVolumesToStorageGroupS(ctx context.Context, symID, storageGroupID string, force bool, volumeIDs ...string) error
	// AddVolumesToProtectedStorageGroup Adds one or more volumes (given by their volumeIDs) to a Protected StorageGroup
	AddVolumesToProtectedStorageGroup(ctx context.Context, symID, storageGroupID, remoteSymID, remoteStorageGroupID string, force bool, volumeIDs ...string) error

	// RemoveVolumesFromStorageGroup Removes volume(s) synchronously from a StorageGroup
	RemoveVolumesFromStorageGroup(ctx context.Context, symID string, storageGroupID string, force bool, volumeIDs ...string) (*types.StorageGroup, error)

	// RemoveVolumesFromProtectedStorageGroup removes one or more volumes (given by their volumeIDs) from a Protected StorageGroup.
	RemoveVolumesFromProtectedStorageGroup(ctx context.Context, symID string, storageGroupID, remoteSymID, remoteStorageGroupID string, force bool, volumeIDs ...string) (*types.StorageGroup, error)

	// InitiateDeallocationOfTracksFromVolume Initiate a job to remove storage space from the volume.
	InitiateDeallocationOfTracksFromVolume(ctx context.Context, symID string, volumeID string) (*types.Job, error)

	// DeleteVolume Deletes a volume
	DeleteVolume(ctx context.Context, symID string, volumeID string) error

	// GetMaskingViewList  returns a list of the MaskingView names.
	GetMaskingViewList(ctx context.Context, symID string) (*types.MaskingViewList, error)

	// GetMaskingViewByID returns a masking view given its identifier (which is the name)
	GetMaskingViewByID(ctx context.Context, symID string, maskingViewID string) (*types.MaskingView, error)

	// GetMaskingViewConnections returns the connections of a masking view (optionally for a specific volume id.)
	// Here volume id is the 5 digit volume ID.
	GetMaskingViewConnections(ctx context.Context, symID string, maskingViewID string, volumeID string) ([]*types.MaskingViewConnection, error)

	// CreateMaskingView creates a masking view given the Masking view id, Storage group id,
	// host id and the port id and returns the masking view object
	CreateMaskingView(ctx context.Context, symID string, maskingViewID string, storageGroupID string, hostOrhostGroupID string, isHost bool, portGroupID string) (*types.MaskingView, error)

	// CreatePortGroup creates a port group given the Port Group id and a list of dir/port ids
	CreatePortGroup(ctx context.Context, symID string, portGroupID string, dirPorts []types.PortKey, protocol string) (*types.PortGroup, error)

	// RenamePortGroup renames port group given it is identifier (which is the name)
	RenamePortGroup(ctx context.Context, symID string, portGroupID string, newName string) (*types.PortGroup, error)

	// GetSymmetrixIDList gets symmetrix list
	GetSymmetrixIDList(ctx context.Context) (*types.SymmetrixIDList, error)
	// GetSymmetrixByID gets symmetrix by given ID
	GetSymmetrixByID(ctx context.Context, id string) (*types.Symmetrix, error)

	// GetJobIDList retrieves the list of jobs on a given Symmetrix.
	// If optional parameter statusQuery is a types.JobStatusRunning or similar string, will search for jobs
	// with a particular status.
	GetJobIDList(ctx context.Context, symID string, statusQuery string) ([]string, error)
	GetJobByID(ctx context.Context, symID string, jobID string) (*types.Job, error)
	WaitOnJobCompletion(ctx context.Context, symID string, jobID string) (*types.Job, error)
	JobToString(job *types.Job) string

	// GetPortGroupList returns a list of all the Port Group ids.
	GetPortGroupList(ctx context.Context, symID string, portGroupType string) (*types.PortGroupList, error)
	// GetPortGroupByID returns a port group given the PortGroup id.
	GetPortGroupByID(ctx context.Context, symID string, portGroupID string) (*types.PortGroup, error)

	// GetInitiatorList returns a list of all the Initiator ids based on filters supplied
	GetInitiatorList(ctx context.Context, symID string, initiatorHBA string, isISCSI bool, inHost bool) (*types.InitiatorList, error)
	// GetInitiatorByID returns an Initiator given the Initiator id.
	GetInitiatorByID(ctx context.Context, symID string, initID string) (*types.Initiator, error)

	// GetHostList returns a list of all the Host ids.
	GetHostList(ctx context.Context, symID string) (*types.HostList, error)
	// GetHostByID returns a Host given the Host id.
	GetHostByID(ctx context.Context, symID string, hostID string) (*types.Host, error)
	// CreateHost creates a host from a list of InitiatorIDs (and optional HostFlags) return returns a types.Host.
	// Initiator IDs do not contain the storage port designations, just the IQN string or FC WWN.
	// Initiator IDs cannot be a member of more than one host.
	CreateHost(ctx context.Context, symID string, hostID string, initiatorIDs []string, hostFlags *types.HostFlags) (*types.Host, error)
	// DeleteHost deletes a host given the hostID.
	DeleteHost(ctx context.Context, symID string, hostID string) error
	// UpdateHostInitiators will update the inititators
	UpdateHostInitiators(ctx context.Context, symID string, host *types.Host, initiatorIDs []string) (*types.Host, error)
	UpdateHostName(ctx context.Context, symID, oldHostID, newHostID string) (*types.Host, error)
	UpdateHostFlags(ctx context.Context, symID string, hostID string, hostFlags *types.HostFlags) (*types.Host, error)
	// GetDirectorIDList returns a list of directors
	GetDirectorIDList(ctx context.Context, symID string) (*types.DirectorIDList, error)
	// GetPortList returns a list of all the ports on a specified director/array.
	GetPortList(ctx context.Context, symID string, directorID string, query string) (*types.PortList, error)
	// GetPortListByProtocol returns a list of ports associated with a given protocol for a specified Symmetrix array.
	GetPortListByProtocol(ctx context.Context, symID string, protocol string) (*types.PortList, error)
	// GetPort returns port details.
	GetPort(ctx context.Context, symID string, directorID string, portID string) (*types.Port, error)
	// GetListOfTargetAddresses returns an array of all IP addresses which expose iscsi targets.
	GetListOfTargetAddresses(ctx context.Context, symID string) ([]string, error)
	// GetNVMeTCPTargets returns a list of NVMeTCP targets for given sym id
	GetNVMeTCPTargets(ctx context.Context, symID string) ([]NVMeTCPTarget, error)
	// GetISCSITargets returns a list of ISCSI Targets for a given sym id
	GetISCSITargets(ctx context.Context, symID string) ([]ISCSITarget, error)
	// CreateHostGroup creates a hostGroup from a list of hostIDs (and optional HostFlags) and  returns a types.HostGroup.
	CreateHostGroup(ctx context.Context, symID string, hostGroupID string, hostIDs []string, hostFlags *types.HostFlags) (*types.HostGroup, error)
	// GetHostGroupList returns a list of all the HostGroup ids.
	GetHostGroupList(ctx context.Context, symID string) (*types.HostGroupList, error)
	// GetHostGroupByID returns a HostGroup given the HostGroup id.
	GetHostGroupByID(ctx context.Context, symID string, hostGroupID string) (*types.HostGroup, error)
	// DeleteHostGroup deletes a hostGroup given the hostGroupID.
	DeleteHostGroup(ctx context.Context, symID string, hostGroupID string) error
	// UpdateHostGroupName updates a hostGroup with new hostGroup ID and returns a types.HostGroup.
	UpdateHostGroupName(ctx context.Context, symID, oldHostGroupID, newHostGroupID string) (*types.HostGroup, error)
	// UpdateHostGroupFlags updates the hostflags of the hostGroup
	UpdateHostGroupFlags(ctx context.Context, symID string, hostGroupID string, hostFlags *types.HostFlags) (*types.HostGroup, error)
	// UpdateHostGroupHosts will add/remove the hosts for a host group
	UpdateHostGroupHosts(ctx context.Context, symID string, hostGroupID string, hostIDs []string) (*types.HostGroup, error)

	// SetAllowedArrays sets the list of arrays which can be manipulated
	// an empty list will allow all arrays to be accessed
	SetAllowedArrays(arrays []string) error
	// GetAllowedArrays returns a slice of arrays that can be manipulated
	GetAllowedArrays() []string
	// IsAllowedArray checks to see if we can manipulate the specified array
	IsAllowedArray(array string) (bool, error)

	// GetSnapVolumeList returns a list of all snapshot volumes on the array.
	GetSnapVolumeList(ctx context.Context, symID string, queryParams types.QueryParams) (*types.SymVolumeList, error)
	// GetVolumeSnapInfo returns snapVx information associated with a volume.
	GetVolumeSnapInfo(ctx context.Context, symID string, volume string) (*types.SnapshotVolumeGeneration, error)
	// GetSnapshotInfo returns snapVx information of the specified volume
	GetSnapshotInfo(ctx context.Context, symID, volume, SnapID string) (*types.VolumeSnapshot, error)
	// CreateSnapshot creates a snapVx snapshot of a volume using the input parameters
	CreateSnapshot(ctx context.Context, symID string, SnapID string, sourceVolumeList []types.VolumeList, ttl int64) error

	// ModifySnapshot executes actions on a snapshot asynchronously
	// This creates a job and waits on its completion
	ModifySnapshot(ctx context.Context, symID string, sourceVol []types.VolumeList,
		targetVol []types.VolumeList, SnapID string, action string,
		newSnapID string, generation int64, isCopy bool) error

	// ModifySnapshotS executes actions on a snapshot synchronously
	ModifySnapshotS(ctx context.Context, symID string, sourceVol []types.VolumeList,
		targetVol []types.VolumeList, SnapID string, action string,
		newSnapID string, generation int64, isCopy bool) error
	// DeleteSnapshot deletes a snapshot from a volume
	// This is an asynchronous call and waits for the job to complete
	DeleteSnapshot(ctx context.Context, symID, SnapID string, sourceVolumes []types.VolumeList, generation int64) error

	// DeleteSnapshotS deletes a snapshot from a volume
	// This is a synchronous call and doesn't create a job
	DeleteSnapshotS(ctx context.Context, symID, SnapID string, sourceVolumes []types.VolumeList, generation int64) error

	// GetSnapshotGenerations returns a list of all the snapshot generation on a specific snapshot
	GetSnapshotGenerations(ctx context.Context, symID, volume, SnapID string) (*types.VolumeSnapshotGenerations, error)
	// GetSnapshotGenerationInfo returns the specific generation info related to a snapshot
	GetSnapshotGenerationInfo(ctx context.Context, symID, volume, SnapID string, generation int64) (*types.VolumeSnapshotGeneration, error)
	// GetReplicationCapabilities returns details about SnapVX and SRDF execution capabilities on the Symmetrix array
	GetReplicationCapabilities(ctx context.Context) (*types.SymReplicationCapabilities, error)
	// GetPrivVolumeByID returns a Volume structure given the symmetrix and volume ID (volume ID is in WWN format)
	GetPrivVolumeByID(ctx context.Context, symID string, volumeID string) (*types.VolumeResultPrivate, error)

	// DeletePortGroup deletes a port group
	DeletePortGroup(ctx context.Context, symID string, portGroupID string) error
	// UpdatePortGroup updates a port group
	UpdatePortGroup(ctx context.Context, symID string, portGroupID string, ports []types.PortKey) (*types.PortGroup, error)

	// ModifyMobilityForVolume allows enabling/disabling mobility id for the volume
	ModifyMobilityForVolume(ctx context.Context, symID string, volumeID string, mobility bool) (*types.Volume, error)
	// ExpandVolume expands the size of an existing volume
	ExpandVolume(ctx context.Context, symID string, volumeID string, rdfGNo int, volumeSize interface{}, capUnits ...string) (*types.Volume, error)
	// GetCreateVolInSGPayload returns a payload to create a volume in a storage group
	GetCreateVolInSGPayload(volumeSize interface{}, capUnit string, volumeName string, isSync, enableMobility bool, remoteSymID, storageGroupID string, opts ...http.Header) (payload interface{})
	// GetCreateVolInSGPayloadWithMetaDataHeaders(sizeInCylinders int, volumeName string, isSync bool, remoteSymID, remoteStorageGroupID string, metadata http.Header) (payload interface{})

	// GetRDFGroupList GetRDFGroupList fetches all RDF group
	GetRDFGroupList(ctx context.Context, symID string, queryParams types.QueryParams) (*types.RDFGroupList, error)
	// GetRDFGroupByID fetches RDF group information
	GetRDFGroupByID(ctx context.Context, symID, rdfGroup string) (*types.RDFGroup, error)

	// GetProtectedStorageGroup returns protected storage group given the storage group ID
	GetProtectedStorageGroup(ctx context.Context, symID, storageGroup string) (*types.RDFStorageGroup, error)
	// CreateSGReplica creates a storage group on remote array and protect them with given RDF Mode and a given source storage group
	CreateSGReplica(ctx context.Context, symID, remoteSymID, rdfMode, rdfGroupNo, sourceSG, remoteSGName, remoteServiceLevel string, bias bool) (*types.SGRDFInfo, error)
	// ExecuteReplicationActionOnSG executes supported replication based actions on the protected SG
	ExecuteReplicationActionOnSG(ctx context.Context, symID, action, storageGroup, rdfGroup string, force, exemptConsistency, bias bool) error

	// CreateRDFPair creates a volume replication pair
	CreateRDFPair(ctx context.Context, symID, rdfGroupNo, deviceID, rdfMode, rdfType string, establish, exemptConsistency bool) (*types.RDFDevicePairList, error)
	// GetRDFDevicePairInfo returns RDF volume information
	GetRDFDevicePairInfo(ctx context.Context, symID, rdfGroup, volumeID string) (*types.RDFDevicePair, error)
	// GetStorageGroupRDFInfo returns the of RDF info of protected storage group
	GetStorageGroupRDFInfo(ctx context.Context, symID, sgName, rdfGroupNo string) (*types.StorageGroupRDFG, error)
	// GetFreeLocalAndRemoteRDFg returns list of Local and Remote Free RDFg in the array
	GetFreeLocalAndRemoteRDFg(ctx context.Context, localSymmID string, remoteSymmID string) (*types.NextFreeRDFGroup, error)
	// ExecuteCreateRDFGroup creates a new RDF group based on payload
	ExecuteCreateRDFGroup(ctx context.Context, symID string, CreateRDFPayload *types.RDFGroupCreate) error
	// GetLocalOnlineRDFDirs returns a List of ONLINE RDF Directors for a given array
	GetLocalOnlineRDFDirs(ctx context.Context, localSymID string) (*types.RDFDirList, error)
	// GetLocalOnlineRDFPorts returs List of ONLINE RDF Ports associated for a given ONLINE RDF Director
	GetLocalOnlineRDFPorts(ctx context.Context, rdfDir string, localSymID string) (*types.RDFPortList, error)
	// GetRemoteRDFPortOnSAN returns an array of Remote RDF Ports on the SAN that are connected to given local RDF Dir:Port
	GetRemoteRDFPortOnSAN(ctx context.Context, localSymID string, rdfDir string, rdfPort string) (*types.RemoteRDFPortDetails, error)
	// GetLocalRDFPortDetails returns details about the local RDFDir:port
	GetLocalRDFPortDetails(ctx context.Context, localSymID string, rdfDir string, rdfPort int) (*types.RDFPortDetails, error)

	// GetStorageGroupMetrics returns the list of required metrics
	GetStorageGroupMetrics(ctx context.Context, symID string, storageGroupID string, metricsQuery []string, firstAvailableDate int64, lastAvailableTime int64) (*types.StorageGroupMetricsIterator, error)
	// GetVolumesMetrics returns the list of volume metrics for specific storage groups
	GetVolumesMetrics(ctx context.Context, symID string, storageGroups string, metricsQuery []string, firstAvailableDate int64, lastAvailableTime int64) (*types.VolumeMetricsIterator, error)
	// GetStorageGroupPerfKeys returns the performance keys of storage group
	GetStorageGroupPerfKeys(ctx context.Context, symID string) (*types.StorageGroupKeysResult, error)
	// GetArrayPerfKeys returns the performance keys of array
	GetArrayPerfKeys(ctx context.Context) (*types.ArrayKeysResult, error)
	// GetVolumesMetricsByID returns a given Volume performance metrics
	GetVolumesMetricsByID(ctx context.Context, symID string, volID string, metricsQuery []string, firstAvailableTime, lastAvailableTime int64) (*types.VolumeMetricsIterator, error)
	// GetFileSystemMetricsByID returns a given FileSystem performance metrics
	GetFileSystemMetricsByID(ctx context.Context, symID string, fsID string, metricsQuery []string, firstAvailableTime, lastAvailableTime int64) (*types.FileSystemMetricsIterator, error)

	// CreateMigrationEnvironment creates a migration environment
	CreateMigrationEnvironment(ctx context.Context, sourceSymID, remoteSymID string) (*types.MigrationEnv, error)
	// CreateSGMigration create migration session on a storage group
	CreateSGMigration(ctx context.Context, localSymID, remoteSymID, storageGroup string) (*types.MigrationSession, error)
	// ModifyMigrationSession updates a migration session on a storage group
	ModifyMigrationSession(ctx context.Context, localSymID, action, storageGroup string) error
	// DeleteMigrationEnvironment deletes a migration environment
	DeleteMigrationEnvironment(ctx context.Context, localSymID, remoteSymID string) error
	// GetMigrationEnvironment returns a migration environment
	GetMigrationEnvironment(ctx context.Context, localSymID, remoteSymID string) (*types.MigrationEnv, error)
	// MigrateStorageGroup creates a Storage Group given the storageGroupID (name), srpID (storage resource pool), service level, and boolean for thick volumes.
	// If srpID is "None" then serviceLevel and thickVolumes settings are ignored
	MigrateStorageGroup(ctx context.Context, symID, storageGroupID, srpID, serviceLevel string, thickVolumes bool) (*types.StorageGroup, error)

	// GetStorageGroupMigration returns migration sessions on the array
	GetStorageGroupMigration(ctx context.Context, localSymID string) (*types.MigrationStorageGroups, error)
	// GetStorageGroupMigrationByID returns migration details for a storage group
	GetStorageGroupMigrationByID(ctx context.Context, localSymID, storageGroupID string) (*types.MigrationSession, error)

	// GetSnapshotPolicy returns a SnapshotPolicy given the Symmetrix ID and SnapshotPolicy ID (which is really a name).
	GetSnapshotPolicy(ctx context.Context, symID string, snapshotPolicyID string) (*types.SnapshotPolicy, error)
	// GetSnapshotPolicyList returns all the SnapshotPolicy names given the Symmetrix ID
	GetSnapshotPolicyList(ctx context.Context, symID string) (*types.SnapshotPolicyList, error)
	// DeleteSnapshotPolicy deletes a SnapshotPolicy entry.
	DeleteSnapshotPolicy(ctx context.Context, symID string, snapshotPolicyID string) error
	// CreateSnapshotPolicy creates a Snapshot policy and returns a types.SnapshotPolicy.
	CreateSnapshotPolicy(ctx context.Context, symID string, snapshotPolicyID string, interval string, offsetMins int32, complianceCountWarn int64,
		complianceCountCritical int64, optionalPayload map[string]interface{}) (*types.SnapshotPolicy, error)
	// UpdateSnapshotPolicy is a general method to update a SnapshotPolicy (PUT operation) based on the action using a UpdateSnapshotPolicyPayload.
	UpdateSnapshotPolicy(ctx context.Context, symID string, action string, snapshotPolicyID string, optionalPayload map[string]interface{}) error

	// GetFileSystemList get file system list on a symID
	GetFileSystemList(ctx context.Context, symID string, query types.QueryParams) (*types.FileSystemIterator, error)
	// GetFileSystemByID get file system  on a symID
	GetFileSystemByID(ctx context.Context, symID, fsID string) (*types.FileSystem, error)
	// CreateFileSystem creates a file system
	CreateFileSystem(ctx context.Context, symID, name, nasServer, serviceLevel string, sizeInMiB int64) (*types.FileSystem, error)
	// ModifyFileSystem  updates a File system
	ModifyFileSystem(ctx context.Context, symID, fsID string, payload types.ModifyFileSystem) (*types.FileSystem, error)
	// DeleteFileSystem deletes a file system
	DeleteFileSystem(ctx context.Context, symID, fsID string) error
	// GetNFSExportList get NFS export list on a symID
	GetNFSExportList(ctx context.Context, symID string, query types.QueryParams) (*types.NFSExportIterator, error)
	// GetNFSExportByID get file system  on a symID
	GetNFSExportByID(ctx context.Context, symID, nfsExportID string) (*types.NFSExport, error)
	// CreateNFSExport creates a NFSExport
	CreateNFSExport(ctx context.Context, symID string, createNFSExportPayload types.CreateNFSExport) (*types.NFSExport, error)
	// ModifyNFSExport updates a NFS export
	ModifyNFSExport(ctx context.Context, symID, nfsExportID string, payload types.ModifyNFSExport) (*types.NFSExport, error)
	// DeleteNFSExport deletes a nfs export
	DeleteNFSExport(ctx context.Context, symID, nfsExportID string) error
	// GetNASServerList get NAS Server list on a symID
	GetNASServerList(ctx context.Context, symID string, query types.QueryParams) (*types.NASServerIterator, error)
	// GetNASServerByID fetch specific NAS server on a symID
	GetNASServerByID(ctx context.Context, symID, nasID string) (*types.NASServer, error)
	// ModifyNASServer updates a NAS Server
	ModifyNASServer(ctx context.Context, symID, nasID string, payload types.ModifyNASServer) (*types.NASServer, error)
	// DeleteNASServer deletes a NAS Server
	DeleteNASServer(ctx context.Context, symID, nasID string) error
	// GetFileInterfaceByID gets a FileInterface
	GetFileInterfaceByID(ctx context.Context, symID, interfaceID string) (*types.FileInterface, error)

	// RefreshSymmetrix refreshes cache on the symID
	RefreshSymmetrix(ctx context.Context, symID string) error

	// GetNFSServerList get NFS Server list on a symID
	GetNFSServerList(ctx context.Context, symID string) (*types.NFSServerIterator, error)

	// GetNFSServerByID fetch specific NFS server on symID
	GetNFSServerByID(ctx context.Context, symID, nfsID string) (*types.NFSServer, error)
}
