package iboxapi

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

type ConsistencyGroupInfo struct {
	CreatedBySnapshotPolicyID   int      `json:"created_by_snapshot_policy_id,omitempty"`
	UpdateAt                    int      `json:"updated_at,omitempty"`
	SnapshotRetention           int      `json:"snapshot_retention,omitempty"`
	CreatedBySnapshotPolicyName string   `json:"created_by_snapshot_policy_name,omitempty"`
	MembersCount                int      `json:"members_count,omitempty"`
	SnapshotPolicyName          string   `json:"snapshot_policy_name,omitempty"`
	ID                          int      `json:"id,omitempty"`
	ParentID                    int      `json:"parent_id,omitempty"`
	LockState                   string   `json:"lock_state,omitempty"`
	CreatedByScheduleID         int      `json:"created_by_schedule_id,omitempty"`
	CGType                      string   `json:"type,omitempty"`
	PoolName                    string   `json:"pool_name,omitempty"`
	ReplicationTypes            []string `json:"replication_types,omitempty"`
	HasChildren                 bool     `json:"has_children,omitempty"`
	LockExpiresAt               int      `json:"lock_expires_at,omitempty"`
	CreatedByScheduleName       string   `json:"created_by_schedule_name,omitempty"`
	RmrSnapshotGUID             string   `json:"rmr_snapshot_guid,omitempty"`
	Name                        string   `json:"name,omitempty"`
	TenantID                    int      `json:"tenant_id,omitempty"`
	CreatedAt                   int      `json:"created_at,omitempty"`
	SnapshotExpiresAt           int      `json:"snapshot_expires_at,omitempty"`
	PoolID                      int      `json:"pool_id,omitempty"`
	SnapshotPolicyID            int      `json:"snapshot_policy_id,omitempty"`
	IsReplicated                bool     `json:"is_replicated,omitempty"`
}

type MemberInfo struct {
	MemberType                          string   `json:"type,omitempty"`
	Depth                               int      `json:"depth,omitempty"`
	ID                                  int      `json:"id,omitempty"`
	Name                                string   `json:"name,omitempty"`
	CreatedAt                           int      `json:"created_at,omitempty"`
	UpdateAt                            int      `json:"updated_at,omitempty"`
	Mapped                              bool     `json:"mapped,omitempty"`
	WriteProtected                      bool     `json:"write_protected,omitempty"`
	Size                                int      `json:"size,omitempty"`
	ProvType                            string   `json:"provtype,omitempty"`
	SSDEnabled                          bool     `json:"ssd_enabled,omitempty"`
	SSAExpressEnabled                   bool     `json:"ssa_express_enabled,omitempty"`
	SSAExpressStatus                    string   `json:"ssa_express_status,omitempty"`
	CompressionEnabled                  bool     `json:"compression_enabled,omitempty"`
	Serial                              string   `json:"serial,omitempty"`
	RmrTarget                           bool     `json:"rmr_target,omitempty"`
	RMRSource                           bool     `json:"rmr_source,omitempty"`
	RMRActiveActivePeer                 bool     `json:"rmr_active_active_peer,omitempty"`
	MobilitySource                      string   `json:"mobility_source,omitempty"`
	RMRSnapshotGUID                     string   `json:"rmr_snapshot_guid,omitempty"`
	DataSnapshotGUID                    string   `json:"data_snapshot_guid,omitempty"`
	MgmtSnapshotGUID                    string   `json:"mgmt_snapshot_guid,omitempty"`
	CGSnapshotGUID                      string   `json:"_cg_snapshot_guid,omitempty"`
	CGGUID                              string   `json:"_cg_guid,omitempty"`
	FamilyID                            int      `json:"family_id,omitempty"`
	LockExpiresAt                       int      `json:"lock_expires_at,omitempty"`
	ReclaimedSnapshotRemoteSystemSerial string   `json:"_reclaimed_snapshot_remote_system_serial,omitempty"`
	SnapshotRetention                   string   `json:"snapshot_retention,omitempty"`
	DatasetType                         string   `json:"dataset_type,omitempty"`
	Used                                int      `json:"used,omitempty"`
	TreeAllocated                       int      `json:"tree_allocated,omitempty"`
	Allocated                           int      `json:"allocated,omitempty"`
	CompressionSuppressed               bool     `json:"compression_suppressed,omitempty"`
	CapacitySaving                      int      `json:"capacity_savings,omitempty"`
	UDID                                int      `json:"udid,omitempty"`
	PathsAvailable                      bool     `json:"paths_available,omitempty"`
	SourceReplicatedSGID                int      `json:"source_replicated_sg_id,omitempty"`
	PoolID                              int      `json:"pool_id,omitempty"`
	ParentID                            int      `json:"parent_id,omitempty"`
	CGName                              string   `json:"cg_name,omitempty"`
	CGID                                int      `json:"cg_id,omitempty"`
	SnapshotPolicyID                    int      `json:"snapshot_policy_id,omitempty"`
	HasChildren                         bool     `json:"has_children,omitempty"`
	SnapshotExpiresAt                   int      `json:"snapshot_expires_at,omitempty"`
	CreatedBySnapshotPolicyID           int      `json:"created_by_snapshot_policy_id,omitempty"`
	CreatedByScheduleID                 int      `json:"created_by_schedule_id,omitempty"`
	TenantID                            int      `json:"tenant_id,omitempty"`
	QOSPolicyName                       string   `json:"qos_policy_name,omitempty"`
	PoolName                            string   `json:"pool_name,omitempty"`
	NGUID                               string   `json:"nguid,omitempty"`
	ReplicaIDs                          []int    `json:"replica_ids,omitempty"`
	ReplicationTypes                    []string `json:"replication_types,omitempty"`
	NumBlocks                           int      `json:"num_blocks,omitempty"`
	QOSPolicyID                         int      `json:"qos_policy_id,omitempty"`
	QOSSharedPolicyID                   int      `json:"qos_shared_policy_id,omitempty"`
	QOSSharedPolicyName                 string   `json:"qos_shared_policy_name,omitempty"`
	LockState                           string   `json:"lock_state,omitempty"`
	SnapshotPolicyName                  string   `json:"snapshot_policy_name,omitempty"`
	CreatedBySnapshotPolicyName         string   `json:"created_by_snapshot_policy_name,omitempty"`
	CreatedByScheduleName               string   `json:"created_by_schedule_name,omitempty"`
}

type CreateSnapshotGroupRequest struct {
	CGID       int    `json:"parent_id"`
	SnapName   string `json:"name"`
	SnapPrefix string `json:"snap_prefix"`
	SnapSuffix string `json:"snap_suffix"`
}

type CreateSnapshotGroupResponse struct {
	Metadata Metadata             `json:"metadata"`
	Result   ConsistencyGroupInfo `json:"result"`
	Error    Error                `json:"error"`
}
type GetConsistencyGroupResponse struct {
	Metadata Metadata             `json:"metadata"`
	Result   ConsistencyGroupInfo `json:"result"`
	Error    Error                `json:"error"`
}

type DeleteConsistencyGroupResponse struct {
	Metadata Metadata             `json:"metadata"`
	Result   ConsistencyGroupInfo `json:"result"`
	Error    Error                `json:"error"`
}
type AddMemberToSnapshotGroupRequest struct {
	DatasetID int `json:"dataset_id"`
}
type AddMemberToSnapshotGroupResponse struct {
	Metadata Metadata             `json:"metadata"`
	Result   ConsistencyGroupInfo `json:"result"`
	Error    Error                `json:"error"`
}

type CreateConsistencyGroupRequest struct {
	Name   string `json:"name"`
	PoolID int    `json:"pool_id"`
}

type CreateConsistencyGroupResponse struct {
	Metadata Metadata             `json:"metadata"`
	Result   ConsistencyGroupInfo `json:"result"`
	Error    Error                `json:"error"`
}
type GetMembersByCGIDResponse struct {
	Metadata Metadata     `json:"metadata"`
	Result   []MemberInfo `json:"result"`
	Error    Error        `json:"error"`
}
type GetConsistencyGroupByNameResponse struct {
	Metadata Metadata               `json:"metadata"`
	Result   []ConsistencyGroupInfo `json:"result"`
	Error    Error                  `json:"error"`
}

const (
	REPLICATE_TO_ASYNC_TARGET = "replicate_to_async_target"
	DELETE_MEMBERS            = "delete_members"
)

func (client *IboxClient) CreateConsistencyGroup(ctx context.Context, req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/cgs")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return nil, common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := request.URL.Query()
	values.Add(REPLICATE_TO_ASYNC_TARGET, PARAMETER_VALUE_FALSE)
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

	var responseObject CreateConsistencyGroupResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	slog.Log(ctx, common.LevelTrace, "info", "Export ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (client *IboxClient) AddMemberToSnapshotGroup(ctx context.Context, volumeID, cgID int) error {
	url := fmt.Sprintf("%s%s/%s/members", client.Creds.URL, "api/rest/cgs", strconv.Itoa(cgID))
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "volume ID", volumeID, "cg ID", cgID)

	req := AddMemberToSnapshotGroupRequest{
		DatasetID: volumeID,
	}
	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject AddMemberToSnapshotGroupResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	slog.Log(ctx, common.LevelTrace, "info", "response", responseObject.Result)
	return nil
}

func (client *IboxClient) GetMembersByCGID(ctx context.Context, cgID int) (memberInfo []MemberInfo, err error) {
	url := fmt.Sprintf("%s%s/%d/members", client.Creds.URL, "api/rest/cgs", cgID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "cg ID", cgID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return memberInfo, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return memberInfo, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return memberInfo, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetMembersByCGIDResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return memberInfo, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		memberInfo = append(memberInfo, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return memberInfo, nil
}

func (client *IboxClient) CreateSnapshotGroup(ctx context.Context, req CreateSnapshotGroupRequest) (newCG *ConsistencyGroupInfo, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/cgs")
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
		return nil, common.Errorf("do - error %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject CreateSnapshotGroupResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	slog.Log(ctx, common.LevelTrace, "info", "Export ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (client *IboxClient) GetConsistencyGroupByName(ctx context.Context, name string) (cg *ConsistencyGroupInfo, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/cgs")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "cg name", name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add("name", name)
	values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(common.IBOXDefaultQueryPageSize))
	values.Add(PARAMETER_PAGE, strconv.Itoa(1))
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
	var responseObject GetConsistencyGroupByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if len(responseObject.Result) == 0 {
		return nil, ErrNotFound
	}
	cg = &responseObject.Result[0]

	return cg, nil
}

func (client *IboxClient) DeleteConsistencyGroup(ctx context.Context, cgID int) (err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/cgs", cgID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "cg ID", cgID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	values.Add(DELETE_MEMBERS, PARAMETER_VALUE_TRUE)
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
		return common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var responseObject DeleteConsistencyGroupResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "CG_NOT_FOUND" {
			return common.Errorf("iboxAPI errorCode: %s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return nil
}
func (client *IboxClient) GetConsistencyGroup(ctx context.Context, cgID int) (cg *ConsistencyGroupInfo, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/cgs", cgID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "cg ID", cgID)

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
	var responseObject GetConsistencyGroupResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "CG_NOT_FOUND" {
			return nil, common.Errorf("%s - error: %w url: %s", responseObject.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return &responseObject.Result, nil
}
