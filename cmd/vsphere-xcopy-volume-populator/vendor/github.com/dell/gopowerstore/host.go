/*
 *
 * Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	hostURL        = "host"
	hostMappingURL = "host_volume_mapping"
)

func getHostDefaultQueryParams(c Client) api.QueryParamsEncoder {
	host := Host{}
	return c.APIClient().QueryParamsWithFields(&host)
}

func getHostVolumeMappingQueryParams(c Client) api.QueryParamsEncoder {
	hostMapping := HostVolumeMapping{}
	return c.APIClient().QueryParamsWithFields(&hostMapping)
}

// GetHosts returns hosts list
func (c *ClientIMPL) GetHosts(ctx context.Context) (resp []Host, err error) {
	err = c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []Host
		qp := getHostDefaultQueryParams(c)
		qp.Limit(paginationDefaultPageSize)
		qp.Offset(offset)
		qp.Order("name")
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    hostURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			resp = append(resp, page...)
		}
		return meta, err
	})
	return resp, err
}

// GetHost get host by id
func (c *ClientIMPL) GetHost(ctx context.Context, id string) (resp Host, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    hostURL,
			ID:          id,
			QueryParams: getHostDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// GetHostByName get host by name
func (c *ClientIMPL) GetHostByName(ctx context.Context, name string) (resp Host, err error) {
	var hostList []Host
	qp := getHostDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    hostURL,
			QueryParams: qp,
		},
		&hostList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(hostList) != 1 {
		return resp, NewHostIsNotExistError()
	}
	return hostList[0], err
}

// CreateHost register new host
func (c *ClientIMPL) CreateHost(ctx context.Context, createParams *HostCreate) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: hostURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteHost removes host registration
func (c *ClientIMPL) DeleteHost(ctx context.Context,
	deleteParams *HostDelete, id string,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: hostURL,
			ID:       id,
			Body:     deleteParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// ModifyHost update host info
func (c *ClientIMPL) ModifyHost(ctx context.Context,
	modifyParams *HostModify, id string,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: hostURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetHostVolumeMappings returns volume mapping
func (c *ClientIMPL) GetHostVolumeMappings(ctx context.Context) (resp []HostVolumeMapping, err error) {
	err = c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []HostVolumeMapping
		qp := getHostVolumeMappingQueryParams(c)
		qp.Limit(paginationDefaultPageSize)
		qp.Offset(offset)
		qp.Order("id")
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    hostMappingURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			resp = append(resp, page...)
		}
		return meta, err
	})

	return resp, WrapErr(err)
}

// GetHostVolumeMapping returns volume mapping by id
func (c *ClientIMPL) GetHostVolumeMapping(ctx context.Context, id string) (resp HostVolumeMapping, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    hostMappingURL,
			ID:          id,
			QueryParams: getHostVolumeMappingQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// GetHostVolumeMappingByVolumeID returns volume mapping by volumeID
func (c *ClientIMPL) GetHostVolumeMappingByVolumeID(
	ctx context.Context, volumeID string,
) (resp []HostVolumeMapping, err error) {
	err = c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []HostVolumeMapping
		qp := getHostVolumeMappingQueryParams(c)
		qp.RawArg("volume_id", fmt.Sprintf("eq.%s", volumeID))
		qp.Order("id")
		qp.Limit(paginationDefaultPageSize)
		qp.Offset(offset)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    hostMappingURL,
				QueryParams: qp,
			},
			&page)
		err = WrapErr(err)
		if err == nil {
			resp = append(resp, page...)
		}
		return meta, err
	})
	return resp, WrapErr(err)
}

// AttachVolumeToHost attaches volume to host
func (c *ClientIMPL) AttachVolumeToHost(
	ctx context.Context,
	hostID string,
	attachParams *HostVolumeAttach,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: hostURL,
			ID:       hostID,
			Action:   "attach",
			Body:     attachParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DetachVolumeFromHost detaches volume to host
func (c *ClientIMPL) DetachVolumeFromHost(
	ctx context.Context,
	hostID string,
	detachParams *HostVolumeDetach,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: hostURL,
			ID:       hostID,
			Action:   "detach",
			Body:     detachParams,
		},
		&resp)
	return resp, WrapErr(err)
}
