// Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goscaleio

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// GetNodeByID gets the node details based on ID
func (gc *GatewayClient) GetNodeByID(id string) (*types.NodeDetails, error) {
	defer TimeSpent("GetNodeByID", time.Now())

	path := fmt.Sprintf("/Api/V1/ManagedDevice/%v", id)

	var node types.NodeDetails
	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode == 200 {
		parseError := json.Unmarshal([]byte(responseString), &node)

		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Node: %s", parseError)
		}
	} else {
		return nil, fmt.Errorf("Couldn't find nodes with the given filter")
	}
	return &node, nil
}

// GetAllNodes gets all the node details
func (gc *GatewayClient) GetAllNodes() ([]types.NodeDetails, error) {
	defer TimeSpent("GetNodeByID", time.Now())

	path := fmt.Sprintf("/Api/V1/ManagedDevice")

	var nodes []types.NodeDetails
	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode == 200 {
		parseError := json.Unmarshal([]byte(responseString), &nodes)

		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Node: %s", parseError)
		}
	} else {
		return nil, fmt.Errorf("Couldn't find nodes with the given filter")
	}
	return nodes, nil
}

// GetNodeByFilters gets the node details based on the provided filter
func (gc *GatewayClient) GetNodeByFilters(key string, value string) ([]types.NodeDetails, error) {
	defer TimeSpent("GetNodeByFilters", time.Now())

	path := fmt.Sprintf("/Api/V1/ManagedDevice?filter=eq,%v,%v", key, value)

	var nodes []types.NodeDetails
	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode == 200 {
		parseError := json.Unmarshal([]byte(responseString), &nodes)
		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Node: %s", parseError)
		}

		if len(nodes) == 0 {
			return nil, errors.New("Couldn't find nodes with the given filter")
		}
	} else {
		return nil, fmt.Errorf("Couldn't find nodes with the given filter")
	}
	return nodes, nil
}

// GetNodePoolByID gets the nodepool details based on ID
func (gc *GatewayClient) GetNodePoolByID(id int) (*types.NodePoolDetails, error) {
	defer TimeSpent("GetNodePoolByID", time.Now())

	path := fmt.Sprintf("/Api/V1/nodepool/%v", id)

	var nodePool types.NodePoolDetails
	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode == 200 && responseString != "" {
		parseError := json.Unmarshal([]byte(responseString), &nodePool)
		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Nodepool: %s", parseError)
		}

	} else {
		return nil, fmt.Errorf("Couldn't find nodes with the given filter")
	}

	return &nodePool, nil
}

// GetNodePoolByName gets the nodepool details based on name
func (gc *GatewayClient) GetNodePoolByName(name string) (*types.NodePoolDetails, error) {
	defer TimeSpent("GetNodePoolByName", time.Now())

	nodePools, err := gc.GetAllNodePools()
	if err != nil {
		return nil, err
	}

	for _, nodePool := range nodePools.NodePoolDetails {
		if nodePool.GroupName == name {
			return gc.GetNodePoolByID(nodePool.GroupSeqID)
		}
	}
	return nil, errors.New("no node pool found with name " + name)
}

// GetAllNodePools gets all the nodepool details
func (gc *GatewayClient) GetAllNodePools() (*types.NodePoolDetailsFilter, error) {
	defer TimeSpent("GetAllNodePools", time.Now())

	path := fmt.Sprintf("/Api/V1/nodepool")

	var nodePools types.NodePoolDetailsFilter
	req, httpError := http.NewRequest(http.MethodGet, gc.host+path, nil)
	if httpError != nil {
		return nil, httpError
	}

	if gc.version == "4.0" {
		req.Header.Set("Authorization", "Bearer "+gc.token)

		err := setCookie(req.Header, gc.host)
		if err != nil {
			return nil, fmt.Errorf("Error While Handling Cookie: %s", err)
		}

	} else {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(gc.username+":"+gc.password)))
	}

	req.Header.Set("Content-Type", "application/json")

	client := gc.http
	httpResp, httpRespError := client.Do(req)
	if httpRespError != nil {
		return nil, httpRespError
	}

	responseString, _ := extractString(httpResp)

	if httpResp.StatusCode == 200 {
		parseError := json.Unmarshal([]byte(responseString), &nodePools)

		if parseError != nil {
			return nil, fmt.Errorf("Error While Parsing Response Data For Nodepool: %s", parseError)
		}
	} else {
		return nil, fmt.Errorf("Couldn't find nodes with the given filter")
	}
	return &nodePools, nil
}
