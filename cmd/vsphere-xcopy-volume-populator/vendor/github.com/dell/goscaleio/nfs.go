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
	"errors"
	"fmt"
	"net/http"
	"strings"

	types "github.com/dell/goscaleio/types/v1"
)

// GetFileInterface gets a FileInterface by id
func (s *System) GetFileInterface(id string) (*types.FileInterface, error) {
	if id == "" {
		return nil, errors.New("id is mandatory, please enter a valid value")
	}
	path := fmt.Sprintf("/rest/v1/file-interfaces/%s?select=*", id)

	var resp *types.FileInterface

	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, errors.New("could not find the File interface using id")
	}

	return resp, nil
}

// GetNASByIDName gets a NAS server by name or ID
func (s *System) GetNASByIDName(id string, name string) (*types.NAS, error) {
	var nasList []types.NAS

	if name == "" && id == "" {
		return nil, errors.New("NAS server name or ID is mandatory, please enter a valid value")
	}

	// Get NAS server by id
	if id != "" {
		path := fmt.Sprintf("/rest/v1/nas-servers/%s?select=*", id)

		var resp *types.NAS
		err := s.client.getJSONWithRetry(
			http.MethodGet, path, nil, &resp)
		if err != nil {
			return nil, errors.New("could not find NAS server by id")
		}
		return resp, nil

	}

	// Get NAS server by name
	path := "/rest/v1/nas-servers?select=*"
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &nasList)
	if err != nil {
		return nil, fmt.Errorf("could not find NAS server by name: %s, err: %s", name, err.Error())
	}

	name = strings.ToLower(name)
	for _, nas := range nasList {
		if strings.Contains(name, strings.ToLower(nas.Name)) {
			return &nas, nil
		}
	}

	return nil, errors.New("could not find given NAS server by name")
}

// CreateNAS creates a NAS server
func (s *System) CreateNAS(name string, protectionDomainID string) (*types.CreateNASResponse, error) {
	var resp types.CreateNASResponse

	path := "/rest/v1/nas-servers"

	var body types.CreateNASParam = types.CreateNASParam{
		Name:               name,
		ProtectionDomainID: protectionDomainID,
	}

	err := s.client.getJSONWithRetry(http.MethodPost, path, body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// DeleteNAS deletes a NAS server
func (s *System) DeleteNAS(id string) error {
	path := fmt.Sprintf("/rest/v1/nas-servers/%s", id)

	err := s.client.getJSONWithRetry(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// PingNAS pings a NAS server
func (s *System) PingNAS(id string, ipaddress string) error {
	path := fmt.Sprintf("rest/v1/nas-servers/%s/ping", id)
	body := types.PingNASParam{
		DestinationAddress: ipaddress,
		IsIPV6:             false,
	}

	err := s.client.getJSONWithRetry(http.MethodPost, path, body, nil)
	if err != nil {
		return errors.New("Could not ping NAS server " + id)
	}

	return nil
}
