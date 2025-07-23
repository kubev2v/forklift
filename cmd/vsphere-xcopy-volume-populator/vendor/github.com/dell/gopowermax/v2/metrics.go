/*
 Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"encoding/json"
	"net/http"
	"time"

	types "github.com/dell/gopowermax/v2/types/v100"
)

// The following constants are for the query of performance metrics for pmax
const (
	Average      = "Average"
	Performance  = "performance"
	StorageGroup = "/StorageGroup"
	Volume       = "/Volume"
	FileSystem   = "/file/filesystem"
	Metrics      = "/metrics"
	Keys         = "/keys"
	Array        = "/Array"
)

// GetStorageGroupPerfKeys returns the available timestamp for the storage group performance
func (c *Client) GetStorageGroupPerfKeys(ctx context.Context, symID string) (*types.StorageGroupKeysResult, error) {
	defer c.TimeSpent("GetStorageGroupPerfKeys", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := RESTPrefix + Performance + StorageGroup + Keys
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	params := types.StorageGroupKeysParam{
		SymmetrixID: symID,
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodPost, URL, c.getDefaultHeaders(), params)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	storageGroupInfo := &types.StorageGroupKeysResult{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(storageGroupInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return storageGroupInfo, nil
}

// GetArrayPerfKeys returns the available timestamp for the array performance
func (c *Client) GetArrayPerfKeys(ctx context.Context) (*types.ArrayKeysResult, error) {
	defer c.TimeSpent("GetArrayPerfKeys", time.Now())
	URL := RESTPrefix + Performance + Array + Keys
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodGet, URL, c.getDefaultHeaders(), nil)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	ArrayInfo := &types.ArrayKeysResult{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(ArrayInfo); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return ArrayInfo, nil
}

// GetStorageGroupMetrics returns a list of Storage Group performance metrics
func (c *Client) GetStorageGroupMetrics(ctx context.Context, symID string, storageGroupID string, metricsQuery []string, firstAvailableTime, lastAvailableTime int64) (*types.StorageGroupMetricsIterator, error) {
	defer c.TimeSpent("GetStorageGroupMetrics", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := RESTPrefix + Performance + StorageGroup + Metrics
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	params := types.StorageGroupMetricsParam{
		SymmetrixID:    symID,
		StartDate:      firstAvailableTime,
		EndDate:        lastAvailableTime,
		DataFormat:     Average,
		StorageGroupID: storageGroupID,
		Metrics:        metricsQuery,
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodPost, URL, c.getDefaultHeaders(), params)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	metricsList := &types.StorageGroupMetricsIterator{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(metricsList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return metricsList, nil
}

// GetVolumesMetrics returns a list of Volume performance metrics
func (c *Client) GetVolumesMetrics(ctx context.Context, symID string, storageGroups string, metricsQuery []string, firstAvailableTime, lastAvailableTime int64) (*types.VolumeMetricsIterator, error) {
	defer c.TimeSpent("GetVolumesMetrics", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := RESTPrefix + Performance + Volume + Metrics
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	params := types.VolumeMetricsParam{
		SystemID:                       symID,
		StartDate:                      firstAvailableTime,
		EndDate:                        lastAvailableTime,
		DataFormat:                     Average,
		CommaSeparatedStorageGroupList: storageGroups,
		Metrics:                        metricsQuery,
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodPost, URL, c.getDefaultHeaders(), params)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	metricsList := &types.VolumeMetricsIterator{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(metricsList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return metricsList, nil
}

// GetVolumesMetricsByID returns a given Volume performance metrics
func (c *Client) GetVolumesMetricsByID(ctx context.Context, symID string, volID string, metricsQuery []string, firstAvailableTime, lastAvailableTime int64) (*types.VolumeMetricsIterator, error) {
	defer c.TimeSpent("GetVolumesMetricsByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := RESTPrefix + Performance + Volume + Metrics
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	params := types.VolumeMetricsParam{
		SystemID:         symID,
		StartDate:        firstAvailableTime,
		EndDate:          lastAvailableTime,
		VolumeStartRange: volID,
		VolumeEndRange:   volID,
		DataFormat:       Average,
		Metrics:          metricsQuery,
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodPost, URL, c.getDefaultHeaders(), params)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	metricsList := &types.VolumeMetricsIterator{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(metricsList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return metricsList, nil
}

// GetFileSystemMetricsByID returns a given FileSystem performance metrics
func (c *Client) GetFileSystemMetricsByID(ctx context.Context, symID string, fsID string, metricsQuery []string, firstAvailableTime, lastAvailableTime int64) (*types.FileSystemMetricsIterator, error) {
	defer c.TimeSpent("GetFileSystemMetricsByID", time.Now())
	if _, err := c.IsAllowedArray(symID); err != nil {
		return nil, err
	}
	URL := RESTPrefix + Performance + FileSystem + Metrics
	ctx, cancel := c.GetTimeoutContext(ctx)
	defer cancel()
	params := types.FileSystemMetricsParam{
		SystemID:     symID,
		StartDate:    firstAvailableTime,
		EndDate:      lastAvailableTime,
		DataFormat:   Average,
		FileSystemID: fsID,
		Metrics:      metricsQuery,
	}
	resp, err := c.api.DoAndGetResponseBody(ctx, http.MethodPost, URL, c.getDefaultHeaders(), params)
	if err = c.checkResponse(resp); err != nil {
		return nil, err
	}
	metricsList := &types.FileSystemMetricsIterator{}
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(metricsList); err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return metricsList, nil
}
