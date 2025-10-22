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

// Sdt defines struct for Sdt
type Sdt struct {
	Sdt    *types.Sdt
	client *Client
}

// NewSdt returns a new Sdt
func NewSdt(client *Client, sdt *types.Sdt) *Sdt {
	return &Sdt{
		Sdt:    sdt,
		client: client,
	}
}

// GetAllSdts returns all sdt
func (s *System) GetAllSdts() ([]types.Sdt, error) {
	defer TimeSpent("GetAllSdts", time.Now())

	path := "/api/types/Sdt/instances"

	var allSdts []types.Sdt
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &allSdts)
	if err != nil {
		return nil, err
	}

	return allSdts, nil
}

// GetSdtByID returns an sdt searched by id
func (s *System) GetSdtByID(id string) (*types.Sdt, error) {
	defer TimeSpent("GetSdtByID", time.Now())

	path := fmt.Sprintf("api/instances/Sdt::%v", id)

	var sdt types.Sdt
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &sdt)
	if err != nil {
		return nil, err
	}

	return &sdt, nil
}

// CreateSdt creates a new Sdt
func (pd *ProtectionDomain) CreateSdt(param *types.SdtParam) (*types.SdtResp, error) {
	defer TimeSpent("CreateSdt", time.Now())

	if len(param.IPList) == 0 {
		return nil, fmt.Errorf("Must provide at least 1 SDT IP")
	}

	param.ProtectionDomainID = pd.ProtectionDomain.ID

	resp := &types.SdtResp{}
	path := "/api/types/Sdt/instances"
	err := pd.client.getJSONWithRetry(
		http.MethodPost, path, param, resp)
	return resp, err
}

// RenameSdt changes the name of the sdt.
func (s *System) RenameSdt(id, name string) error {
	defer TimeSpent("RenameSdt", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/renameSdt", id)

	body := types.SdtRenameParam{
		NewName: name,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// SetSdtNvmePort set the NVMe port for the sdt.
func (s *System) SetSdtNvmePort(id string, port int) error {
	defer TimeSpent("SetSdtNvmePort", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/modifyNvmePort", id)

	body := types.SdtNvmePortParam{
		NewNvmePort: types.IntString(port),
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// SetSdtStoragePort sets the storage port for the sdt.
func (s *System) SetSdtStoragePort(id string, port int) error {
	defer TimeSpent("SetSdtStoragePort", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/modifyStoragePort", id)

	body := types.SdtStoragePortParam{
		NewStoragePort: types.IntString(port),
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// SetSdtDiscoveryPort sets the discovery port for the sdt.
func (s *System) SetSdtDiscoveryPort(id string, port int) error {
	defer TimeSpent("SetSdtDiscoveryPort", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/modifyDiscoveryPort", id)

	body := types.SdtDiscoveryPortParam{
		NewDiscoveryPort: types.IntString(port),
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// AddSdtTargetIP adds target IP and role for the sdt.
func (s *System) AddSdtTargetIP(id, ip, role string) error {
	defer TimeSpent("AddSdtTargetIP", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/addIp", id)

	body := types.SdtIP{
		IP:   ip,
		Role: role,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// RemoveSdtTargetIP removes target IP and role from the sdt.
func (s *System) RemoveSdtTargetIP(id, ip string) error {
	defer TimeSpent("RemoveSdtTargetIP", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/removeIp", id)

	body := types.SdtRemoveIPParam{
		IP: ip,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// ModifySdtIPRole modify target IP role for the sdt.
func (s *System) ModifySdtIPRole(id, ip, role string) error {
	defer TimeSpent("ModifySdtIPRole", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/modifyIpRole", id)

	body := types.SdtIPRoleParam{
		IP:   ip,
		Role: role,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}

	return nil
}

// DeleteSdt deletes the sdt
func (s *System) DeleteSdt(id string) error {
	defer TimeSpent("DeleteSdt", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/removeSdt", id)

	param := &types.EmptyPayload{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, param, nil)
	if err != nil {
		return err
	}
	return nil
}

// EnterSdtMaintenanceMode enter sdt maintenance mode
func (s *System) EnterSdtMaintenanceMode(id string) error {
	defer TimeSpent("EnterSdtMaintenanceMode", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/enterMaintenanceMode", id)

	param := &types.EmptyPayload{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, param, nil)
	if err != nil {
		return err
	}
	return nil
}

// ExitSdtMaintenanceMode exit sdt maintenance mode
func (s *System) ExitSdtMaintenanceMode(id string) error {
	defer TimeSpent("ExitSdtMaintenanceMode", time.Now())

	path := fmt.Sprintf("/api/instances/Sdt::%v/action/exitMaintenanceMode", id)

	param := &types.EmptyPayload{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, param, nil)
	if err != nil {
		return err
	}
	return nil
}
