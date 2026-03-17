/*
 *
 * Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"strings"

	"github.com/dell/gopowerstore/api"
)

const (
	limitURL = "limit"
)

func (c *ClientIMPL) callGetLimit(ctx context.Context) (resp []Limit, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    limitURL,
			QueryParams: getLimitDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

// GetMaxVolumeSize - Returns the max size of a volume supported by the array
func (c *ClientIMPL) GetMaxVolumeSize(ctx context.Context) (int64, error) {
	resp, err := c.callGetLimit(ctx)

	limit := int64(-1)
	for _, entry := range resp {
		if strings.EqualFold(entry.ID, string(MaxVolumeSize)) {
			limit = entry.Limit
			break
		}
	}

	return limit, err
}

func getLimitDefaultQueryParams(c Client) api.QueryParamsEncoder {
	limit := Limit{}
	return c.APIClient().QueryParamsWithFields(&limit)
}
