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

package api

import (
	"net/url"
	"strconv"
	"strings"
)

// QueryParamsEncoder interface provide ability to manipulate query string parameters
type QueryParamsEncoder interface {
	RawArg(string, string) QueryParamsEncoder
	Select(...string) QueryParamsEncoder
	Order(...string) QueryParamsEncoder
	Limit(int) QueryParamsEncoder
	Offset(int) QueryParamsEncoder
	Async(bool) QueryParamsEncoder
	Encode() string
}

// QueryParams struct holds additional query options for PowerStore API
type QueryParams struct {
	// rawArgs  GET only
	// Add raw args to query.
	// This also allows to filter rows in a query, by constraining the result to rows matching the
	// property condition(s) specified
	// [not.]<operator>.<filter value>
	// Example value: m["age"] = "ge.13"
	rawArgs map[string]string
	// selectParam GET only
	// This filters columns by selecting which properties to return from the query.
	// <property>,...
	// Example: id,name
	selectParam *[]string
	// orderParam GET only
	// Sorts the result set by the properties specified
	// 	<property>[.asc|.desc]
	// Example: last_name,first_name
	orderParam *[]string
	// offsetParam GET only
	// Starting row of the result set, used with limit for paging
	// <int>
	// Example: 30
	offsetParam *int
	// limitParam GET only
	// Optional page size desired for the response
	// <int>
	// Example: 15
	limitParam *int
	// asyncParam non-GET only
	// Control sync vs. async response for non-query requests.
	// Normally requests return the response body (which may be empty) or an error message body.
	// <boolean>
	// Example: true
	asyncParam *bool
}

func (qp *QueryParams) addTo(attr **[]string, fields []string) {
	if *attr == nil {
		attrArr := make([]string, 0)
		*attr = &attrArr
	}
	**attr = append(**attr, fields...)
}

// RawArg allows to set query params in key/value form
func (qp *QueryParams) RawArg(key string, value string) QueryParamsEncoder {
	if qp.rawArgs == nil {
		qp.rawArgs = make(map[string]string)
	}
	qp.rawArgs[key] = value
	return qp
}

// Select adds values to QueryParams.selectParam array
func (qp *QueryParams) Select(fields ...string) QueryParamsEncoder {
	qp.addTo(&qp.selectParam, fields)
	return qp
}

// Order adds values to QueryParams.orderParam array
func (qp *QueryParams) Order(fields ...string) QueryParamsEncoder {
	qp.addTo(&qp.orderParam, fields)
	return qp
}

// Limit set value of QueryParams.limitParam
func (qp *QueryParams) Limit(value int) QueryParamsEncoder {
	qp.limitParam = &value
	return qp
}

// Offset set value of QueryParams.offsetParam
func (qp *QueryParams) Offset(value int) QueryParamsEncoder {
	qp.offsetParam = &value
	return qp
}

// Async set value of QueryParams.asyncParam
func (qp *QueryParams) Async(value bool) QueryParamsEncoder {
	qp.asyncParam = &value
	return qp
}

// Encode encodes the values into “URL encoded” form
// ("bar=baz&foo=quux") sorted by key.
func (qp *QueryParams) Encode() string {
	q := make(url.Values)
	if qp.selectParam != nil {
		q.Set("select", strings.Join(*qp.selectParam, ","))
	}
	if qp.orderParam != nil {
		q.Set("order", strings.Join(*qp.orderParam, ","))
	}
	if qp.offsetParam != nil {
		q.Set("offset", strconv.Itoa(*qp.offsetParam))
	}
	if qp.limitParam != nil {
		q.Set("limit", strconv.Itoa(*qp.limitParam))
	}
	if qp.asyncParam != nil {
		q.Set("is_async", strconv.FormatBool(*qp.asyncParam))
	}

	for k, v := range qp.rawArgs {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q.Encode()
}
