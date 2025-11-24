/*
 *
 * Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"strconv"
	"strings"

	"github.com/dell/gopowerstore/api"

	log "github.com/sirupsen/logrus"
)

const apiSoftwareInstalledURL = "software_installed"

func getSoftwareInstalledDefaultQueryParams(c Client) api.QueryParamsEncoder {
	softwareInstalled := SoftwareInstalled{}
	return c.APIClient().QueryParamsWithFields(&softwareInstalled)
}

// GetSoftwareInstalled queries the software packages that are installed on each appliance, or on the cluster as a whole
func (c *ClientIMPL) GetSoftwareInstalled(
	ctx context.Context,
) (resp []SoftwareInstalled, err error) {
	err = c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []SoftwareInstalled
		qp := getSoftwareInstalledDefaultQueryParams(c)
		qp.Limit(paginationDefaultPageSize)
		qp.Offset(offset)
		qp.Order("id")
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    apiSoftwareInstalledURL,
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

func (c *ClientIMPL) GetSoftwareMajorMinorVersion(
	ctx context.Context,
) (majorMinorVersion float32, err error) {
	resp, err := c.GetSoftwareInstalled(ctx)
	if err != nil {
		log.Errorf("couldn't find the softwares installed on the Powerstore array %s", err.Error())
		return 0.0, err
	}

	for _, softwareInstalled := range resp {
		if softwareInstalled.IsCluster {
			softwareVersion := softwareInstalled.BuildVersion
			versions := strings.Split(softwareVersion, ".")

			if len(versions) > 2 {
				var majorVersion, minorVersion int

				if majorVersion, err = strconv.Atoi(versions[0]); err != nil {
					log.Errorf("couldn't get the software major version installed on the PowerStore array: %s", err.Error())
					return 0.0, err
				}

				if minorVersion, err = strconv.Atoi(versions[1]); err != nil {
					log.Errorf("couldn't get the software minor version installed on the PowerStore array: %s", err.Error())
					return 0.0, err
				}

				majorMinorVersion = float32(majorVersion) + float32(minorVersion)*0.1
			}
		}
	}
	return majorMinorVersion, nil
}
