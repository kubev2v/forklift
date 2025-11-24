/*
 *
 * Copyright © 2022-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	hostGroupURL = "host_group"
)

func getHostGroupDefaultQueryParams(c Client) api.QueryParamsEncoder {
	host := HostGroup{}
	return c.APIClient().QueryParamsWithFields(&host)
}

// AttachVolumeToHost attaches volume to hostGroup
func (c *ClientIMPL) AttachVolumeToHostGroup(
	ctx context.Context,
	hostGroupID string,
	attachParams *HostVolumeAttach,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: hostGroupURL,
			ID:       hostGroupID,
			Action:   "attach",
			Body:     attachParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DetachVolumeFromHost detaches volume to hostGroup
func (c *ClientIMPL) DetachVolumeFromHostGroup(
	ctx context.Context,
	hostGroupID string,
	detachParams *HostVolumeDetach,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: hostGroupURL,
			ID:       hostGroupID,
			Action:   "detach",
			Body:     detachParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetHostGroupByName get host by name
func (c *ClientIMPL) GetHostGroupByName(ctx context.Context, name string) (resp HostGroup, err error) {
	var hostList []HostGroup
	qp := getHostGroupDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    hostGroupURL,
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

// GetHostGroup query and return specific host group id
func (c *ClientIMPL) GetHostGroup(ctx context.Context, id string) (resp HostGroup, err error) {
	hostGroup := HostGroup{}
	qc := c.APIClient().QueryParamsWithFields(&hostGroup)
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    hostGroupURL,
			ID:          id,
			QueryParams: qc,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetHostGroups returns a list of host groups
func (c *ClientIMPL) GetHostGroups(ctx context.Context) ([]HostGroup, error) {
	var result []HostGroup
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []HostGroup
		hostGroup := HostGroup{}
		qp := c.APIClient().QueryParamsWithFields(&hostGroup)
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    hostGroupURL,
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

// CreateHostGroup creates new host group
func (c *ClientIMPL) CreateHostGroup(ctx context.Context,
	createParams *HostGroupCreate,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: hostGroupURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteHostGroup deletes existing Host Group
func (c *ClientIMPL) DeleteHostGroup(ctx context.Context, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: hostGroupURL,
			ID:       id,
		},
		&resp)
	return resp, WrapErr(err)
}

func (c *ClientIMPL) ModifyHostGroup(ctx context.Context,
	modifyParams *HostGroupModify, id string,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: hostGroupURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}
