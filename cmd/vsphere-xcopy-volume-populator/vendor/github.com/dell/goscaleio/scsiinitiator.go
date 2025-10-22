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

// GetScsiInitiator returns a ScsiInitiator
func (s *System) GetScsiInitiator() ([]types.ScsiInitiator, error) {
	defer TimeSpent("GetScsiInitiator", time.Now())

	path := fmt.Sprintf(
		"/api/instances/System::%v/relationships/ScsiInitiator",
		s.System.ID)

	var si []types.ScsiInitiator
	err := s.client.getJSONWithRetry(
		http.MethodGet, path, nil, &si)
	if err != nil {
		return nil, err
	}

	return si, nil
}
