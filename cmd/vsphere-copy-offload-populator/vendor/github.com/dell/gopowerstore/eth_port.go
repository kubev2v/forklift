/*
 *
 * Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

	log "github.com/sirupsen/logrus"
)

const apiEthPortURL = "eth_port"

func getEthPortDefaultQueryParams(c Client) api.QueryParamsEncoder {
	ethPort := EthPort{}
	return c.APIClient().QueryParamsWithFields(&ethPort)
}

// GetEthPorts returns a list of Ethernet ports for the array
func (c *ClientIMPL) GetEthPorts(ctx context.Context) (resp []EthPort, err error) {
	err = c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []EthPort
		qp := getEthPortDefaultQueryParams(c)

		majorMinorVersion, err := c.GetSoftwareMajorMinorVersion(ctx)
		if err != nil {
			log.Errorf("Couldn't find the array version %s", err.Error())
		} else {
			// Add version-specific fields
			if majorMinorVersion >= 3.0 {
				qp.Select("is_in_use,permanent_mac_address")
			}
			if majorMinorVersion >= 3.5 {
				qp.Select("fsn_id")
			}
			if majorMinorVersion >= 4.1 {
				qp.Select("l2_discovery_details")
			}
		}
		qp.Limit(paginationDefaultPageSize)
		qp.Offset(offset)
		qp.Order("id")
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    apiEthPortURL,
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

// GetEthPort gets an Ethernet port by ID
func (c *ClientIMPL) GetEthPort(ctx context.Context, id string) (resp EthPort, err error) {
	qp := getEthPortDefaultQueryParams(c)

	majorMinorVersion, err := c.GetSoftwareMajorMinorVersion(ctx)
	if err != nil {
		log.Errorf("Couldn't find the array version %s", err.Error())
	} else {
		if majorMinorVersion >= 3.0 {
			qp.Select("is_in_use,permanent_mac_address")
		}
		if majorMinorVersion >= 3.5 {
			qp.Select("fsn_id")
		}
		if majorMinorVersion >= 4.1 {
			qp.Select("l2_discovery_details")
		}
	}

	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    apiEthPortURL,
			ID:          id,
			QueryParams: qp,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetEthPortByName gets an Ethernet port by name
func (c *ClientIMPL) GetEthPortByName(ctx context.Context, name string) (resp EthPort, err error) {
	var portList []EthPort
	qp := getEthPortDefaultQueryParams(c)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))

	majorMinorVersion, err := c.GetSoftwareMajorMinorVersion(ctx)
	if err != nil {
		log.Errorf("Couldn't find the array version %s", err.Error())
	} else {
		if majorMinorVersion >= 3.0 {
			qp.Select("is_in_use,permanent_mac_address")
		}
		if majorMinorVersion >= 3.5 {
			qp.Select("fsn_id")
		}
		if majorMinorVersion >= 4.1 {
			qp.Select("l2_discovery_details")
		}
	}

	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    apiEthPortURL,
			QueryParams: qp,
		},
		&portList)

	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(portList) != 1 {
		return resp, NewNotFoundError()
	}
	return portList[0], nil
}

// ModifyEthPort modifies an Ethernet port's requested speed
func (c *ClientIMPL) ModifyEthPort(ctx context.Context, modifyParams *EthPortModify, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: apiEthPortURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}
