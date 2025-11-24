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

// StoragePool struct defines struct for StoragePool
type StoragePool struct {
	StoragePool *types.StoragePool
	client      *Client
}

// NewStoragePool returns a new StoragePool
func NewStoragePool(client *Client) *StoragePool {
	return &StoragePool{
		StoragePool: &types.StoragePool{},
		client:      client,
	}
}

// NewStoragePoolEx returns a new StoragePoolEx
func NewStoragePoolEx(client *Client, pool *types.StoragePool) *StoragePool {
	return &StoragePool{
		StoragePool: pool,
		client:      client,
	}
}

// CreateStoragePool creates a storage pool
func (pd *ProtectionDomain) CreateStoragePool(sp *types.StoragePoolParam) (string, error) {
	path := fmt.Sprintf("/api/types/StoragePool/instances")
	sp.ProtectionDomainID = pd.ProtectionDomain.ID
	spResponse := types.StoragePoolResp{}
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, sp, &spResponse)
	if err != nil {
		return "", err
	}
	return spResponse.ID, nil
}

// ModifyStoragePoolName Modifies Storagepool Name
func (pd *ProtectionDomain) ModifyStoragePoolName(ID, name string) (string, error) {
	storagePoolParam := &types.ModifyStoragePoolName{
		Name: name,
	}

	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setStoragePoolName", ID)

	spResp := types.StoragePoolResp{}
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, storagePoolParam, &spResp)
	if err != nil {
		return "", err
	}

	return spResp.ID, nil
}

// ModifyStoragePoolMedia Modifies Storagepool Media Type
func (pd *ProtectionDomain) ModifyStoragePoolMedia(ID, mediaType string) (string, error) {
	storagePool := &types.StoragePoolMediaType{
		MediaType: mediaType,
	}

	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setMediaType", ID)

	spResp := types.StoragePoolResp{}
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, storagePool, &spResp)
	if err != nil {
		return "", err
	}

	return spResp.ID, nil
}

// ModifyRMCache Sets Read RAM Cache
func (sp *StoragePool) ModifyRMCache(useRmcache string) error {
	link, err := GetLink(sp.StoragePool.Links, "self")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%v/action/setUseRmcache", link.HREF)
	payload := &types.StoragePoolUseRmCache{
		UseRmcache: useRmcache,
	}
	err = sp.client.getJSONWithRetry(
		http.MethodPost, path, payload, nil)
	return err
}

// EnableRFCache Enables RFCache
func (pd *ProtectionDomain) EnableRFCache(ID string) (string, error) {
	storagePoolParam := &types.StoragePoolUseRfCache{}

	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/enableRfcache", ID)

	spResp := types.StoragePoolResp{}
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, storagePoolParam, &spResp)
	if err != nil {
		return "", err
	}

	return spResp.ID, nil
}

// EnableOrDisableZeroPadding Enables / disables zero padding
func (pd *ProtectionDomain) EnableOrDisableZeroPadding(ID string, zeroPadValue string) error {
	zeroPaddedParam := &types.StoragePoolZeroPadEnabled{
		ZeroPadEnabled: zeroPadValue,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setZeroPaddingPolicy", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, zeroPaddedParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetReplicationJournalCapacity Sets replication journal capacity
func (pd *ProtectionDomain) SetReplicationJournalCapacity(ID string, replicationJournalCapacity string) error {
	replicationJournalCapacityParam := &types.ReplicationJournalCapacityParam{
		ReplicationJournalCapacityMaxRatio: replicationJournalCapacity,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setReplicationJournalCapacity", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, replicationJournalCapacityParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetCapacityAlertThreshold Sets high or critical capacity alert threshold
func (pd *ProtectionDomain) SetCapacityAlertThreshold(ID string, capacityAlertThreshold *types.CapacityAlertThresholdParam) error {
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setCapacityAlertThresholds", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, capacityAlertThreshold, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetProtectedMaintenanceModeIoPriorityPolicy sets protected maintenance mode IO priority policy
func (pd *ProtectionDomain) SetProtectedMaintenanceModeIoPriorityPolicy(ID string, protectedMaintenanceModeParam *types.ProtectedMaintenanceModeParam) error {
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setProtectedMaintenanceModeIoPriorityPolicy", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, protectedMaintenanceModeParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetRebalanceEnabled sets rebalance enabled.
func (pd *ProtectionDomain) SetRebalanceEnabled(ID string, rebalanceEnabledValue string) error {
	rebalanceEnabledParam := &types.RebalanceEnabledParam{
		RebalanceEnabled: rebalanceEnabledValue,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setRebalanceEnabled", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, rebalanceEnabledParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetRebalanceIoPriorityPolicy Sets rebalance I/O priority policy
func (pd *ProtectionDomain) SetRebalanceIoPriorityPolicy(ID string, protectedMaintenanceModeParam *types.ProtectedMaintenanceModeParam) error {
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setRebalanceIoPriorityPolicy", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, protectedMaintenanceModeParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetVTreeMigrationIOPriorityPolicy Sets V-Tree migration I/O priority policy
func (pd *ProtectionDomain) SetVTreeMigrationIOPriorityPolicy(ID string, protectedMaintenanceModeParam *types.ProtectedMaintenanceModeParam) error {
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setVTreeMigrationIoPriorityPolicy", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, protectedMaintenanceModeParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetSparePercentage Sets spare percentage
func (pd *ProtectionDomain) SetSparePercentage(ID string, sparePercentageValue string) error {
	percentageParam := &types.SparePercentageParam{
		SparePercentage: sparePercentageValue,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setSparePercentage", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, percentageParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetRMcacheWriteHandlingMode Sets RMcache write handling mode
func (pd *ProtectionDomain) SetRMcacheWriteHandlingMode(ID string, writeHandlingModeValue string) error {
	writeHandlingParam := &types.RmcacheWriteHandlingModeParam{
		RmcacheWriteHandlingMode: writeHandlingModeValue,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setRmcacheWriteHandlingMode", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, writeHandlingParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetRebuildEnabled Sets Rebuild Enabled
func (pd *ProtectionDomain) SetRebuildEnabled(ID string, rebuildEnabledValue string) error {
	rebuildEnabled := &types.RebuildEnabledParam{
		RebuildEnabled: rebuildEnabledValue,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setRebuildEnabled", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, rebuildEnabled, nil)
	if err != nil {
		return err
	}
	return nil
}

// SetRebuildRebalanceParallelismParam Sets rebuild/rebalance parallelism
func (pd *ProtectionDomain) SetRebuildRebalanceParallelismParam(ID string, limitValue string) error {
	rebuildRebalanceParam := &types.RebuildRebalanceParallelismParam{
		Limit: limitValue,
	}
	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/setRebuildRebalanceParallelism", ID)
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, rebuildRebalanceParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// Fragmentation enables or disables fragmentation
func (pd *ProtectionDomain) Fragmentation(ID string, value bool) error {
	payload := &types.FragmentationParam{}
	if value {

		path := fmt.Sprintf("/api/instances/StoragePool::%v/action/enableFragmentation", ID)
		err := pd.client.getJSONWithRetry(
			http.MethodPost, path, payload, nil)
		if err != nil {
			return err
		}
	} else {
		path := fmt.Sprintf("/api/instances/StoragePool::%v/action/disableFragmentation", ID)
		err := pd.client.getJSONWithRetry(
			http.MethodPost, path, payload, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// DisableRFCache Disables RFCache
func (pd *ProtectionDomain) DisableRFCache(ID string) (string, error) {
	payload := &types.StoragePoolUseRfCache{}

	path := fmt.Sprintf("/api/instances/StoragePool::%v/action/disableRfcache", ID)

	spResp := types.StoragePoolResp{}
	err := pd.client.getJSONWithRetry(

		http.MethodPost, path, payload, &spResp)
	if err != nil {
		return "", err
	}

	return spResp.ID, nil
}

// DeleteStoragePool will delete a storage pool
func (pd *ProtectionDomain) DeleteStoragePool(name string) error {
	// get the storage pool name
	pool, err := pd.FindStoragePool("", name, "")
	if err != nil {
		return err
	}

	link, err := GetLink(pool.Links, "self")
	if err != nil {
		return err
	}

	storagePoolParam := &types.EmptyPayload{}

	path := fmt.Sprintf("%v/action/removeStoragePool", link.HREF)

	err = pd.client.getJSONWithRetry(
		http.MethodPost, path, storagePoolParam, nil)
	if err != nil {
		return err
	}

	return nil
}

// GetStoragePool returns a storage pool
func (pd *ProtectionDomain) GetStoragePool(
	storagepoolhref string,
) ([]*types.StoragePool, error) {
	var (
		err error
		sp  = &types.StoragePool{}
		sps []*types.StoragePool
	)

	if storagepoolhref == "" {
		var link *types.Link
		link, err := GetLink(pd.ProtectionDomain.Links,
			"/api/ProtectionDomain/relationship/StoragePool")
		if err != nil {
			return nil, err
		}
		err = pd.client.getJSONWithRetry(
			http.MethodGet, link.HREF, nil, &sps)
	} else {
		err = pd.client.getJSONWithRetry(
			http.MethodGet, storagepoolhref, nil, sp)
	}
	if err != nil {
		return nil, err
	}

	if storagepoolhref != "" {
		sps = append(sps, sp)
	}
	return sps, nil
}

// FindStoragePool returns a storagepool based on id or name
func (pd *ProtectionDomain) FindStoragePool(
	id, name, href string,
) (*types.StoragePool, error) {
	sps, err := pd.GetStoragePool(href)
	if err != nil {
		return nil, fmt.Errorf("Error getting protection domains %s", err)
	}

	for _, sp := range sps {
		if sp.ID == id || sp.Name == name || href != "" {
			return sp, nil
		}
	}

	return nil, errors.New("Couldn't find storage pool")
}

// GetStatistics returns statistics
func (sp *StoragePool) GetStatistics() (*types.Statistics, error) {
	link, err := GetLink(sp.StoragePool.Links,
		"/api/StoragePool/relationship/Statistics")
	if err != nil {
		return nil, err
	}

	stats := types.Statistics{}
	err = sp.client.getJSONWithRetry(
		http.MethodGet, link.HREF, nil, &stats)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// GetSDSStoragePool return SDS instances associated with storage pool
func (sp *StoragePool) GetSDSStoragePool() ([]types.Sds, error) {
	link, err := GetLink(sp.StoragePool.Links,
		"/api/StoragePool/relationship/SpSds")
	if err != nil {
		return nil, err
	}

	sds := []types.Sds{}
	err = sp.client.getJSONWithRetry(
		http.MethodGet, link.HREF, nil, &sds)
	if err != nil {
		return nil, err
	}

	return sds, nil
}

// GetStoragePoolByID returns a Storagepool by ID
func (s *System) GetStoragePoolByID(id string) (*types.StoragePool, error) {
	defer TimeSpent("GetStoragePoolByID", time.Now())

	path := fmt.Sprintf("/api/instances/StoragePool::%s", id)

	var storagepool *types.StoragePool
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &storagepool)
	if err != nil {
		return nil, err
	}

	return storagepool, err
}

// GetAllStoragePools returns all Storage pools on the system
func (s *System) GetAllStoragePools() ([]types.StoragePool, error) {
	defer TimeSpent("GetStoragepool", time.Now())
	path := "/api/types/StoragePool/instances"

	var storagepools []types.StoragePool
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &storagepools)
	if err != nil {
		return nil, err
	}

	return storagepools, nil
}
