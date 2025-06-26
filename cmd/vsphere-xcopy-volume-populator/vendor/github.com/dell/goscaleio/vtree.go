// Copyright © 2019 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// GetVTrees returns vtrees present in the cluster
func (c *Client) GetVTrees() ([]types.VTreeDetails, error) {
	defer TimeSpent("GetVTrees", time.Now())

	path := "/api/types/VTree/instances"

	var vTree []types.VTreeDetails
	err := c.getJSONWithRetry(http.MethodGet, path, nil, &vTree)
	if err != nil {
		return nil, err
	}

	return vTree, nil
}

// GetVTreeByID returns the VTree details for the given ID
func (c *Client) GetVTreeByID(id string) (*types.VTreeDetails, error) {
	defer TimeSpent("GetVTreeByID", time.Now())

	path := fmt.Sprintf("/api/instances/VTree::%v", id)

	var vTree types.VTreeDetails
	err := c.getJSONWithRetry(http.MethodGet, path, nil, &vTree)
	if err != nil {
		return nil, err
	}
	return &vTree, nil
}

// GetVTreeInstances returns the VTree details for the given IDs
func (c *Client) GetVTreeInstances(ids []string) ([]types.VTreeDetails, error) {
	defer TimeSpent("GetVTrees", time.Now())

	path := "/api/types/VTree/instances/action/queryBySelectedIds"

	payload := types.VTreeQueryBySelectedIDsParam{
		IDs: ids,
	}
	var vTree []types.VTreeDetails
	err := c.getJSONWithRetry(http.MethodPost, path, payload, &vTree)
	if err != nil {
		return nil, err
	}
	return vTree, nil
}

// GetVTreeByVolumeID returns VTree details based on Volume ID
func (c *Client) GetVTreeByVolumeID(id string) (*types.VTreeDetails, error) {
	defer TimeSpent("GetVTreeByVolumeID", time.Now())

	volDetails, err := c.GetVolume("", id, "", "", false)
	if err != nil {
		return nil, err
	}

	return c.GetVTreeByID(volDetails[0].VTreeID)
}
