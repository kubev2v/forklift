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

type Replica struct {
	ID                             int    `json:"id"`
	Version                        int    `json:"_version"`
	Description                    string `json:"description"`
	ReplicaConfigurationVersion    string `json:"_replica_configuration_version"`
	ManagementConfigurationVersion string `json:"_management_configuration_version"`
	LocalLinkGUID                  string `json:"_local_link_guid"`
	RemoteReplicaID                int    `json:"remote_replica_id"`
	Role                           string `json:"role"`
	IsInternalSource               any    `json:"_is_internal_source"`
	ConcurrentReplica              bool   `json:"concurrent_replica"`
	IncludingSnapshots             bool   `json:"including_snapshots"`
	SnapshotsRetention             any    `json:"snapshots_retention"`
	RemoteSnapshotSuffix           string `json:"remote_snapshot_suffix"`
	LockRemoteSnapshotRetention    any    `json:"lock_remote_snapshot_retention"`
	EntityType                     string `json:"entity_type"`
	State                          string `json:"state"`
	StateDescription               any    `json:"state_description"`
	StateReason                    any    `json:"state_reason"`
	JobState                       string `json:"job_state"`
	Jobs                           []struct {
		ID        int    `json:"id"`
		State     string `json:"state"`
		IsInitial bool   `json:"is_initial"`
		Type      string `json:"type"`
		StartTime int64  `json:"start_time"`
		EndTime   any    `json:"end_time"`
	} `json:"jobs"`
	IsInitial                     bool   `json:"is_initial"`
	PendingJobCount               int    `json:"pending_job_count"`
	NextJobStartTime              any    `json:"next_job_start_time"`
	Enabled                       bool   `json:"_enabled"`
	ReservedForInfinisafe         bool   `json:"reserved_for_infinisafe"`
	SyncInterval                  int    `json:"sync_interval"`
	RpoType                       string `json:"rpo_type"`
	RpoValue                      int    `json:"rpo_value"`
	RpoState                      string `json:"rpo_state"`
	Throughput                    int    `json:"throughput"`
	TemporaryFailureRetryInterval int    `json:"temporary_failure_retry_interval"`
	TemporaryFailureRetryCount    int    `json:"temporary_failure_retry_count"`
	PermanentFailureWaitInterval  int    `json:"permanent_failure_wait_interval"`
	BaseAction                    string `json:"base_action"`
	LocalEntityID                 int    `json:"local_entity_id"`
	RemoteEntityID                int    `json:"remote_entity_id"`
	LocalEntityName               string `json:"local_entity_name"`
	RemoteEntityName              string `json:"remote_entity_name"`
	LocalCgName                   string `json:"local_cg_name"`
	RemoteCgID                    int    `json:"remote_cg_id"`
	RemoteCgName                  string `json:"remote_cg_name"`
	RemotePoolID                  int    `json:"remote_pool_id"`
	LocalPoolID                   int    `json:"local_pool_id"`
	LocalPoolName                 string `json:"local_pool_name"`
	RemotePoolName                string `json:"remote_pool_name"`
	CreatedAt                     int64  `json:"created_at"`
	UpdatedAt                     int64  `json:"updated_at"`
	EntityPairs                   []struct {
		ID             int `json:"id"`
		RemotePairID   int `json:"remote_pair_id"`
		RemoteEntityID int `json:"remote_entity_id"`
		LocalEntity    struct {
			DatasetType                         string   `json:"dataset_type"`
			Type                                string   `json:"type"`
			Depth                               int      `json:"depth"`
			ID                                  int      `json:"id"`
			Name                                string   `json:"name"`
			CreatedAt                           int64    `json:"created_at"`
			UpdatedAt                           int64    `json:"updated_at"`
			Mapped                              bool     `json:"mapped"`
			WriteProtected                      bool     `json:"write_protected"`
			Size                                int      `json:"size"`
			Provtype                            string   `json:"provtype"`
			SsdEnabled                          bool     `json:"ssd_enabled"`
			SsaExpressEnabled                   bool     `json:"ssa_express_enabled"`
			SsaExpressStatus                    any      `json:"ssa_express_status"`
			CompressionEnabled                  bool     `json:"compression_enabled"`
			Serial                              string   `json:"serial"`
			RmrTarget                           bool     `json:"rmr_target"`
			RmrSource                           bool     `json:"rmr_source"`
			RmrActiveActivePeer                 bool     `json:"rmr_active_active_peer"`
			MobilitySource                      any      `json:"mobility_source"`
			RmrSnapshotGUID                     any      `json:"rmr_snapshot_guid"`
			DataSnapshotGUID                    any      `json:"data_snapshot_guid"`
			MgmtSnapshotGUID                    any      `json:"mgmt_snapshot_guid"`
			CgSnapshotGUID                      any      `json:"_cg_snapshot_guid"`
			CgGUID                              any      `json:"_cg_guid"`
			FamilyID                            int      `json:"family_id"`
			LockExpiresAt                       any      `json:"lock_expires_at"`
			ReclaimedSnapshotRemoteSystemSerial any      `json:"_reclaimed_snapshot_remote_system_serial"`
			SnapshotRetention                   any      `json:"snapshot_retention"`
			Used                                any      `json:"used"`
			TreeAllocated                       any      `json:"tree_allocated"`
			Allocated                           any      `json:"allocated"`
			CompressionSuppressed               any      `json:"compression_suppressed"`
			CapacitySavings                     any      `json:"capacity_savings"`
			CapacitySavingsPerEntity            any      `json:"capacity_savings_per_entity"`
			DiskUsage                           any      `json:"disk_usage"`
			DataReductionRatio                  float64  `json:"data_reduction_ratio"`
			Udid                                any      `json:"udid"`
			PathsAvailable                      any      `json:"paths_available"`
			SourceReplicatedSgID                any      `json:"source_replicated_sg_id"`
			PoolID                              int      `json:"pool_id"`
			ParentID                            int      `json:"parent_id"`
			CgName                              string   `json:"cg_name"`
			CgID                                int      `json:"cg_id"`
			HasChildren                         bool     `json:"has_children"`
			SnapshotPolicyID                    any      `json:"snapshot_policy_id"`
			SnapshotExpiresAt                   any      `json:"snapshot_expires_at"`
			CreatedBySnapshotPolicyID           any      `json:"created_by_snapshot_policy_id"`
			CreatedByScheduleID                 any      `json:"created_by_schedule_id"`
			TenantID                            int      `json:"tenant_id"`
			PoolName                            string   `json:"pool_name"`
			Nguid                               string   `json:"nguid"`
			ReplicaIDs                          []any    `json:"replica_ids"`
			ReplicationTypes                    []string `json:"replication_types"`
			NumBlocks                           int      `json:"num_blocks"`
			QosPolicyID                         any      `json:"qos_policy_id"`
			QosPolicyName                       any      `json:"qos_policy_name"`
			QosSharedPolicyID                   any      `json:"qos_shared_policy_id"`
			QosSharedPolicyName                 any      `json:"qos_shared_policy_name"`
			LockState                           string   `json:"lock_state"`
			SnapshotPolicyName                  any      `json:"snapshot_policy_name"`
			CreatedBySnapshotPolicyName         any      `json:"created_by_snapshot_policy_name"`
			CreatedByScheduleName               any      `json:"created_by_schedule_name"`
		} `json:"local_entity"`
		TargetOldRoState          bool   `json:"_target_old_ro_state"`
		RemoteBaseEntityID        any    `json:"remote_base_entity_id"`
		RemoteBaseAction          string `json:"remote_base_action"`
		RemoteBaseDiffableID      any    `json:"_remote_base_diffable_id"`
		LocalBaseEntityID         any    `json:"local_base_entity_id"`
		LocalBaseAction           string `json:"local_base_action"`
		LocalBaseDiffableID       any    `json:"_local_base_diffable_id"`
		RemoteEntityName          string `json:"remote_entity_name"`
		LocalEntityName           string `json:"local_entity_name"`
		ConsistentGUID            any    `json:"_consistent_guid"`
		CgConsistentGUID          any    `json:"_cg_consistent_guid"`
		CgGUID                    any    `json:"_cg_guid"`
		NextConsistentGUID        string `json:"_next_consistent_guid"`
		Progress                  int    `json:"progress"`
		LastSynchronized          any    `json:"last_synchronized"`
		RestorePoint              int    `json:"restore_point"`
		Duration                  int    `json:"duration"`
		IsInitial                 bool   `json:"is_initial"`
		SyncJobCommitted          bool   `json:"_sync_job_committed"`
		LocalReclaimedSnapshotID  any    `json:"_local_reclaimed_snapshot_id"`
		RemoteReclaimedSnapshotID any    `json:"_remote_reclaimed_snapshot_id"`
		ReclaimedSnapshotGUID     any    `json:"_reclaimed_snapshot_guid"`
		ReclaimedSnapshotTime     any    `json:"_reclaimed_snapshot_time"`
		SessionSnapshotsID        int    `json:"_session_snapshots_id"`
		ReportEntityID            any    `json:"_report_entity_id"`
		ReportSystemSerialNumber  any    `json:"_report_system_serial_number"`
		ActiveActivePortBit       any    `json:"_active_active_port_bit"`
		LocalPairGUID             any    `json:"_local_pair_guid"`
		ReplicaID                 int    `json:"replica_id"`
		LocalEntityID             int    `json:"local_entity_id"`
	} `json:"entity_pairs"`
	RemoteIPAddresses        any    `json:"_remote_ip_addresses"`
	StartedAt                int64  `json:"started_at"`
	SyncDuration             int    `json:"sync_duration"`
	LastSynchronized         any    `json:"last_synchronized"`
	Progress                 int    `json:"progress"`
	RestorePoint             any    `json:"restore_point"`
	ConsistentGUID           any    `json:"_consistent_guid"`
	NextRestorePoint         any    `json:"next_restore_point"`
	NextConsistentGUID       string `json:"_next_consistent_guid"`
	SnapshotGUID             any    `json:"_snapshot_guid"`
	SnapshotTime             any    `json:"_snapshot_time"`
	LocalReclaimedSgID       any    `json:"_local_reclaimed_sg_id"`
	RemoteReclaimedSgID      any    `json:"_remote_reclaimed_sg_id"`
	StagingAreaAllocatedSize any    `json:"staging_area_allocated_size"`
	AssignedLocalIPIndex     int    `json:"_assigned_local_ip_index"`
	ReplicationType          string `json:"replication_type"`
	Domino                   any    `json:"domino"`
	SyncState                any    `json:"sync_state"`
	AsyncMode                any    `json:"async_mode"`
	Latency                  any    `json:"latency"`
	MobilitySource           any    `json:"mobility_source"`
	IsPreferred              any    `json:"is_preferred,omitempty"`
	SuspendedFromLocal       any    `json:"suspended_from_local"`
	LinkID                   int    `json:"link_id"`
	AssignedRemoteIP         string `json:"_assigned_remote_ip"`
	AssignedSyncRemoteIps    any    `json:"_assigned_sync_remote_ips"`
	LocalCgID                int    `json:"local_cg_id"`
}

// is_preferred is applicable when ACTIVE_ACTIVE, it is a pointer to a bool
// to allow for the field to not be marshalled when the pointer is nil which
// we want when not specifying ACTIVE_ACTIVE (e.g. when we specify ASYNC)
// sync_interval and rpo_value is applicable when ASYNC, not ACTIVE_ACTIVE
type CreateReplicaRequest struct {
	IsPreferred      *bool  `json:"is_preferred,omitempty"`
	SyncInterval     int    `json:"sync_interval,omitempty"`
	Description      string `json:"description"`
	EntityType       string `json:"entity_type"`
	RemoteEntityName string `json:"remote_entity_name,omitempty"`
	LocalEntityID    int    `json:"local_entity_id"`
	ReplicationType  string `json:"replication_type"`
	BaseAction       string `json:"base_action"`
	LinkID           int    `json:"link_id"`
	RpoValue         int    `json:"rpo_value,omitempty"`
	RemotePoolID     int    `json:"remote_pool_id"`
}

type GetReplicaResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Replica  `json:"result"`
	Error    Error    `json:"error"`
}
type CreateReplicaResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Replica  `json:"result"`
	Error    Error    `json:"error"`
}
type DeleteReplicaResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   Replica  `json:"result"`
	Error    Error    `json:"error"`
}
type GetReplicasResponse struct {
	Metadata Metadata  `json:"metadata"`
	Result   []Replica `json:"result"`
	Error    Error     `json:"error"`
}

func (client *IboxClient) CreateReplica(ctx context.Context, req CreateReplicaRequest) (*Replica, error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/replicas")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "request", req)

	slog.Debug("info", "is_preferred", req.IsPreferred)

	jsonBytes, err := json.Marshal(req)
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

	var responseObject CreateReplicaResponse
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

func (client *IboxClient) GetReplicas(ctx context.Context) (results []Replica, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/replicas")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return results, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
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
		var responseObject GetReplicasResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, common.Errorf("unmarshal - error: %w url; %s", err, url)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}

func (client *IboxClient) DeleteReplica(ctx context.Context, replicaID int) (err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/replicas", replicaID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "replica ID", replicaID)

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
	var response DeleteReplicaResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	if response.Error.Code != "" {
		if response.Error.Code == "REPLICA_NOT_FOUND" {
			return common.Errorf("errorCode: %s - error: %w url: %s", response.Error.Code, ErrNotFound, url)
		}

		return common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}
	return nil
}

func (client *IboxClient) GetReplica(ctx context.Context, replicaID int) (*Replica, error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/replicas", replicaID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "replica ID", replicaID)

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
	var response GetReplicaResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if response.Error.Code != "" {
		if response.Error.Code == "REPLICA_NOT_FOUND" {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", response.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}
	return &response.Result, nil
}
