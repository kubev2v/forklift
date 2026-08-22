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
	eventEndpoint string = "event"
)

func getEventsDefaultQueryParams(c Client) api.QueryParamsEncoder {
	event := Event{}
	return c.APIClient().QueryParamsWithFields(&event)
}

func (c *ClientIMPL) GetEvents(ctx context.Context, opts GetEventsOpts) (*GetEventsResponse, error) {
	var events []Event
	qp := getEventsDefaultQueryParams(c)
	qp.Order("generated_timestamp.desc")

	opts.RequestPagination.SetQueryParams(qp)

	for k, v := range opts.Queries {
		qp.RawArg(k, v)
	}

	meta, err := c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    eventEndpoint,
			QueryParams: qp,
		},
		&events)
	err = WrapErr(err)
	if err != nil {
		return nil, err
	}

	response := &GetEventsResponse{
		EventsResponseMeta{meta},
		events,
	}

	return response, nil
}
