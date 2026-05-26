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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

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

func (client *IboxClient) GetFileSystemsByPool(ctx context.Context, poolID int, fsPrefix string) (results []FileSystem, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/filesystems")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "pool ID", poolID, "fsprefix", fsPrefix)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return results, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add("pool_id", strconv.Itoa(poolID))
		values.Add("name", "like:"+fsPrefix)
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return results, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetFileSystemsByPoolResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		if responseObject.Error.Code != "" {
			return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (client *IboxClient) GetFileSystemByID(ctx context.Context, fsID int) (fs *FileSystem, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/filesystems", fsID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "filesystem ID", fsID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var responseObject GetFileSystemByIDResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) CreateFileSystem(ctx context.Context, req CreateFileSystemRequest) (*FileSystem, error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/filesystems")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateFileSystemResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	slog.Log(ctx, common.LevelTrace, "info", "FileSystem ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (client *IboxClient) GetFileSystemByName(ctx context.Context, name string) (*FileSystem, error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/filesystems")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "name", name)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	page := 1
	slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add("name", name)
	values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
	values.Add(PARAMETER_PAGE, strconv.Itoa(page))
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var responseObject GetFileSystemByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if len(responseObject.Result) == 0 {
		return nil, ErrNotFound
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("iboxAPI error - errorCode: %s url: %s", responseObject.Error.Code, url)
	}
	return &responseObject.Result[0], nil
}

func (client *IboxClient) GetFileSystemsByParentID(ctx context.Context, parentID int) (results []FileSystem, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/filesystems")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "parent ID", parentID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return results, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add("parent_id", strconv.Itoa(parentID))
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return results, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetFileSystemsByParentIDResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}

		if responseObject.Error.Code != "" {
			return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (client *IboxClient) DeleteFileSystem(ctx context.Context, fsID int) error {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/filesystems", fsID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return common.Errorf("readAll -error: %w url: %s", err, url)
	}
	var responseObject DeleteFileSystemResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return nil
}

func (client *IboxClient) UpdateFileSystem(ctx context.Context, fsID int, fs FileSystem) (*FileSystem, error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/filesystems/", fsID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "fs ID", fsID)

	jsonBytes, err := json.Marshal(fs)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	SetAuthHeader(request, client.Creds)

	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject UpdateFileSystemResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == FILESYSTEM_NOT_FOUND {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}

func (client *IboxClient) CreateFileSystemSnapshot(ctx context.Context, snapshot FileSystemSnapshot) (*FileSystemSnapshotResponse, error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/filesystems")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "snapshotParam", snapshot)

	jsonBytes, err := json.Marshal(snapshot)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	request.URL.RawQuery = values.Encode()

	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateFileSystemSnapshotResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	slog.Log(ctx, common.LevelTrace, "info", "FileSystem response", responseObject.Result)
	return &responseObject.Result, nil
}
