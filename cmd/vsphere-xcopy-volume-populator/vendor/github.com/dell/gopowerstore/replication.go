/*
 *
 * Copyright © 2021-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *      http://www.apache.org/licenses/LICENSE-2.0
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

type ActionType string

const (
	RsActionFailover  ActionType = "failover"
	RsActionReprotect ActionType = "reprotect"
	RsActionResume    ActionType = "resume"
	RsActionPause     ActionType = "pause"
	RsActionSync      ActionType = "sync"
)

const (
	replicationRuleURL    = "replication_rule"
	policyURL             = "policy"
	replicationSessionURL = "replication_session"
)

// CreateReplicationRule creates new replication rule
func (c *ClientIMPL) CreateReplicationRule(ctx context.Context,
	createParams *ReplicationRuleCreate,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: replicationRuleURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

func (c *ClientIMPL) GetReplicationRuleByName(ctx context.Context,
	ruleName string,
) (resp ReplicationRule, err error) {
	var ruleList []ReplicationRule
	rule := ReplicationRule{}
	qp := c.APIClient().QueryParamsWithFields(&rule)
	qp.RawArg("name", fmt.Sprintf("eq.%s", ruleName))
	qp.Select("policies")
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    replicationRuleURL,
			QueryParams: qp,
		},
		&ruleList)

	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(ruleList) != 1 {
		return resp, replicationRuleNotExists()
	}
	return ruleList[0], nil
}

// GetReplicationRule query and return specific replication rule by id
func (c *ClientIMPL) GetReplicationRule(ctx context.Context, id string) (resp ReplicationRule, err error) {
	rule := ReplicationRule{}
	qp := c.APIClient().QueryParamsWithFields(&rule)

	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    replicationRuleURL,
			ID:          id,
			QueryParams: qp,
		},
		&resp)
	return resp, WrapErr(err)
}

// CreateProtectionPolicy creates new protection policy
func (c *ClientIMPL) CreateProtectionPolicy(ctx context.Context,
	createParams *ProtectionPolicyCreate,
) (resp CreateResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: policyURL,
			Body:     createParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// ModifyProtectionPolicy updates existing protection policy
func (c *ClientIMPL) ModifyProtectionPolicy(ctx context.Context, modifyParams *ProtectionPolicyCreate, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: policyURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}

func (c *ClientIMPL) GetProtectionPolicyByName(ctx context.Context,
	policyName string,
) (resp ProtectionPolicy, err error) {
	var policyList []ProtectionPolicy
	policy := ProtectionPolicy{}
	qp := c.APIClient().QueryParamsWithFields(&policy)
	qp.RawArg("name", fmt.Sprintf("eq.%s", policyName))
	qp.RawArg("type", fmt.Sprintf("eq.%s", "Protection"))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    policyURL,
			QueryParams: qp,
		},
		&policyList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(policyList) != 1 {
		return resp, protectionPolicyNotExists()
	}
	return policyList[0], nil
}

// GetProtectionPolicy query and return specific protection policy id
func (c *ClientIMPL) GetProtectionPolicy(ctx context.Context, id string) (resp ProtectionPolicy, err error) {
	protectionPolicy := ProtectionPolicy{}
	qc := c.APIClient().QueryParamsWithFields(&protectionPolicy)
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    policyURL,
			ID:          id,
			QueryParams: qc,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetProtectionPolicies returns a list of protection policies
func (c *ClientIMPL) GetProtectionPolicies(ctx context.Context) ([]ProtectionPolicy, error) {
	var result []ProtectionPolicy
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []ProtectionPolicy
		policy := ProtectionPolicy{}
		qp := c.APIClient().QueryParamsWithFields(&policy)
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    policyURL,
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

func (c *ClientIMPL) GetReplicationSessionByLocalResourceID(ctx context.Context, id string) (resp ReplicationSession, err error) {
	var sessionList []ReplicationSession
	ses := ReplicationSession{}
	qp := c.APIClient().QueryParamsWithFields(&ses)
	qp.RawArg("local_resource_id", fmt.Sprintf("eq.%s", id))
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    replicationSessionURL,
			QueryParams: qp,
		},
		&sessionList)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}
	if len(sessionList) != 1 {
		return resp, replicationGroupNotExists()
	}
	return sessionList[0], err
}

func (c *ClientIMPL) GetReplicationSessionByID(ctx context.Context, id string) (resp ReplicationSession, err error) {
	var session ReplicationSession
	ses := ReplicationSession{}
	qp := c.APIClient().QueryParamsWithFields(&ses)
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    replicationSessionURL,
			QueryParams: qp,
			ID:          id,
		},
		&session)
	err = WrapErr(err)
	if err != nil {
		return resp, err
	}

	return session, err
}

// DeleteReplicationRule deletes existing RR
func (c *ClientIMPL) DeleteReplicationRule(ctx context.Context, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: replicationRuleURL,
			ID:       id,
		},
		&resp)
	return resp, WrapErr(err)
}

// DeleteProtectionPolicy deletes existing PP
func (c *ClientIMPL) DeleteProtectionPolicy(ctx context.Context, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "DELETE",
			Endpoint: policyURL,
			ID:       id,
		},
		&resp)
	return resp, WrapErr(err)
}

func (c *ClientIMPL) ExecuteActionOnReplicationSession(ctx context.Context, id string, actionType ActionType, params *FailoverParams) (resp EmptyResponse, err error) {
	var res interface{}
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: replicationSessionURL,
			ID:       id,
			Action:   string(actionType),
			Body:     params,
		},
		&res)
	return resp, WrapErr(err)
}

// ModifyReplicationRule modifies replication rule
func (c *ClientIMPL) ModifyReplicationRule(ctx context.Context, modifyParams *ReplicationRuleModify, id string) (resp EmptyResponse, err error) {
	_, err = c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "PATCH",
			Endpoint: replicationRuleURL,
			ID:       id,
			Body:     modifyParams,
		},
		&resp)
	return resp, WrapErr(err)
}

// GetReplicationRules returns a list of replication rules
func (c *ClientIMPL) GetReplicationRules(ctx context.Context) ([]ReplicationRule, error) {
	var result []ReplicationRule
	err := c.readPaginatedData(func(offset int) (api.RespMeta, error) {
		var page []ReplicationRule
		policy := ReplicationRule{}
		qp := c.APIClient().QueryParamsWithFields(&policy)
		qp.Order("name")
		qp.Offset(offset).Limit(paginationDefaultPageSize)
		meta, err := c.APIClient().Query(
			ctx,
			RequestConfig{
				Method:      "GET",
				Endpoint:    replicationRuleURL,
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
