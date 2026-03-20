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
	"net/http"

	types "github.com/dell/goscaleio/types/v1"
)

// GetCompatibilityManagement Gets Compatibility Management
func (s *System) GetCompatibilityManagement() (*types.CompatibilityManagement, error) {
	path := "/api/v1/Compatibility"
	var compatibilityManagement types.CompatibilityManagement
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &compatibilityManagement)
	if err != nil {
		return nil, err
	}

	return &compatibilityManagement, nil
}

// SetCompatibilityManagement Sets Compatibility Management
func (s *System) SetCompatibilityManagement(compatibilityManagement *types.CompatibilityManagementPost) (*types.CompatibilityManagement, error) {
	path := "/api/v1/Compatibility"
	resp := types.CompatibilityManagement{}
	err := s.client.getJSONWithRetry(
		http.MethodPost, path, compatibilityManagement, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
