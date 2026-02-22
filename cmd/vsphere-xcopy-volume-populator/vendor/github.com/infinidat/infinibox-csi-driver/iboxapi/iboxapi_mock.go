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

	"context"

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
func (m *MockAPIService) GetPoolByName(ctx context.Context, name string) (*PoolResult, error) {
	args := m.Called(ctx, name)
	resp, _ := args.Get(0).(*PoolResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetPoolByID(ctx context.Context, id int) (*PoolResult, error) {
	args := m.Called(ctx, id)
	resp, _ := args.Get(0).(*PoolResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetHostByName(ctx context.Context, name string) (*Host, error) {
	args := m.Called(ctx, name)
	resp, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetSystem(ctx context.Context) (*SystemDetails, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).(*SystemDetails)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteHost(ctx context.Context, hostID int) (*Host, error) {
	args := m.Called(ctx, hostID)
	resp, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetMetadata(ctx context.Context, objectID int) ([]GetMetadataResult, error) {
	args := m.Called(ctx, objectID)
	resp, _ := args.Get(0).([]GetMetadataResult)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetAllLunByHost(ctx context.Context, hostID int) ([]LunInfo, error) {
	args := m.Called(ctx, hostID)
	resp, _ := args.Get(0).([]LunInfo)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) PutMetadata(ctx context.Context, objectID int, metadata map[string]any) (*PutMetadataResponse, error) {
	args := m.Called(ctx, objectID, metadata)
	res, _ := args.Get(0).(PutMetadataResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) DeleteMetadata(ctx context.Context, objectID int) (*DeleteMetadataResponse, error) {
	args := m.Called(ctx, objectID)
	res, _ := args.Get(0).(DeleteMetadataResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) CreateVolume(ctx context.Context, request CreateVolumeRequest) (*Volume, error) {
	args := m.Called(ctx, request)
	res, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return res, err
}
func (m *MockAPIService) PromoteSnapshot(ctx context.Context, snapshotID int) (*Volume, error) {
	args := m.Called(ctx, snapshotID)
	res, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return res, err
}
func (m *MockAPIService) CreateHost(ctx context.Context, name string) (*Host, error) {
	args := m.Called(ctx, name)
	res, _ := args.Get(0).(*Host)
	err, _ := args.Get(1).(error)
	return res, err
}

func (m *MockAPIService) DeleteVolume(ctx context.Context, objectID int) (*DeleteVolumeResponse, error) {
	args := m.Called(ctx, objectID)
	res, _ := args.Get(0).(DeleteVolumeResponse)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) GetVolumeByName(ctx context.Context, volumeName string) (*Volume, error) {
	args := m.Called(ctx, volumeName)
	resp, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) UpdateVolume(ctx context.Context, volumeID int, volume Volume) (*Volume, error) {
	args := m.Called(ctx, volumeID, volume)
	res, _ := args.Get(0).(Volume)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) GetVolume(ctx context.Context, volumeID int) (*Volume, error) {
	args := m.Called(ctx, volumeID)
	resp, _ := args.Get(0).(*Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetFileSystemByID(ctx context.Context, fsID int) (*FileSystem, error) {
	args := m.Called(ctx, fsID)
	resp, _ := args.Get(0).(*FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetExportByID(ctx context.Context, fsID int) (*Export, error) {
	args := m.Called(ctx, fsID)
	resp, _ := args.Get(0).(*Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetExportsByFileSystemID(ctx context.Context, fsID int) ([]Export, error) {
	args := m.Called(ctx, fsID)
	resp, _ := args.Get(0).([]Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteExport(ctx context.Context, exportID int) (*Export, error) {
	args := m.Called(ctx, exportID)
	resp, _ := args.Get(0).(*Export)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateExport(ctx context.Context, request CreateExportRequest) (*Export, error) {
	args := m.Called(ctx, request)
	res, _ := args.Get(0).(Export)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) CreateFileSystem(ctx context.Context, request CreateFileSystemRequest) (*FileSystem, error) {
	args := m.Called(ctx, request)
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

func (m *MockAPIService) GetFileSystemByName(ctx context.Context, name string) (*FileSystem, error) {
	args := m.Called(ctx, name)
	resp, _ := args.Get(0).(*FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetFileSystemsByPool(ctx context.Context, poolID int, fsPrefix string) ([]FileSystem, error) {
	args := m.Called(ctx, poolID, fsPrefix)
	resp, _ := args.Get(0).([]FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}
func (m *MockAPIService) GetFileSystemsByParentID(ctx context.Context, parentID int) ([]FileSystem, error) {
	args := m.Called(ctx, parentID)
	resp, _ := args.Get(0).([]FileSystem)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetNtpStatus(ctx context.Context) ([]NtpStatus, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).([]NtpStatus)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetMaxTreeqPerFs(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).(int)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetFCPorts(ctx context.Context) ([]FCNode, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).([]FCNode)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetNetworkSpaceByName(ctx context.Context, name string) (*NetworkSpace, error) {
	args := m.Called(ctx, name)
	resp, _ := args.Get(0).(*NetworkSpace)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteFileSystem(ctx context.Context, fsID int) error {
	args := m.Called(ctx, fsID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockAPIService) UpdateFileSystem(ctx context.Context, fsID int, fs FileSystem) (*FileSystem, error) {
	args := m.Called(ctx, fsID, fs)
	res, _ := args.Get(0).(FileSystem)
	err, _ := args.Get(1).(error)
	return &res, err
}

func (m *MockAPIService) GetTreeqByName(ctx context.Context, fsID int, name string) (*Treeq, error) {
	args := m.Called(ctx, fsID, name)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) GetTreeq(ctx context.Context, fsID int, treeqID int) (*Treeq, error) {
	args := m.Called(ctx, fsID, treeqID)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteTreeq(ctx context.Context, fsID, treeqID int) (*Treeq, error) {
	args := m.Called(ctx, fsID, treeqID)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateTreeq(ctx context.Context, fsID int, treeqRequest CreateTreeqRequest) (*Treeq, error) {
	args := m.Called(ctx, fsID, treeqRequest)
	resp, _ := args.Get(0).(*Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) UpdateTreeq(ctx context.Context, fsID, treeqID int, updateRequest UpdateTreeqRequest) (*Treeq, error) {
	args := m.Called(ctx, fsID, treeqID, updateRequest)
	resp, _ := args.Get(0).(Treeq)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetTreeqsByFileSystem(ctx context.Context, fsID int) ([]Treeq, error) {
	args := m.Called(ctx, fsID)
	resp, _ := args.Get(0).([]Treeq)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateSnapshotVolume(ctx context.Context, snapshotParam CreateSnapshotVolumeRequest) (*Snapshot, error) {
	args := m.Called(ctx, snapshotParam)
	resp, _ := args.Get(0).(Snapshot)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetVolumesByParentID(ctx context.Context, parentID int) ([]Volume, error) {
	args := m.Called(ctx, parentID)
	resp, _ := args.Get(0).([]Volume)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) MapVolumeToHost(ctx context.Context, hostID, volumeID, lun int) (*LunInfo, error) {
	args := m.Called(ctx, hostID, volumeID, lun)
	resp, _ := args.Get(0).(LunInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetLunByHostVolume(ctx context.Context, hostID, volumeID int) (*LunInfo, error) {
	args := m.Called(ctx, hostID, volumeID)
	resp, _ := args.Get(0).(LunInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) UnMapVolumeFromHost(ctx context.Context, hostID, volumeID int) (*UnMapVolumeFromHostResponse, error) {
	args := m.Called(ctx, hostID, volumeID)
	resp, _ := args.Get(0).(UnMapVolumeFromHostResponse)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetConsistencyGroup(ctx context.Context, cgID int) (*ConsistencyGroupInfo, error) {
	args := m.Called(ctx, cgID)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) DeleteConsistencyGroup(ctx context.Context, cgID int) error {
	args := m.Called(ctx, cgID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockAPIService) GetConsistencyGroupByName(ctx context.Context, name string) (*ConsistencyGroupInfo, error) {
	args := m.Called(ctx, name)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) CreateSnapshotGroup(ctx context.Context, req CreateSnapshotGroupRequest) (*ConsistencyGroupInfo, error) {
	args := m.Called(ctx, req)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}
func (m *MockAPIService) GetMembersByCGID(ctx context.Context, cgID int) ([]MemberInfo, error) {
	args := m.Called(ctx, cgID)
	resp, _ := args.Get(0).([]MemberInfo)
	err, _ := args.Get(1).(error)
	return resp, err
}
func (m *MockAPIService) AddMemberToSnapshotGroup(ctx context.Context, volumeID, cgID int) error {
	args := m.Called(ctx, volumeID, cgID)
	err, _ := args.Get(0).(error)
	return err
}
func (m *MockAPIService) CreateConsistencyGroup(ctx context.Context, req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error) {
	args := m.Called(ctx, req)
	resp, _ := args.Get(0).(ConsistencyGroupInfo)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetLinks(ctx context.Context) ([]Link, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).([]Link)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) CreateReplica(ctx context.Context, req CreateReplicaRequest) (*Replica, error) {
	args := m.Called(ctx, req)
	resp, _ := args.Get(0).(Replica)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) GetReplicas(ctx context.Context) ([]Replica, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).([]Replica)
	err, _ := args.Get(1).(error)
	return resp, err
}

func (m *MockAPIService) DeleteReplica(ctx context.Context, replicaID int) error {
	args := m.Called(ctx, replicaID)
	err, _ := args.Get(0).(error)
	return err
}

func (m *MockAPIService) GetReplica(ctx context.Context, id int) (*Replica, error) {
	args := m.Called(ctx, id)
	resp, _ := args.Get(0).(Replica)
	err, _ := args.Get(1).(error)
	return &resp, err
}

func (m *MockAPIService) UpdateExportPermissions(ctx context.Context, export Export, exportPathRef ExportPathRef) (*Export, error) {
	args := m.Called(ctx, export, exportPathRef)
	res, _ := args.Get(0).(Export)
	err, _ := args.Get(1).(error)
	return &res, err
}
func (m *MockAPIService) CreateFileSystemSnapshot(ctx context.Context, params FileSystemSnapshot) (*FileSystemSnapshotResponse, error) {
	args := m.Called(ctx, params)
	resp, _ := args.Get(0).(FileSystemSnapshotResponse)
	err, _ := args.Get(1).(error)
	return &resp, err
}
