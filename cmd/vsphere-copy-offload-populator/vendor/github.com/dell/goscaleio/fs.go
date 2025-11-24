// Copyright © 2019 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// FileSystem defines struct for file system
type FileSystem struct {
	FileSystem *types.FileSystem
	client     *Client
}

// NewFileSystem returns a new file system
func NewFileSystem(client *Client, fs *types.FileSystem) *FileSystem {
	return &FileSystem{
		FileSystem: fs,
		client:     client,
	}
}

// GetAllFileSystems returns a file system
func (s *System) GetAllFileSystems() ([]types.FileSystem, error) {
	defer TimeSpent("GetAllFileSystems", time.Now())

	path := fmt.Sprintf("/rest/v1/file-systems?select=*")
	var fs []types.FileSystem
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &fs)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

// GetFileSystemByIDName returns a file system by Name or ID
func (s *System) GetFileSystemByIDName(id string, name string) (*types.FileSystem, error) {
	defer TimeSpent("GetFileSystemByIDName", time.Now())

	if id == "" && name == "" {
		return nil, errors.New("file system name or ID is mandatory, please enter a valid value")
	}

	// Get filesystem by id
	if id != "" {
		path := fmt.Sprintf("/rest/v1/file-systems/%v?select=*", id)
		var fs types.FileSystem
		err := s.client.getJSONWithRetry(
			http.MethodGet, path, nil, &fs)
		if err != nil {
			return nil, errors.New("couldn't find filesystem by id")
		}

		return &fs, nil
	}

	// Get filesystem by name
	filesystems, err := s.GetAllFileSystems()
	if err != nil {
		return nil, err
	}

	for _, fs := range filesystems {
		if fs.Name == name {
			return &fs, nil
		}
	}

	return nil, errors.New("couldn't find file system by name")
}

// CreateFileSystem creates a file system
func (s *System) CreateFileSystem(fs *types.FsCreate) (*types.FileSystemResp, error) {
	defer TimeSpent("CreateFileSystem", time.Now())

	path := fmt.Sprintf("/rest/v1/file-systems")
	fsResponse := types.FileSystemResp{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, fs, &fsResponse)
	if err != nil {
		return nil, err
	}

	return &fsResponse, nil
}

// CreateFileSystemSnapshot creates a snapshot for a given file system
func (s *System) CreateFileSystemSnapshot(createSnapParam *types.CreateFileSystemSnapshotParam, fsID string) (*types.CreateFileSystemSnapshotResponse, error) {
	defer TimeSpent("CreateFileSystemSnapshot", time.Now())

	path := fmt.Sprintf("/rest/v1/file-systems/%v/snapshot", fsID)
	snapResponse := types.CreateFileSystemSnapshotResponse{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, createSnapParam, &snapResponse)
	if err != nil {
		return nil, err
	}

	return &snapResponse, nil
}

// RestoreFileSystemFromSnapshot restores the filesystem from a given snapshot using filesytem id
func (s *System) RestoreFileSystemFromSnapshot(restoreSnapParam *types.RestoreFsSnapParam, fsID string) (*types.RestoreFsSnapResponse, error) {
	defer TimeSpent("CreateFileSystemSnapshot", time.Now())

	path := fmt.Sprintf("/rest/v1/file-systems/%v/restore", fsID)

	restoreFsResponse := types.RestoreFsSnapResponse{}
	var err error
	if restoreSnapParam.CopyName == "" {
		err = s.client.getJSONWithRetry(
			http.MethodPost, path, restoreSnapParam, nil)
		if err == nil {
			return nil, nil
		}
	} else {
		err = s.client.getJSONWithRetry(
			http.MethodPost, path, restoreSnapParam, &restoreFsResponse)
		if err == nil {
			return &restoreFsResponse, nil
		}
	}

	if err != nil {
		return nil, err
	}

	return nil, nil
}

// GetFsSnapshotsByVolumeID gets list of snapshots associated with a filesystem
func (s *System) GetFsSnapshotsByVolumeID(fsID string) ([]types.FileSystem, error) {
	defer TimeSpent("GetFsSnapshotsByVolumeID", time.Now())
	var snapshotList []types.FileSystem
	fsList, err := s.GetAllFileSystems()
	if err != nil {
		return nil, err
	}
	for _, fs := range fsList {
		if fs.ParentID == fsID {
			snapshotList = append(snapshotList, fs)
		}
	}
	return snapshotList, err
}

// DeleteFileSystem deletes a file system
func (s *System) DeleteFileSystem(name string) error {
	defer TimeSpent("DeleteFileSystem", time.Now())

	fs, err := s.GetFileSystemByIDName("", name)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/rest/v1/file-systems/%v", fs.ID)

	err = s.client.getJSONWithRetry(
		http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// ModifyFileSystem modifies a file system
func (s *System) ModifyFileSystem(modifyFsParam *types.FSModify, id string) error {
	defer TimeSpent("ModifyFileSystem", time.Now())

	fs, err := s.GetFileSystemByIDName(id, "")
	if err != nil {
		return err
	}

	var body *types.FSModify = modifyFsParam
	path := fmt.Sprintf("/rest/v1/file-systems/%v", fs.ID)

	err = s.client.getJSONWithRetry(http.MethodPatch, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}
