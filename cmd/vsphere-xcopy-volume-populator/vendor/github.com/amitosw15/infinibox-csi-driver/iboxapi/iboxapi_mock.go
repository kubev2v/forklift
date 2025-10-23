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

	//	"github.com/amitosw15/infinibox-csi-driver/api/client"

	"github.com/stretchr/testify/mock"
)

type MockApiService struct {
	mock.Mock
	Client
}

type MockApiClient struct {
	mock.Mock
}

// GetAllPools mock
func (m *MockApiService) GetPoolByName(name string) (*PoolResult, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*PoolResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetPoolByID(id int) (*PoolResult, error) {
	args := m.Called(id)
	resp, _ := args.Get(0).(*PoolResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetHostByName(name string) (*Host, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return resp, err
}

// DeleteHost mock
func (m *MockApiService) DeleteHost(hostID int) (*Host, error) {
	args := m.Called(hostID)
	resp, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetMetadata(objectID int) ([]GetMetadataResult, error) {
	args := m.Called(objectID)
	resp, _ := args.Get(0).([]GetMetadataResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetAllLunByHost(hostID int) ([]LunInfo, error) {
	args := m.Called(hostID)
	resp, _ := args.Get(0).([]LunInfo)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) PutMetadata(objectID int, metadata map[string]interface{}) (*PutMetadataResponse, error) {
	args := m.Called(objectID, metadata)
	res, _ := args.Get(0).(PutMetadataResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockApiService) DeleteMetadata(objectID int) (*DeleteMetadataResponse, error) {
	args := m.Called(objectID)
	res, _ := args.Get(0).(DeleteMetadataResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockApiService) CreateVolume(request CreateVolumeRequest) (*Volume, error) {
	args := m.Called(request)
	res, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return res, err
}
func (m *MockApiService) CreateHost(name string) (*Host, error) {
	args := m.Called(name)
	res, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return res, err
}

func (m *MockApiService) DeleteVolume(objectID int) (*DeleteVolumeResponse, error) {
	args := m.Called(objectID)
	res, _ := args.Get(0).(DeleteVolumeResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockApiService) GetVolumeByName(volumeName string) (*Volume, error) {
	args := m.Called(volumeName)
	resp, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) UpdateVolume(volumeID int, volume Volume) (*Volume, error) {
	args := m.Called(volumeID, volume)
	res, _ := args.Get(0).(Volume)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockApiService) GetVolume(volumeID int) (*Volume, error) {
	args := m.Called(volumeID)
	resp, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetFileSystemByID(fsID int) (*FileSystem, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).(*FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetExportByID(fsID int) (*Export, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).(*Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetExportsByFileSystemID(fsID int) ([]Export, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).([]Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) DeleteExport(exportID int) (*Export, error) {
	args := m.Called(exportID)
	resp, _ := args.Get(0).(*Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) CreateExport(request CreateExportRequest) (*Export, error) {
	args := m.Called(request)
	res, _ := args.Get(0).(Export)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockApiService) CreateFileSystem(request CreateFileSystemRequest) (*FileSystem, error) {
	//args := m.Called(request)
	//res, _ := args.Get(0).(*FileSystem)
	//err, _ := args.Get(1).(error)
	//return res, err
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

func (m *MockApiService) GetFileSystemByName(name string) (*FileSystem, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetFileSystemsByPool(poolID int, fsPrefix string) ([]FileSystem, error) {
	args := m.Called(poolID, fsPrefix)
	resp, _ := args.Get(0).([]FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}
func (m *MockApiService) GetFileSystemsByParentID(parentID int) ([]FileSystem, error) {
	args := m.Called(parentID)
	resp, _ := args.Get(0).([]FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetNtpStatus() ([]NtpStatus, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]NtpStatus)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetMaxTreeqPerFs() (int, error) {
	args := m.Called()
	resp, _ := args.Get(0).(int)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetFCPorts() ([]FCNode, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]FCNode)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetNetworkSpaceByName(name string) (*NetworkSpace, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(*NetworkSpace)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) DeleteFileSystem(fsID int) error {
	args := m.Called(fsID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockApiService) UpdateFileSystem(fsID int, fs FileSystem) (*FileSystem, error) {
	args := m.Called(fsID, fs)
	res, _ := args.Get(0).(FileSystem)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockApiService) GetTreeqByName(fsID int, name string) (*Treeq, error) {
	args := m.Called(fsID, name)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) GetTreeq(fsID int, treeqID int) (*Treeq, error) {
	args := m.Called(fsID, treeqID)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) DeleteTreeq(fsID, treeqID int) (*Treeq, error) {
	args := m.Called(fsID, treeqID)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) CreateTreeq(fsID int, treeqRequest CreateTreeqRequest) (*Treeq, error) {
	args := m.Called(fsID, treeqRequest)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) UpdateTreeq(fsID, treeqID int, updateRequest UpdateTreeqRequest) (*Treeq, error) {
	args := m.Called(fsID, treeqID, updateRequest)
	resp, _ := args.Get(0).(Treeq)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) GetTreeqsByFileSystem(fsID int) ([]Treeq, error) {
	args := m.Called(fsID)
	resp, _ := args.Get(0).([]Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) CreateSnapshotVolume(snapshotParam CreateSnapshotVolumeRequest) (*Snapshot, error) {
	args := m.Called(snapshotParam)
	resp, _ := args.Get(0).(Snapshot)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) GetVolumesByParentID(parentID int) ([]Volume, error) {
	args := m.Called(parentID)
	resp, _ := args.Get(0).([]Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) MapVolumeToHost(hostID, volumeID, lun int) (*LunInfo, error) {
	args := m.Called(hostID, volumeID, lun)
	resp, _ := args.Get(0).(LunInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) GetLunByHostVolume(hostID, volumeID int) (*LunInfo, error) {
	args := m.Called(hostID, volumeID)
	resp, _ := args.Get(0).(LunInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) UnMapVolumeFromHost(hostID, volumeID int) (*UnMapVolumeFromHostResponse, error) {
	args := m.Called(hostID, volumeID)
	resp, _ := args.Get(0).(UnMapVolumeFromHostResponse)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) GetConsistencyGroup(cgID int) (*ConsistencyGroupInfo, error) {
	args := m.Called(cgID)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) DeleteConsistencyGroup(cgID int) error {
	args := m.Called(cgID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockApiService) GetConsistencyGroupByName(name string) (*ConsistencyGroupInfo, error) {
	args := m.Called(name)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) CreateSnapshotGroup(req CreateSnapshotGroupRequest) (*ConsistencyGroupInfo, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}
func (m *MockApiService) GetMembersByCGID(cgID int) ([]MemberInfo, error) {
	args := m.Called(cgID)
	resp, _ := args.Get(0).([]MemberInfo)
	err, _ := args.Get(1).(error)
	return resp, err
}
func (m *MockApiService) AddMemberToSnapshotGroup(volumeID, cgID int) error {
	args := m.Called(volumeID, cgID)
	err, _ := args.Get(0).(error)
	return err
}
func (m *MockApiService) CreateConsistencyGroup(req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) GetLinks() ([]Link, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]Link)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) CreateReplica(req CreateReplicaRequest) (*Replica, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(Replica)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) GetReplicas() ([]Replica, error) {
	args := m.Called()
	resp, _ := args.Get(0).([]Replica)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockApiService) DeleteReplica(replicaID int) error {
	args := m.Called(replicaID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockApiService) GetReplica(id int) (*Replica, error) {
	args := m.Called(id)
	resp, _ := args.Get(0).(Replica)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockApiService) UpdateExportPermissions(export Export, exportPathRef ExportPathRef) (*Export, error) {
	args := m.Called(export, exportPathRef)
	res, _ := args.Get(0).(Export)
	err, _ := args.Get(1).(error)
	return &res, err
}
func (m *MockApiService) CreateFileSystemSnapshot(params FileSystemSnapshot) (*FileSystemSnapshotResponse, error) {
	args := m.Called(params)
	resp, _ := args.Get(0).(FileSystemSnapshotResponse)
	err, _ := args.Get(1).(error)
	return &resp, err
}
