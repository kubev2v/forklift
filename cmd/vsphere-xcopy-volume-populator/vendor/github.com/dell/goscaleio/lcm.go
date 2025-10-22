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
	"strconv"
	"strings"
	"time"

	types "github.com/dell/goscaleio/types/v1"
)

// CheckPfmpVersion checks if the PFMP version is greater than the given version
// Returns -1 if PFMP version < the given version,
// Returns 1 if PFMP version > the given version,
// Returns 0 if PFMP version == the given version.
func CheckPfmpVersion(client *Client, version string) (int, error) {
	defer TimeSpent("CheckPfmpVersion", time.Now())

	lcmStatus, err := GetPfmpStatus(*client)
	if err != nil {
		return -1, fmt.Errorf("failed to get PFMP version : %v", err)
	}

	result, err := CompareVersion(lcmStatus.ClusterVersion, version)
	if err != nil {
		return -1, err
	}
	return result, nil
}

// GetPfmpStatus gets the PFMP status
func GetPfmpStatus(client Client) (*types.LcmStatus, error) {
	defer TimeSpent("GetPfmpStatus", time.Now())

	path := "/Api/V1/corelcm/status"

	var status types.LcmStatus
	err := client.getJSONWithRetry(
		http.MethodGet, path, nil, &status)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

// CompareVersion compares two version strings.
// Returns -1 if versionA < versionB,
// Returns 1 if versionA > versionB,
// Returns 0 if versionA == versionB.
func CompareVersion(versionA, versionB string) (int, error) {
	partsA := strings.Split(versionA, ".")
	partsB := strings.Split(versionB, ".")

	maxLength := len(partsA)
	if len(partsB) > maxLength {
		maxLength = len(partsB)
	}

	// Compare each part of the versions
	for i := 0; i < maxLength; i++ {
		var partA, partB int
		var err error

		if i < len(partsA) {
			partA, err = strconv.Atoi(partsA[i])
			if err != nil {
				err := fmt.Errorf("error parsing part PFMP version: %s", versionA)
				return -1, err
			}
		}

		if i < len(partsB) {
			partB, err = strconv.Atoi(partsB[i])
			if err != nil {
				err := fmt.Errorf("error parsing part PFMP version: %s", versionB)
				return -1, err
			}
		}

		if partA < partB {
			return -1, nil
		} else if partA > partB {
			return 1, nil
		}
	}

	return 0, nil
}
