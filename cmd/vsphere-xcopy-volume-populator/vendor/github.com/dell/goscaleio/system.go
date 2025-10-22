// Copyright © 2019 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// System defines struct for System
type System struct {
	System *types.System
	client *Client
}

// NewSystem returns a new system
func NewSystem(client *Client) *System {
	return &System{
		System: &types.System{},
		client: client,
	}
}

// GetSystems returns systems
func (c *Client) GetSystems() ([]*types.System, error) {
	defer TimeSpent("GetSystems", time.Now())

	systems, err := c.GetInstance("")
	if err != nil {
		return nil, fmt.Errorf("err: problem getting instances: %s", err)
	}
	return systems, nil
}

// FindSystem returns a system based on ID or name
func (c *Client) FindSystem(
	instanceID, name, href string,
) (*System, error) {
	defer TimeSpent("FindSystem", time.Now())

	systems, err := c.GetInstance(href)
	if err != nil {
		return nil, fmt.Errorf("err: problem getting instances: %s", err)
	}

	for _, system := range systems {
		if system.ID == instanceID || system.Name == name || href != "" {
			outSystem := NewSystem(c)
			outSystem.System = system
			return outSystem, nil
		}
	}
	return nil, fmt.Errorf("err: systemid or systemname not found")
}

// GetStatistics returns system statistics
func (s *System) GetStatistics() (*types.Statistics, error) {
	defer TimeSpent("GetStatistics", time.Now())

	link, err := GetLink(s.System.Links,
		"/api/System/relationship/Statistics")
	if err != nil {
		return nil, err
	}

	stats := types.Statistics{}
	err = s.client.getJSONWithRetry(
		http.MethodGet, link.HREF, nil, &stats)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// CreateSnapshotConsistencyGroup creates a snapshot consistency group
func (s *System) CreateSnapshotConsistencyGroup(
	snapshotVolumesParam *types.SnapshotVolumesParam,
) (*types.SnapshotVolumesResp, error) {
	defer TimeSpent("CreateSnapshotConsistencyGroup", time.Now())

	link, err := GetLink(s.System.Links, "self")
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%v/action/snapshotVolumes", link.HREF)

	snapResp := types.SnapshotVolumesResp{}
	err = s.client.getJSONWithRetry(
		http.MethodPost, path, snapshotVolumesParam, &snapResp)
	if err != nil {
		return nil, err
	}

	return &snapResp, nil
}

// GetMDMClusterDetails returns MDM cluster details
func (s *System) GetMDMClusterDetails() (*types.MdmCluster, error) {
	defer TimeSpent("GetMDMClusterDetails", time.Now())

	path := "api/instances/System/queryMdmCluster"
	mdmParam := &types.EmptyPayload{}

	mdmResp := types.MdmCluster{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, mdmParam, &mdmResp)
	if err != nil {
		return nil, err
	}

	return &mdmResp, nil
}

// AddStandByMdm adds the standby MDMs to the MDM cluster
func (s *System) AddStandByMdm(mdmParam *types.StandByMdm) (string, error) {
	defer TimeSpent("AddStandByMdm", time.Now())

	path := "api/instances/System/action/addStandbyMdm"
	mdm := &types.Mdm{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, mdmParam, &mdm)
	if err != nil {
		return "", err
	}
	return mdm.ID, nil
}

// RemoveStandByMdm removes standby MDM
func (s *System) RemoveStandByMdm(id string) error {
	defer TimeSpent("RemoveStandByMdm", time.Now())

	path := "/api/instances/System/action/removeStandbyMdm"
	mdmParam := &types.RemoveStandByMdmParam{
		ID: id,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, mdmParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// ModifyPerformanceProfileMdmCluster modifies performance profile of MDM cluster
func (s *System) ModifyPerformanceProfileMdmCluster(perfProfile string) error {
	defer TimeSpent("ModifyPerformanceProfileMdmCluster", time.Now())

	path := "/api/instances/System/action/setMdmPerformanceParameters"
	mdmParam := &types.ChangeMdmPerfProfile{
		PerfProfile: perfProfile,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, mdmParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// SwitchClusterMode changes the MDM cluster mode
func (s *System) SwitchClusterMode(switchClusterMode *types.SwitchClusterMode) error {
	defer TimeSpent("SwitchClusterMode", time.Now())

	path := "/api/instances/System/action/switchClusterMode"

	err := s.client.getJSONWithRetry(
		http.MethodPost, path, switchClusterMode, nil)
	if err != nil {
		return err
	}
	return nil
}

// ChangeMdmOwnerShip modifies the primary MDM
func (s *System) ChangeMdmOwnerShip(id string) error {
	defer TimeSpent("ChangeMdmOwnerShip", time.Now())

	path := "/api/instances/System/action/changeMdmOwnership"
	mdmParam := &types.ChangeMdmOwnerShip{
		ID: id,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, mdmParam, nil)
	if err != nil {
		return err
	}
	return nil
}

// RenameMdm modifies name of the MDM
func (s *System) RenameMdm(renameMdm *types.RenameMdm) error {
	defer TimeSpent("ChangeMdmOwnerShip", time.Now())

	path := "/api/instances/System/action/renameMdm"
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, renameMdm, nil)
	if err != nil {
		return err
	}
	return nil
}
