/*
 *
 * Copyright © 2020-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

	"github.com/dell/gopowerstore/api"

	log "github.com/sirupsen/logrus"
)

const apiFCPortURL = "fc_port"

func getFCPortDefaultQueryParams(c Client) api.QueryParamsEncoder {
	fcPort := FcPort{}
	return c.APIClient().QueryParamsWithFields(&fcPort)
}

// GetFCPorts returns a list of fc ports for array
func (c *ClientIMPL) GetFCPorts(
	ctx context.Context,
) (resp []FcPort, err error) {
	err = c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []FcPort
		qp := getFCPortDefaultQueryParams(c)

		majorMinorVersion, err := c.GetSoftwareMajorMinorVersion(ctx)
		if err != nil {
			log.Errorf("Couldn't find the array version %s", err.Error())
		} else {
			if majorMinorVersion >= 3.0 {
				qp.Select("wwn_nvme,wwn_node")
			}
		}
		qp.Limit(paginationDefaultPageSize)
		qp.Offset(offset)
		qp.Order("id")
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    apiFCPortURL,
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

// GetFCPort get FC port by id
func (c *ClientIMPL) GetFCPort(ctx context.Context, id string) (resp FcPort, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    apiFCPortURL,
			ID:          id,
			QueryParams: getFCPortDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}
