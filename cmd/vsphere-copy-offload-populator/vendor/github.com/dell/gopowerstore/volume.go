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
	"fmt"

	"github.com/dell/gopowerstore/api"
)

const (
	volumeURL    = "volume"
	applianceURL = "appliance"
)

func getVolumeDefaultQueryParams(c Client) api.QueryParamsEncoder {
	vol := Volume{}
	return c.APIClient().QueryParamsWithFields(&vol)
}

func getApplianceDefaultQueryParams(c Client) api.QueryParamsEncoder {
	app := ApplianceInstance{}
	return c.APIClient().QueryParamsWithFields(&app)
}

// GetVolume query and return specific volume by id
func (c *ClientIMPL) GetVolume(ctx context.Context, id string) (resp Volume, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    volumeURL,
			ID:          id,
			QueryParams: getVolumeDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// GetVolumeByName query and return specific volume by name
func (c *ClientIMPL) GetVolumeByName(ctx context.Context, name string) (resp Volume, err error) {
	var volList []Volume
	qp := getVolumeDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    volumeURL,
			QueryParams: qp,
		},
		&volList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(volList) != 1 {
		return resp, NewNotFoundError()
	}
	return volList[0], err
}

// GetVolumes returns a list of volumes
func (c *ClientIMPL) GetVolumes(ctx context.Context) ([]Volume, error) {
	var result []Volume
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []Volume
		qp := getVolumeDefaultQueryParams(c)
		qp.RawArg("type", fmt.Sprintf("not.eq.%s", VolumeTypeEnumSnapshot))
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    volumeURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

// GetSnapshot query and return specific snapshot by it's id
func (c *ClientIMPL) GetSnapshot(ctx context.Context, snapID string) (resVol Volume, err error) {
	qp := getVolumeDefaultQueryParams(c)
	qp.RawArg("type", fmt.Sprintf("eq.%s", VolumeTypeEnumSnapshot))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    volumeURL,
			ID:          snapID,
			QueryParams: qp,
		},
		&resVol)
	return resVol, WrapErr(err)
}

// GetSnapshotByName query and return specific snapshot by name
func (c *ClientIMPL) GetSnapshotByName(ctx context.Context, snapName string) (resVol Volume, err error) {
	var volList []Volume
	qp := getVolumeDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", snapName))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    volumeURL,
			QueryParams: qp,
		},
		&volList)
	err = WrapErr(err)
	if err != nil {
		return resVol, err
	}
	if len(volList) != 1 {
		return resVol, NewNotFoundError()
	}
	return volList[0], err
}

// GetSnapshots returns all snapshots
func (c *ClientIMPL) GetSnapshots(ctx context.Context) ([]Volume, error) {
	var result []Volume
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []Volume
		qp := getVolumeDefaultQueryParams(c)
		qp.RawArg("type", fmt.Sprintf("eq.%s", VolumeTypeEnumSnapshot))
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    volumeURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

// GetSnapshotsByVolumeID returns a list of snapshots for specific volume
func (c *ClientIMPL) GetSnapshotsByVolumeID(ctx context.Context, volID string) ([]Volume, error) {
	var result []Volume
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []Volume
		qp := getVolumeDefaultQueryParams(c)
		qp.RawArg("protection_data->>source_id", fmt.Sprintf("eq.%s", volID))
		qp.RawArg("type", fmt.Sprintf("eq.%s", VolumeTypeEnumSnapshot))
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    volumeURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			result = append(result, page...)
		}
		return meta, err
	})
	return result, err
}

// CreateVolume creates new volume
func (c *ClientIMPL) CreateVolume(ctx context.Context,
	createParams *VolumeCreate,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// ModifyVolume changes some volumes properties. Used for volume expansion
func (c *ClientIMPL) ModifyVolume(ctx context.Context,
	modifyParams *VolumeModify, volID string,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: volumeURL,
			ID:       volID,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// ComputeDifferences a) finds allocated nonzero blocks or b) computes differences between
// two snapshots from the same volume
func (c *ClientIMPL) ComputeDifferences(ctx context.Context,
	computeDiffParams *VolumeComputeDifferences, volID string,
) (resp VolumeComputeDifferencesResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			ID:       volID,
			Action:   VolumeActionComputeDifferences,
			Body:     computeDiffParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateVolumeFromSnapshot creates a new volume by cloning a snapshot
func (c *ClientIMPL) CreateVolumeFromSnapshot(ctx context.Context,
	createParams *VolumeClone, snapID string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			ID:       snapID,
			Action:   VolumeActionClone,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateSnapshot creates a new snapshot
func (c *ClientIMPL) CreateSnapshot(ctx context.Context,
	createSnapParams *SnapshotCreate, id string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			ID:       id,
			Action:   VolumeActionSnapshot,
			Body:     createSnapParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteVolume deletes existing volume
func (c *ClientIMPL) DeleteVolume(ctx context.Context,
	deleteParams *VolumeDelete, id string,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: volumeURL,
			ID:       id,
			Body:     deleteParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteSnapshot is an alias for delete volume, because snapshots are essentially -- volumes
func (c *ClientIMPL) DeleteSnapshot(ctx context.Context,
	deleteParams *VolumeDelete, id string,
) (resp EmptyResponse, err error) {
	return c.DeleteVolume(ctx, deleteParams, id)
}

// CloneVolume creates a new volume by cloning a snapshot
func (c *ClientIMPL) CloneVolume(ctx context.Context,
	createParams *VolumeClone, volID string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			ID:       volID,
			Action:   VolumeActionClone,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetAppliance query and return specific appliance by ID
func (c *ClientIMPL) GetAppliance(ctx context.Context, id string) (resp ApplianceInstance, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    applianceURL,
			ID:          id,
			QueryParams: getApplianceDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// GetApplianceByName query and return specific appliance by name
func (c *ClientIMPL) GetApplianceByName(ctx context.Context, name string) (resp ApplianceInstance, err error) {
	var appList []ApplianceInstance
	qp := getApplianceDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    applianceURL,
			QueryParams: qp,
		},
		&appList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(appList) != 1 {
		return resp, NewNotFoundError()
	}
	return appList[0], err
}

// ConfigureMetroVolume configures the given volume, id, for metro replication with
// the remote PowerStore system and optional remote PowerStore appliance provided in config.
// Returns the metro replication session ID and any errors.
func (c *ClientIMPL) ConfigureMetroVolume(ctx context.Context, id string, config *MetroConfig) (resp MetroSessionResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			Action:   VolumeActionConfigureMetro,
			ID:       id,
			Body:     config,
		},
		&resp)

	return resp, WrapErr(err)
}

// EndMetroVolume ends the metro session for a volume, id, between two PowerStore systems.
// deleteOpts provides options to delete the replicated volume on the remote system and
// whether or not to force the session removal.
func (c *ClientIMPL) EndMetroVolume(ctx context.Context, id string, deleteOpts *EndMetroVolumeOptions) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: volumeURL,
			Action:   VolumeActionEndMetro,
			ID:       id,
			Body:     deleteOpts,
		},
		&resp)

	return resp, WrapErr(err)
}
