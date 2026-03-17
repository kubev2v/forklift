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

	//	"infinibox-csi-driver/api/client"

	"github.com/stretchr/testify/mock"
)

type MockAPIService struct {
	mock.Mock
	Client
}

type MockAPIClient struct {
	mock.Mock
}

// GetAllPools mock
func (m *MockAPIService) GetPoolByName(name string) (*PoolResult, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*PoolResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetPoolByID(id int) (*PoolResult, error) {
	args := m.Called(id)
	resp, _ := args.Get(0).(*PoolResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetHostByName(name string) (*Host, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetSystem() (*SystemDetails, error) {
	args := m.Called()
	resp, _ := args.Get(0).(*SystemDetails)
	err, _ := args.Get(1).(error)
	return resp, err
}

// DeleteHost mock
func (m *MockAPIService) DeleteHost(hostID int) (*Host, error) {
	args := m.Called(hostID)
	resp, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetMetadata(objectID int) ([]GetMetadataResult, error) {
	args := m.Called(objectID)
	resp, _ := args.Get(0).([]GetMetadataResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetAllLunByHost(hostID int) ([]LunInfo, error) {
	args := m.Called(hostID)
	resp, _ := args.Get(0).([]LunInfo)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) PutMetadata(objectID int, metadata map[string]interface{}) (*PutMetadataResponse, error) {
	args := m.Called(objectID, metadata)
	res, _ := args.Get(0).(PutMetadataResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) DeleteMetadata(objectID int) (*DeleteMetadataResponse, error) {
	args := m.Called(objectID)
	res, _ := args.Get(0).(DeleteMetadataResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) CreateVolume(request CreateVolumeRequest) (*Volume, error) {
	args := m.Called(request)
	res, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return res, err
}
func (m *MockAPIService) CreateHost(name string) (*Host, error) {
	args := m.Called(name)
	res, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return res, err
}

func (m *MockAPIService) DeleteVolume(objectID int) (*DeleteVolumeResponse, error) {
	args := m.Called(objectID)
	res, _ := args.Get(0).(DeleteVolumeResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) GetVolumeByName(volumeName string) (*Volume, error) {
	args := m.Called(volumeName)
	resp, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) UpdateVolume(volumeID int, volume Volume) (*Volume, error) {
	args := m.Called(volumeID, volume)
	res, _ := args.Get(0).(Volume)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) GetVolume(volumeID int) (*Volume, error) {
	args := m.Called(volumeID)
	resp, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetFileSystemByID(fsID int) (*FileSystem, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).(*FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetExportByID(fsID int) (*Export, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).(*Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetExportsByFileSystemID(fsID int) ([]Export, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).([]Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteExport(exportID int) (*Export, error) {
	args := m.Called(exportID)
	resp, _ := args.Get(0).(*Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateExport(request CreateExportRequest) (*Export, error) {
	args := m.Called(request)
	res, _ := args.Get(0).(Export)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) CreateFileSystem(request CreateFileSystemRequest) (*FileSystem, error) {
	args := m.Called(request)
	var resp FileSystem
	if args.Get(0) != nil {
		resp, _ = args.Get(0).(FileSystem)
	}
	var err error
	if args.Get(1) != nil {
		err, _ = args.Get(1).(error)
	}
	return &resp, err
}

func (m *MockAPIService) GetFileSystemByName(name string) (*FileSystem, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetFileSystemsByPool(poolID int, fsPrefix string) ([]FileSystem, error) {
	args := m.Called(poolID, fsPrefix)
	resp, _ := args.Get(0).([]FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}
func (m *MockAPIService) GetFileSystemsByParentID(parentID int) ([]FileSystem, error) {
	args := m.Called(parentID)
	resp, _ := args.Get(0).([]FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetNtpStatus() ([]NtpStatus, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]NtpStatus)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetMaxTreeqPerFs() (int, error) {
	args := m.Called()
	resp, _ := args.Get(0).(int)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetFCPorts() ([]FCNode, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]FCNode)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetNetworkSpaceByName(name string) (*NetworkSpace, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*NetworkSpace)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteFileSystem(fsID int) error {
	args := m.Called(fsID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockAPIService) UpdateFileSystem(fsID int, fs FileSystem) (*FileSystem, error) {
	args := m.Called(fsID, fs)
	res, _ := args.Get(0).(FileSystem)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) GetTreeqByName(fsID int, name string) (*Treeq, error) {
	args := m.Called(fsID, name)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetTreeq(fsID int, treeqID int) (*Treeq, error) {
	args := m.Called(fsID, treeqID)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteTreeq(fsID, treeqID int) (*Treeq, error) {
	args := m.Called(fsID, treeqID)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateTreeq(fsID int, treeqRequest CreateTreeqRequest) (*Treeq, error) {
	args := m.Called(fsID, treeqRequest)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) UpdateTreeq(fsID, treeqID int, updateRequest UpdateTreeqRequest) (*Treeq, error) {
	args := m.Called(fsID, treeqID, updateRequest)
	resp, _ := args.Get(0).(Treeq)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetTreeqsByFileSystem(fsID int) ([]Treeq, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).([]Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateSnapshotVolume(snapshotParam CreateSnapshotVolumeRequest) (*Snapshot, error) {
	args := m.Called(snapshotParam)
	resp, _ := args.Get(0).(Snapshot)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetVolumesByParentID(parentID int) ([]Volume, error) {
	args := m.Called(parentID)
	resp, _ := args.Get(0).([]Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) MapVolumeToHost(hostID, volumeID, lun int) (*LunInfo, error) {
	args := m.Called(hostID, volumeID, lun)
	resp, _ := args.Get(0).(LunInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetLunByHostVolume(hostID, volumeID int) (*LunInfo, error) {
	args := m.Called(hostID, volumeID)
	resp, _ := args.Get(0).(LunInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) UnMapVolumeFromHost(hostID, volumeID int) (*UnMapVolumeFromHostResponse, error) {
	args := m.Called(hostID, volumeID)
	resp, _ := args.Get(0).(UnMapVolumeFromHostResponse)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetConsistencyGroup(cgID int) (*ConsistencyGroupInfo, error) {
	args := m.Called(cgID)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) DeleteConsistencyGroup(cgID int) error {
	args := m.Called(cgID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockAPIService) GetConsistencyGroupByName(name string) (*ConsistencyGroupInfo, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) CreateSnapshotGroup(req CreateSnapshotGroupRequest) (*ConsistencyGroupInfo, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}
func (m *MockAPIService) GetMembersByCGID(cgID int) ([]MemberInfo, error) {
	args := m.Called(cgID)
	resp, _ := args.Get(0).([]MemberInfo)
	err, _ := args.Get(1).(error)
	return resp, err
}
func (m *MockAPIService) AddMemberToSnapshotGroup(volumeID, cgID int) error {
	args := m.Called(volumeID, cgID)
	err, _ := args.Get(0).(error)
	return err
}
func (m *MockAPIService) CreateConsistencyGroup(req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetLinks() ([]Link, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]Link)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateReplica(req CreateReplicaRequest) (*Replica, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(Replica)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetReplicas() ([]Replica, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]Replica)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteReplica(replicaID int) error {
	args := m.Called(replicaID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockAPIService) GetReplica(id int) (*Replica, error) {
	args := m.Called(id)
	resp, _ := args.Get(0).(Replica)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) UpdateExportPermissions(export Export, exportPathRef ExportPathRef) (*Export, error) {
	args := m.Called(export, exportPathRef)
	res, _ := args.Get(0).(Export)
	err, _ := args.Get(1).(error)
	return &res, err
}
func (m *MockAPIService) CreateFileSystemSnapshot(params FileSystemSnapshot) (*FileSystemSnapshotResponse, error) {
	args := m.Called(params)
	resp, _ := args.Get(0).(FileSystemSnapshotResponse)
	err, _ := args.Get(1).(error)
	return &resp, err
}
