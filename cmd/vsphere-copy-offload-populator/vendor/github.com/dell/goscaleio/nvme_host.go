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
	"fmt"
	"net/http"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// NvmeHost defines struct for NvmeHost
type NvmeHost struct {
	NvmeHost *types.NvmeHost
	client   *Client
}

// NewNvmeHost returns a new NvmeHost
func NewNvmeHost(client *Client, nvmeHost *types.NvmeHost) *NvmeHost {
	return &NvmeHost{
		NvmeHost: nvmeHost,
		client:   client,
	}
}

// GetAllNvmeHosts returns all NvmeHost list
func (s *System) GetAllNvmeHosts() ([]types.NvmeHost, error) {
	defer TimeSpent("GetAllNvmeHosts", time.Now())

	path := fmt.Sprintf("/api/instances/System::%v/relationships/Sdc",
		s.System.ID)

	var allHosts []types.NvmeHost
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &allHosts)
	if err != nil {
		return nil, err
	}

	var nvmeHosts []types.NvmeHost
	for _, host := range allHosts {
		if host.HostType == "NVMeHost" {
			nvmeHosts = append(nvmeHosts, host)
		}
	}

	return nvmeHosts, nil
}

// GetNvmeHostByID returns an NVMe host searched by id
func (s *System) GetNvmeHostByID(id string) (*types.NvmeHost, error) {
	defer TimeSpent("GetNvmeHostByID", time.Now())

	path := fmt.Sprintf("api/instances/Sdc::%v", id)

	var nvmeHost types.NvmeHost
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &nvmeHost)
	if err != nil {
		return nil, err
	}

	return &nvmeHost, nil
}

// CreateNvmeHost creates a new NVMe host
func (s *System) CreateNvmeHost(nvmeHostParam types.NvmeHostParam) (*types.NvmeHostResp, error) {
	defer TimeSpent("CreateNvmeHost", time.Now())

	path := "/api/types/Host/instances"
	nvmeHostResp := &types.NvmeHostResp{}

	err := s.client.getJSONWithRetry(
		http.MethodPost, path, nvmeHostParam, nvmeHostResp)
	if err != nil {
		return nil, err
	}

	return nvmeHostResp, nil
}

// ChangeNvmeHostName changes the name of the Nvme host.
func (s *System) ChangeNvmeHostName(id, name string) error {
	defer TimeSpent("ChangeNvmeHostName", time.Now())

	path := fmt.Sprintf("/api/instances/Sdc::%v/action/setSdcName", id)

	body := types.ChangeNvmeHostNameParam{
		SdcName: name,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// ChangeNvmeHostMaxNumPaths changes the max number paths of the Nvme host.
func (s *System) ChangeNvmeHostMaxNumPaths(id string, maxNumPaths int) error {
	defer TimeSpent("ChangeNvmeHostMaxNumPaths", time.Now())

	path := fmt.Sprintf("/api/instances/Host::%v/action/modifyMaxNumPaths", id)

	body := types.ChangeNvmeMaxNumPathsParam{
		MaxNumPaths: types.IntString(maxNumPaths),
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// ChangeNvmeHostMaxNumSysPorts changes the max number of sys ports of the Nvme host.
func (s *System) ChangeNvmeHostMaxNumSysPorts(id string, maxNumSysPorts int) error {
	defer TimeSpent("ChangeNvmeHostMaxNumPaths", time.Now())

	path := fmt.Sprintf("/api/instances/Host::%v/action/modifyMaxNumSysPorts", id)

	body := types.ChangeNvmeHostMaxNumSysPortsParam{
		MaxNumSysPorts: types.IntString(maxNumSysPorts),
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// DeleteNvmeHost deletes the NVMe host
func (s *System) DeleteNvmeHost(id string) error {
	defer TimeSpent("DeleteNvmeHost", time.Now())

	path := fmt.Sprintf("/api/instances/Sdc::%v/action/removeSdc", id)

	param := &types.EmptyPayload{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, param, nil)
	if err != nil {
		return err
	}
	return nil
}

// GetHostNvmeControllers returns all attached NVMe controllers
func (s *System) GetHostNvmeControllers(host types.NvmeHost) ([]types.NvmeController, error) {
	defer TimeSpent("GetHostNvmeControllers", time.Now())
	path := fmt.Sprintf("api/instances/Host::%v/relationships/NvmeController", host.ID)

	var nvmeControllers []types.NvmeController
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &nvmeControllers)
	return nvmeControllers, err
}
