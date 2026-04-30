// Copyright © 2019 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// GetInstance returns an instance
func (c *Client) GetInstance(systemhref string) ([]*types.System, error) {
	defer TimeSpent("GetInstance", time.Now())

	var (
		err     error
		system  = &types.System{}
		systems []*types.System
	)

	if systemhref == "" {
		err = c.getJSONWithRetry(
			http.MethodGet, "api/types/System/instances", nil, &systems)
	} else {
		err = c.getJSONWithRetry(
			http.MethodGet, systemhref, nil, system)
	}
	if err != nil {
		return nil, err
	}

	if systemhref != "" {
		systems = append(systems, system)
	}

	return systems, nil
}

// GetVolume returns a volume
func (c *Client) GetVolume(
	volumehref, volumeid, ancestorvolumeid, volumename string,
	getSnapshots bool,
) ([]*types.Volume, error) {
	defer TimeSpent("GetVolume", time.Now())

	var (
		err     error
		path    string
		volume  = &types.Volume{}
		volumes []*types.Volume
	)

	if volumename != "" {
		volumeid, err = c.FindVolumeID(volumename)
		if err != nil && err.Error() == "Not found" {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("Error: problem finding volume: %s", err)
		}
	}

	if volumeid != "" {
		path = fmt.Sprintf("/api/instances/Volume::%s", volumeid)
	} else if volumehref == "" {
		path = "/api/types/Volume/instances"
	} else {
		path = volumehref
	}

	if volumehref == "" && volumeid == "" {
		err = c.getJSONWithRetry(
			http.MethodGet, path, nil, &volumes)
	} else {
		err = c.getJSONWithRetry(
			http.MethodGet, path, nil, volume)
	}
	if err != nil {
		return nil, err
	}

	if volumehref == "" && volumeid == "" {
		var volumesNew []*types.Volume
		for _, volume := range volumes {
			if (!getSnapshots && volume.AncestorVolumeID == ancestorvolumeid) || (getSnapshots && volume.AncestorVolumeID != "") {
				volumesNew = append(volumesNew, volume)
			}
		}
		volumes = volumesNew
	} else {
		volumes = append(volumes, volume)
	}
	return volumes, nil
}

// FindVolumeID returns a VolumeID
func (c *Client) FindVolumeID(volumename string) (string, error) {
	return findVolumeIDFunc(c, volumename)
}

var findVolumeIDFunc = func(c *Client, volumename string) (string, error) {
	defer TimeSpent("FindVolumeID", time.Now())

	volumeQeryIDByKeyParam := &types.VolumeQeryIDByKeyParam{
		Name: volumename,
	}

	path := "/api/types/Volume/instances/action/queryIdByKey"

	volumeID, err := c.getStringWithRetry(http.MethodPost, path,
		volumeQeryIDByKeyParam)
	fmt.Printf("[FindVolumeID] volumeID: %+v\n", volumeID)
	if err != nil {
		return "", err
	}

	return volumeID, nil
}

// CreateVolume creates a volume
func (c *Client) CreateVolume(
	volume *types.VolumeParam,
	storagePoolName, protectionDomain string,
) (*types.VolumeResp, error) {
	defer TimeSpent("CreateVolume", time.Now())

	path := "/api/types/Volume/instances"

	storagePool, err := c.FindStoragePool("", storagePoolName, "", protectionDomain)
	if err != nil {
		return nil, err
	}

	volume.StoragePoolID = storagePool.ID
	volume.ProtectionDomainID = storagePool.ProtectionDomainID

	vol := &types.VolumeResp{}
	err = c.getJSONWithRetry(
		http.MethodPost, path, volume, vol)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

// GetStoragePool returns a storagepool
func (c *Client) GetStoragePool(
	storagepoolhref string,
) ([]*types.StoragePool, error) {
	defer TimeSpent("GetStoragePool", time.Now())

	var (
		err          error
		storagePool  = &types.StoragePool{}
		storagePools []*types.StoragePool
	)

	if storagepoolhref == "" {
		err = c.getJSONWithRetry(
			http.MethodGet, "/api/types/StoragePool/instances",
			nil, &storagePools)
	} else {
		err = c.getJSONWithRetry(
			http.MethodGet, storagepoolhref, nil, storagePool)
	}
	if err != nil {
		return nil, err
	}

	if storagepoolhref != "" {
		storagePools = append(storagePools, storagePool)
	}
	return storagePools, nil
}

// FindStoragePool returns a StoragePool
func (c *Client) FindStoragePool(
	id, name, href, protectionDomain string,
) (*types.StoragePool, error) {
	defer TimeSpent("FindStoragePool", time.Now())

	storagePools, err := c.GetStoragePool(href)
	if err != nil {
		return nil, fmt.Errorf("Error getting storage pool %s", err)
	}

	for _, storagePool := range storagePools {
		if storagePool.ID == id || storagePool.Name == name || href != "" {
			if storagePool.ProtectionDomainID == protectionDomain || protectionDomain == "" {
				return storagePool, nil
			}
		}
	}

	return nil, errors.New("Couldn't find storage pool")
}

// SnapshotPolicy defines struct for SnapshotPolicy
type SnapshotPolicy struct {
	SnapshotPolicy *types.SnapshotPolicy
	client         *Client
}

// NewSnapshotPolicy returns new SnapshotPolicy
func NewSnapshotPolicy(client *Client) *SnapshotPolicy {
	return &SnapshotPolicy{
		SnapshotPolicy: &types.SnapshotPolicy{},
		client:         client,
	}
}

// FindSnapshotPolicyID retruns a Snapshot Policy ID based on name
func (c *Client) FindSnapshotPolicyID(spname string) (string, error) {
	return findSnapshotPolicyByIDFunc(c, spname)
}

var findSnapshotPolicyByIDFunc = func(c *Client, spid string) (string, error) {
	defer TimeSpent("FindSnapshotPolicyID", time.Now())

	SnapshotPolicyQueryIDByKeyParam := &types.SnapshotPolicyQueryIDByKeyParam{
		Name: spid,
	}

	path := fmt.Sprintf("/api/types/SnapshotPolicy/instances/action/queryIdByKey")

	spID, err := c.getStringWithRetry(
		http.MethodPost, path, SnapshotPolicyQueryIDByKeyParam)
	if err != nil {
		return "", err
	}

	return spID, nil
}

// GetSnapshotPolicy returns a list of snapshot policy
func (c *Client) GetSnapshotPolicy(
	spname, spid string,
) ([]*types.SnapshotPolicy, error) {
	defer TimeSpent("GetSnapshotPolicy", time.Now())

	var (
		err  error
		path string
		sp   = &types.SnapshotPolicy{}
		sps  []*types.SnapshotPolicy
	)

	if spname != "" {
		spid, err = c.FindSnapshotPolicyID(spname)
		if err != nil && err.Error() == "Not found" {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("Error: problem finding snapshot policy: %s", err)
		}
	}

	if spid != "" {
		path = fmt.Sprintf("/api/instances/SnapshotPolicy::%s", spid)
	} else {
		path = "/api/types/SnapshotPolicy/instances"
	}

	if spid == "" {
		err = c.getJSONWithRetry(
			http.MethodGet, path, nil, &sps)
	} else {
		err = c.getJSONWithRetry(
			http.MethodGet, path, nil, sp)
	}
	if err != nil {
		return nil, err
	}

	if spid == "" {
		return sps, nil
	}
	sps = append(sps, sp)
	return sps, nil
}

// GetStoragePoolVolumes returns list of volumes connected to storage pool Storagepool by ID
func (c *Client) GetStoragePoolVolumes(id string) ([]*types.Volume, error) {
	defer TimeSpent("GetStoragePoolByID", time.Now())

	path := fmt.Sprintf("/api/instances/StoragePool::%s/relationships/Volume", id)
	var storagepoolVolumes []*types.Volume
	err := c.getJSONWithRetry(
		http.MethodGet, path, nil, &storagepoolVolumes)
	if err != nil {
		return nil, err
	}

	return storagepoolVolumes, err
}
