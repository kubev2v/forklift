/*
 *
 * Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
)

const (
	alertEndpoint = "alert"
)

func getAlertsDefaultQueryParams(c Client) api.QueryParamsEncoder {
	alert := Alert{}
	return c.APIClient().QueryParamsWithFields(&alert)
}

func (c *ClientIMPL) GetAlerts(ctx context.Context, opts GetAlertsOpts) (*GetAlertsResponse, error) {
	var result []Alert
	qp := getAlertsDefaultQueryParams(c)

	// pagination info
	opts.RequestPagination.SetQueryParams(qp)

	// Add any additional queries to the parameters
	for k, v := range opts.Queries {
		qp.RawArg(k, v)
	}

	meta, err := c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    alertEndpoint,
			QueryParams: qp,
		},
		&result)
	err = WrapErr(err)
	if err != nil {
		return &GetAlertsResponse{}, err
	}

	resp := &GetAlertsResponse{
		AlertsResponseMeta{meta},
		result,
	}
	return resp, nil
}
