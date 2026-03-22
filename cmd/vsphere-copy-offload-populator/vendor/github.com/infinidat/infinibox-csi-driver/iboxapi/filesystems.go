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

/**
log levels
logr.V(0) - Info level logging in zerolog
logr.V(1) - Debug level logging in zerolog
logr.V(2) - Trace level logging in zerolog
*/

type FileSystemSnapshotResponse struct {
	SnapshotID  int    `json:"id"`
	Name        string `json:"name,omitempty"`
	DatasetType string `json:"dataset_type,omitempty"`
	ParentID    int    `json:"parent_id,omitempty"`
	Size        int64  `json:"size,omitempty"`
	CreatedAt   int64  `json:"created_at,omitempty"`
}
type FileSystem struct {
	Type                                string   `json:"type,omitempty"`
	Depth                               int      `json:"depth,omitempty"`
	ID                                  int      `json:"id,omitempty"`
	Name                                string   `json:"name,omitempty"`
	CreatedAt                           int64    `json:"created_at,omitempty"`
	UpdatedAt                           int64    `json:"updated_at,omitempty"`
	Mapped                              bool     `json:"mapped,omitempty"`
	WriteProtected                      bool     `json:"write_protected,omitempty"`
	Size                                int64    `json:"size,omitempty"`
	Provtype                            string   `json:"provtype,omitempty"`
	SSDEnabled                          bool     `json:"ssd_enabled,omitempty"`
	SSAExpressEnabled                   bool     `json:"ssa_express_enabled,omitempty"`
	SSAExpressStatus                    any      `json:"ssa_express_status,omitempty"`
	CompressionEnabled                  bool     `json:"compression_enabled,omitempty"`
	Serial                              string   `json:"serial,omitempty"`
	RmrTarget                           bool     `json:"rmr_target,omitempty"`
	RmrSource                           bool     `json:"rmr_source,omitempty"`
	RmrActiveActivePeer                 bool     `json:"rmr_active_active_peer,omitempty"`
	MobilitySource                      any      `json:"mobility_source,omitempty"`
	RmrSnapshotGUID                     any      `json:"rmr_snapshot_guid,omitempty"`
	DataSnapshotGUID                    any      `json:"data_snapshot_guid,omitempty"`
	MgmtSnapshotGUID                    any      `json:"mgmt_snapshot_guid,omitempty"`
	CgSnapshotGUID                      any      `json:"_cg_snapshot_guid,omitempty"`
	CgGUID                              any      `json:"_cg_guid,omitempty"`
	FamilyID                            int      `json:"family_id,omitempty"`
	LockExpiresAt                       int64    `json:"lock_expires_at,omitempty"`
	ReclaimedSnapshotRemoteSystemSerial any      `json:"_reclaimed_snapshot_remote_system_serial,omitempty"`
	SnapshotRetention                   any      `json:"snapshot_retention,omitempty"`
	DatasetType                         string   `json:"dataset_type,omitempty"`
	Used                                int      `json:"used,omitempty"`
	TreeAllocated                       int      `json:"tree_allocated,omitempty"`
	Allocated                           int      `json:"allocated,omitempty"`
	CompressionSuppressed               bool     `json:"compression_suppressed,omitempty"`
	CapacitySavings                     int      `json:"capacity_savings,omitempty"`
	CapacitySavingsPerEntity            int      `json:"capacity_savings_per_entity,omitempty"`
	DiskUsage                           int      `json:"disk_usage,omitempty"`
	DataReductionRatio                  float64  `json:"data_reduction_ratio,omitempty"`
	WormLegalHold                       any      `json:"worm_legal_hold,omitempty"`
	WormDefaultRetention                any      `json:"worm_default_retention,omitempty"`
	WormMaxRetention                    any      `json:"worm_max_retention,omitempty"`
	NfsFilesystemID                     int      `json:"nfs_filesystem_id,omitempty"`
	AtimeMode                           string   `json:"atime_mode,omitempty"`
	IsConsistent                        bool     `json:"is_consistent,omitempty"`
	IsEstablished                       bool     `json:"_is_established,omitempty"`
	SnapdirName                         string   `json:"snapdir_name,omitempty"`
	VisibleInSnapdir                    bool     `json:"visible_in_snapdir,omitempty"`
	SnapdirAccessible                   bool     `json:"snapdir_accessible,omitempty"`
	SuspendState                        string   `json:"suspend_state,omitempty"`
	SecurityStyle                       string   `json:"security_style,omitempty"`
	AtimeGranularity                    int      `json:"atime_granularity,omitempty"`
	WormLevel                           string   `json:"worm_level,omitempty"`
	ParentID                            int      `json:"parent_id,omitempty"`
	Modified                            bool     `json:"modified,omitempty"`
	Data                                int      `json:"data,omitempty"`
	PoolID                              int      `json:"pool_id,omitempty"`
	CgName                              any      `json:"cg_name,omitempty"`
	CgID                                any      `json:"cg_id,omitempty"`
	HasChildren                         bool     `json:"has_children,omitempty"`
	SnapshotPolicyID                    any      `json:"snapshot_policy_id,omitempty"`
	SnapshotExpiresAt                   any      `json:"snapshot_expires_at,omitempty"`
	CreatedBySnapshotPolicyID           any      `json:"created_by_snapshot_policy_id,omitempty"`
	CreatedByScheduleID                 any      `json:"created_by_schedule_id,omitempty"`
	TenantID                            int      `json:"tenant_id,omitempty"`
	QosPolicyName                       any      `json:"qos_policy_name,omitempty"`
	LockState                           string   `json:"lock_state,omitempty"`
	SnapshotPolicyName                  any      `json:"snapshot_policy_name,omitempty"`
	CreatedBySnapshotPolicyName         any      `json:"created_by_snapshot_policy_name,omitempty"`
	CreatedByScheduleName               any      `json:"created_by_schedule_name,omitempty"`
	QosPolicyID                         any      `json:"qos_policy_id,omitempty"`
	QosSharedPolicyID                   any      `json:"qos_shared_policy_id,omitempty"`
	QosSharedPolicyName                 any      `json:"qos_shared_policy_name,omitempty"`
	PoolName                            string   `json:"pool_name,omitempty"`
	Nguid                               string   `json:"nguid,omitempty"`
	ReplicaIDs                          []any    `json:"replica_ids,omitempty"`
	ReplicationTypes                    []string `json:"replication_types,omitempty"`
	NumBlocks                           int      `json:"num_blocks,omitempty"`
}

type UpdateFileSystemResponse struct {
	Metadata Metadata   `json:"metadata"`
	Result   FileSystem `json:"result"`
	Error    Error      `json:"error"`
}
type CreateFileSystemSnapshotResponse struct {
	Metadata Metadata                   `json:"metadata"`
	Result   FileSystemSnapshotResponse `json:"result"`
	Error    Error                      `json:"error"`
}

type GetFileSystemsByPoolResponse struct {
	Metadata Metadata     `json:"metadata"`
	Result   []FileSystem `json:"result"`
	Error    Error        `json:"error"`
}

type GetFileSystemByNameResponse struct {
	Metadata Metadata     `json:"metadata"`
	Result   []FileSystem `json:"result"`
	Error    Error        `json:"error"`
}

type GetFileSystemsByParentIDResponse struct {
	Metadata Metadata     `json:"metadata"`
	Result   []FileSystem `json:"result"`
	Error    Error        `json:"error"`
}

type GetFileSystemByIDResponse struct {
	Metadata Metadata   `json:"metadata"`
	Result   FileSystem `json:"result"`
	Error    Error      `json:"error"`
}

type DeleteFileSystemResponse struct {
	Metadata Metadata   `json:"metadata"`
	Result   FileSystem `json:"result"`
	Error    Error      `json:"error"`
}

type CreateFileSystemRequest struct {
	AtimeMode  string `json:"atime_mode,omitempty"`
	PoolID     int    `json:"pool_id"`
	Name       string `json:"name"`
	Provtype   string `json:"provtype"`
	Size       int64  `json:"size"`
	SsdEnabled bool   `json:"ssd_enabled,omitempty"`
}

type CreateFileSystemResponse struct {
	Metadata Metadata   `json:"metadata"`
	Result   FileSystem `json:"result"`
	Error    Error      `json:"error"`
}

type FileSystemSnapshot struct {
	LockExpiresAt  int64  `json:"lock_expires_at,omitempty"`
	ParentID       int    `json:"parent_id"`
	SnapshotName   string `json:"name"`
	WriteProtected bool   `json:"write_protected"`
	SSDEnabled     bool   `json:"ssd_enabled,omitempty"`
}

const FILESYSTEM_NOT_FOUND = "FILESYSTEM_NOT_FOUND"

func (iboxClient *IboxClient) GetFileSystemsByPool(poolID int, fsPrefix string) (results []FileSystem, err error) {
	const functionName = "GetFileSystemsByPool"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/filesystems")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "pool ID", poolID, "fsprefix", fsPrefix)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add("pool_id", strconv.Itoa(poolID))
		values.Add("name", "like:"+fsPrefix)
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
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetFileSystemsByPoolResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		if responseObject.Error.Code != "" {
			return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (iboxClient *IboxClient) GetFileSystemByID(fsID int) (fs *FileSystem, err error) {
	const functionName = "GetFileSystemByID"

	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "filesystem ID", fsID)

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
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetFileSystemByIDResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - fs ID '%d' not found", functionName, fsID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) CreateFileSystem(req CreateFileSystemRequest) (*FileSystem, error) {
	const functionName = "CreateFileSystem"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/filesystems")
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
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateFileSystemResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "FileSystem ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetFileSystemByName(name string) (*FileSystem, error) {
	const functionName = "GetFileSystemByName"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/filesystems")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "name", name)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	page := 1
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add("name", name)
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
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetFileSystemByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if len(responseObject.Result) == 0 {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - name '%s' not found", functionName, name)}
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - API error - %s", functionName, responseObject.Error.Code)
	}
	return &responseObject.Result[0], nil
}

func (iboxClient *IboxClient) GetFileSystemsByParentID(parentID int) (results []FileSystem, err error) {
	const functionName = "GetFileSystemsByParentID"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/filesystems")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "parent ID", parentID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return results, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add("parent_id", strconv.Itoa(parentID))
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
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetFileSystemsByParentIDResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}

		if responseObject.Error.Code != "" {
			return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (iboxClient *IboxClient) DeleteFileSystem(fsID int) error {
	const functionName = "DeleteFileSystem"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/filesystems", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s - ReadAll -error %w", functionName, err)
	}
	var responseObject DeleteFileSystemResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return nil
}

func (iboxClient *IboxClient) UpdateFileSystem(fsID int, fs FileSystem) (*FileSystem, error) {
	const functionName = "UpdateFileSystem"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/filesystems/", fsID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "fs ID", fsID)

	jsonBytes, err := json.Marshal(fs)
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
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject UpdateFileSystemResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s- fs ID '%d' not found", functionName, fsID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) CreateFileSystemSnapshot(snapshotParam FileSystemSnapshot) (*FileSystemSnapshotResponse, error) {
	const functionName = "CreateFileSystemSnapshot"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/filesystems")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "snapshotParam", snapshotParam)

	jsonBytes, err := json.Marshal(snapshotParam)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateFileSystemSnapshotResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "FileSystem response", responseObject.Result)
	return &responseObject.Result, nil
}
