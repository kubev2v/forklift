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
	"fmt"

	"github.com/dell/gopowerstore/api"
)

const (
	snapshotRuleURL = "snapshot_rule"
)

func getSnapshotRuleDefaultQueryParams(c Client) api.QueryParamsEncoder {
	snapshotRule := SnapshotRule{}
	return c.APIClient().QueryParamsWithFields(&snapshotRule)
}

// GetSnapshotRule query and return specific snapshot rule by id
func (c *ClientIMPL) GetSnapshotRule(ctx context.Context, id string) (resp SnapshotRule, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    snapshotRuleURL,
			ID:          id,
			QueryParams: getSnapshotRuleDefaultQueryParams(c),
		},
		&resp)
	return resp, WrapErr(err)
}

func (c *ClientIMPL) GetSnapshotRuleByName(ctx context.Context, name string) (resp SnapshotRule, err error) {
	var ruleList []SnapshotRule
	rule := SnapshotRule{}
	qp := c.APIClient().QueryParamsWithFields(&rule)
	qp.RawArg("name", fmt.Sprintf("eq.%s", name))
	qp.Select("policies")
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    snapshotRuleURL,
			QueryParams: qp,
		},
		&ruleList)

	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(ruleList) != 1 {
		return resp, snapshotRuleNotExists()
	}
	return ruleList[0], nil
}

// GetSnapshotRules returns a list of snapshot rules
func (c *ClientIMPL) GetSnapshotRules(ctx context.Context) ([]SnapshotRule, error) {
	var result []SnapshotRule
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []SnapshotRule
		qp := getSnapshotRuleDefaultQueryParams(c)
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    snapshotRuleURL,
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

// CreateSnapshotRule creates new snapshot rule
func (c *ClientIMPL) CreateSnapshotRule(ctx context.Context,
	createParams *SnapshotRuleCreate,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: snapshotRuleURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// ModifySnapshotRule updates existing snapshot rule
// If the snapshot rule is associated with a policy that is currently applied to a storage resource,
// the modified rule is immediately applied to the associated storage resource.
func (c *ClientIMPL) ModifySnapshotRule(ctx context.Context, modifyParams *SnapshotRuleCreate, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: snapshotRuleURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteSnapshotRule deletes existing snapshot rule
func (c *ClientIMPL) DeleteSnapshotRule(ctx context.Context,
	deleteParams *SnapshotRuleDelete, id string,
) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: snapshotRuleURL,
			ID:       id,
			Body:     deleteParams,
		},
		&resp)
	return resp, WrapErr(err)
}
