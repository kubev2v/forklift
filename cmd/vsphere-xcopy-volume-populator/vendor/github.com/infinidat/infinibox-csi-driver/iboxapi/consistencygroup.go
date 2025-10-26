package iboxapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func (iboxClient *IboxClient) CreateConsistencyGroup(req CreateConsistencyGroupRequest) (*ConsistencyGroupInfo, error) {
	const functionName = "CreateConsistencyGroup"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/cgs")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "request", req)

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := request.URL.Query()
	values.Add(REPLICATE_TO_ASYNC_TARGET, PARAMETER_VALUE_FALSE)
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

	var responseObject CreateConsistencyGroupResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "Export ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) AddMemberToSnapshotGroup(volumeID, cgID int) error {
	const functionName = "AddMemberToSnapshotGroup"

	url := fmt.Sprintf("%s%s/%s/members", iboxClient.Creds.URL, "api/rest/cgs", strconv.Itoa(cgID))
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "volume ID", volumeID, "cg ID", cgID)

	req := AddMemberToSnapshotGroupRequest{
		DatasetID: volumeID,
	}
	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, _ := io.ReadAll(response.Body)

	var responseObject AddMemberToSnapshotGroupResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "response", responseObject.Result)
	return nil
}

func (iboxClient *IboxClient) GetMembersByCGID(cgID int) (memberInfo []MemberInfo, err error) {
	const functionName = "GetMembersByCGID"

	url := fmt.Sprintf("%s%s/%d/members", iboxClient.Creds.URL, "api/rest/cgs", cgID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "cg ID", cgID)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "page", page, "totalPages", totalPages)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return memberInfo, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, iboxClient.Creds)

		resp, err := iboxClient.HTTPClient.Do(req)
		if err != nil {
			return memberInfo, fmt.Errorf("%s - Do - error %w", functionName, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return memberInfo, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetMembersByCGIDResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return memberInfo, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
		}
		memberInfo = append(memberInfo, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return memberInfo, nil
}

func (iboxClient *IboxClient) CreateSnapshotGroup(req CreateSnapshotGroupRequest) (newCG *ConsistencyGroupInfo, err error) {
	const functionName = "CreateSnapshotGroup"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/cgs")
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

	var responseObject CreateSnapshotGroupResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "Export ID", responseObject.Result.ID)
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetConsistencyGroupByName(name string) (cg *ConsistencyGroupInfo, err error) {
	const functionName = "GetConsistencyGroupByName"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/cgs")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "cg name", name)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add("name", name)
	values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(common.IBOXDefaultQueryPageSize))
	values.Add(PARAMETER_PAGE, strconv.Itoa(1))
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
	var responseObject GetConsistencyGroupByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if len(responseObject.Result) == 0 {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - cg name '%s' not found", functionName, name)}
	}
	cg = &responseObject.Result[0]

	return cg, nil
}

func (iboxClient *IboxClient) DeleteConsistencyGroup(cgID int) (err error) {
	const functionName = "DeleteConsistencyGroup"

	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/cgs", cgID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "cg ID", cgID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	values := req.URL.Query()
	values.Add(PARAMETER_APPROVED, PARAMETER_VALUE_TRUE)
	values.Add(DELETE_MEMBERS, PARAMETER_VALUE_TRUE)
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
	var responseObject DeleteConsistencyGroupResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		// TODO check for NOT FOUND?  have callers check for ErrNotFound?
		return fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return nil
}
func (iboxClient *IboxClient) GetConsistencyGroup(cgID int) (cg *ConsistencyGroupInfo, err error) {
	const functionName = "GetConsistencyGroup"

	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/cgs", cgID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "cg ID", cgID)

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
	var responseObject GetConsistencyGroupResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		if responseObject.Error.Code == "CG_NOT_FOUND" {
			return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - cg ID '%d' not found", functionName, cgID)}
		}
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}
