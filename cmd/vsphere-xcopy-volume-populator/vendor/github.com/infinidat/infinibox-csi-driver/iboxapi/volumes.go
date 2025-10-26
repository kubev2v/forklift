package iboxapi

/*
Copyright 2025 Infinidat
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type GetLunsByVolumeResponse struct {
	Metadata Metadata  `json:"metadata"`
	Result   []LunInfo `json:"result"`
	Error    Error     `json:"error"`
}

type Volume struct {
	CGID                  int    `json:"cg_id,omitempty"`
	RmrTarget             bool   `json:"rmr_target,omitempty"`
	UpdatedAt             int    `json:"updated_at,omitempty"`
	NumBlocks             int    `json:"num_blocks,omitempty"`
	Allocated             int    `json:"allocated,omitempty"`
	Serial                string `json:"serial,omitempty"`
	Size                  int64  `json:"size,omitempty"`
	SsdEnabled            bool   `json:"ssd_enabled,omitempty"`
	ID                    int    `json:"id,omitempty"`
	ParentID              int    `json:"parent_id,omitempty"`
	CompressionSuppressed bool   `json:"compression_suppressed,omitempty"`
	Type                  string `json:"type,omitempty"`
	RmrSource             bool   `json:"rmr_source,omitempty"`
	Used                  int    `json:"used,omitempty"`
	TreeAllocated         int    `json:"tree_allocated,omitempty"`
	HasChildren           bool   `json:"has_children,omitempty"`
	DatasetType           string `json:"dataset_type,omitempty"`
	Provtype              string `json:"provtype,omitempty"`
	RmrSnapshotGUID       string `json:"rmr_snapshot_guid,omitempty"`
	CapacitySavings       int    `json:"capacity_savings,omitempty"`
	Name                  string `json:"name,omitempty"`
	CreatedAt             int64  `json:"created_at,omitempty"`
	PoolID                int    `json:"pool_id,omitempty"`
	PoolName              string `json:"pool_name,omitempty"`
	CompressionEnabled    bool   `json:"compression_enabled,omitempty"`
	FamilyID              int    `json:"family_id,omitempty"`
	Depth                 int    `json:"depth,omitempty"`
	WriteProtected        bool   `json:"write_protected,omitempty"`
	Mapped                bool   `json:"mapped,omitempty"`
	LockExpiresAt         int64  `json:"lock_expires_at,omitempty"`
	LockState             string `json:"lock_state,omitempty"`
}

type CreateVolumeRequest struct {
	PoolID        int    `json:"pool_id,omitempty"`
	VolumeSize    int64  `json:"size,omitempty"`
	Name          string `json:"name,omitempty"`
	ProvisionType string `json:"provtype,omitempty"`
	SSDEnabled    bool   `json:"ssd_enabled,omitempty"`
}

type CreateVolumeResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Volume   `json:"result"`
	Error    Error    `json:"error"`
}

type UpdateVolumeResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Volume   `json:"result"`
	Error    Error    `json:"error"`
}

type DeleteVolumeResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Volume   `json:"result"`
	Error    Error    `json:"error"`
}

type GetVolumeByNameResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Volume `json:"result"`
	Error    Error    `json:"error"`
}

type GetVolumeResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Volume   `json:"result"`
	Error    Error    `json:"error"`
}

type Snapshot struct {
	SnapShotID int    `json:"id,omitempty"`
	Size       int64  `json:"size,omitempty"`
	SSDEnabled bool   `json:"ssd_enabled,omitempty"`
	ParentID   int    `json:"parent_id,omitempty"`
	PoolID     int    `json:"pool_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

type CreateSnapshotVolumeRequest struct {
	ParentID       int    `json:"parent_id"`
	SnapshotName   string `json:"name"`
	WriteProtected bool   `json:"write_protected"`
	SSDEnabled     bool   `json:"ssd_enabled,omitempty"`
	LockExpiresAt  int64  `json:"lock_expires_at,omitempty"`
}

type CreateSnapshotVolumeResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Snapshot `json:"result"`
	Error    Error    `json:"error"`
}

type GetVolumesByParentIDResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Volume `json:"result"`
	Error    Error    `json:"error"`
}

func (iboxClient *IboxClient) GetLunsByVolume(volumeID int) (results []LunInfo, err error) {
	const functionName = "GetLunsByVolume"
	url := fmt.Sprintf("%s%s/%d/luns", iboxClient.Creds.URL, "api/rest/volumes", volumeID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume ID", volumeID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return results, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetLunsByVolumeResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (iboxClient *IboxClient) CreateVolume(req CreateVolumeRequest) (*Volume, error) {
	const functionName = "CreateVolume"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/volumes")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateVolumeResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "Volume ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) DeleteVolume(volumeID int) (response *DeleteVolumeResponse, err error) {
	const functionName = "DeleteVolume"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/volumes", volumeID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume ID", volumeID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll -error %w", functionName, err)
	}
	var responseObject DeleteVolumeResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		// TODO check for NOT FOUND?  have callers check for ErrNotFound?
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject, nil
}

func (iboxClient *IboxClient) GetVolumeByName(volumeName string) (volume *Volume, err error) {
	const functionName = "GetVolumeByName"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/volumes")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume Name", volumeName)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add("name", volumeName)
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetVolumeByNameResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		if responseObject.Error.Code != "" {
			// TODO check for NOT FOUND?  return ErrNotFound for callers?
			return nil, fmt.Errorf("%s - ibox API - error code %s message %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
		}
		if len(responseObject.Result) > 0 {
			volume = &responseObject.Result[0]
		} else {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - volume name '%s' not found", functionName, volumeName)}
		}

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return volume, nil
}

func (iboxClient *IboxClient) GetVolume(volumeID int) (volume *Volume, err error) {
	const functionName = "GetVolume"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/volumes", volumeID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume ID", volumeID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetVolumeResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "VOLUME_NOT_FOUND" {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - volume ID '%d' not found", functionName, volumeID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) UpdateVolume(volumeID int, volume Volume) (*Volume, error) {
	const functionName = "UpdateVolume"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/volumes/", volumeID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume ID", volumeID)

	jsonBytes, err := json.Marshal(volume)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	SetAuthHeader(request, iboxClient.Creds)

	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject UpdateVolumeResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		// TODO check for NOT FOUND?  return ErrNotFound for callers?
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) CreateSnapshotVolume(req CreateSnapshotVolumeRequest) (*Snapshot, error) {
	const functionName = "CreateSnapshotVolume"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/volumes")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	if req.LockExpiresAt > 0 {
		values := request.URL.Query()
		values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
		request.URL.RawQuery = values.Encode()
	}

	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateSnapshotVolumeResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "Snapshot ID", responseObject.Result.SnapShotID)
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetVolumesByParentID(parentID int) (volumes []Volume, err error) {
	const functionName = "GetVolumesByParentID"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/volumes")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "parent ID", parentID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return volumes, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add("parent_id", strconv.Itoa(parentID))
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return volumes, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(TRACE_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return volumes, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetVolumesByParentIDResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return volumes, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		if responseObject.Error.Code != "" {
			return volumes, fmt.Errorf("%s - ibox API - error code %s message %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
		}

		volumes = append(volumes, responseObject.Result...)
		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return volumes, nil
}
