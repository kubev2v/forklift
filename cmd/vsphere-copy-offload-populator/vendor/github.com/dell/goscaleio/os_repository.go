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

const osRepoPath = "/api/v1/OSRepository"

// GetAllOSRepositories Gets all OS Repositories
func (s *System) GetAllOSRepositories() ([]types.OSRepository, error) {
	defer TimeSpent("GetAllOSRepositories", time.Now())

	var osRepositories []types.OSRepository
	err := s.client.getJSONWithRetry(
		http.MethodGet, osRepoPath, nil, &osRepositories)
	if err != nil {
		return nil, err
	}
	return osRepositories, nil
}

// GetOSRepositoryByID Gets OS Repository by ID
func (s *System) GetOSRepositoryByID(id string) (*types.OSRepository, error) {
	defer TimeSpent("GetOSRepositoryByID", time.Now())

	pathWithID := fmt.Sprintf("%v/%v", osRepoPath, id)
	var osRepository types.OSRepository
	err := s.client.getJSONWithRetry(
		http.MethodGet, pathWithID, nil, &osRepository)
	if err != nil {
		return nil, err
	}

	return &osRepository, nil
}

// CreateOSRepository Creates OS Repository
func (s *System) CreateOSRepository(createOSRepository *types.OSRepository) (*types.OSRepository, error) {
	defer TimeSpent("CreateOSRepository", time.Now())
	var createResponse types.OSRepository
	if createOSRepository == nil {
		return &createResponse, fmt.Errorf("createOSRepository cannot be nil")
	}
	bodyData := map[string]interface{}{
		"name":       createOSRepository.Name,
		"repoType":   createOSRepository.RepoType,
		"sourcePath": createOSRepository.SourcePath,
		"imageType":  createOSRepository.ImageType,
	}
	err := s.client.getJSONWithRetry(
		http.MethodPost, osRepoPath, bodyData, &createResponse)
	if err != nil {
		return nil, err
	}

	return &createResponse, nil
}

// RemoveOSRepository Removes OS Repository
func (s *System) RemoveOSRepository(id string) error {
	defer TimeSpent("RemoveOSRepository", time.Now())
	pathWithID := fmt.Sprintf("%v/%v", osRepoPath, id)
	err := s.client.getJSONWithRetry(
		http.MethodDelete, pathWithID, nil, nil)
	if err != nil {
		return err
	}

	return nil
}
