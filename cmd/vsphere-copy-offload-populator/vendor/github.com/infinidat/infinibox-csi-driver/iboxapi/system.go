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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type NtpStatus struct {
	NodeID             int   `json:"node_id"`
	LastProbeTimestamp int64 `json:"last_probe_timestamp"`
	NtpPeers           []struct {
		TallyCode          string  `json:"tally_code"`
		Remote             string  `json:"remote"`
		Refid              string  `json:"refid"`
		Stratum            int     `json:"stratum"`
		Type               string  `json:"type"`
		WhenSeconds        int     `json:"when_seconds"`
		PollSeconds        int     `json:"poll_seconds"`
		Reach              int     `json:"reach"`
		DelayMilliseconds  float64 `json:"delay_milliseconds"`
		OffsetMilliseconds float64 `json:"offset_milliseconds"`
		JitterMilliseconds float64 `json:"jitter_milliseconds"`
	} `json:"ntp_peers"`
}

type Capacity struct {
	AllocatedPhysicalSpaceWithinPools int64   `json:"allocated_physical_space_within_pools"`
	AllocatedVirtualSpaceWithinPools  int64   `json:"allocated_virtual_space_within_pools"`
	DataReductionRatio                float64 `json:"data_reduction_ratio"`
	DynamicSpareDriveCost             int     `json:"dynamic_spare_drive_cost"`
	FreePhysicalSpace                 int64   `json:"free_physical_space"`
	FreeVirtualSpace                  int64   `json:"free_virtual_space"`
	TotalAllocatedPhysicalSpace       int64   `json:"total_allocated_physical_space"`
	TotalCapacitySavings              int64   `json:"total_capacity_savings"`
	TotalDiskUsageWithinPools         int64   `json:"total_disk_usage_within_pools"`
	TotalPhysicalCapacity             int64   `json:"total_physical_capacity"`
	TotalSpareBytes                   int64   `json:"total_spare_bytes"`
	TotalSparePartitions              int     `json:"total_spare_partitions"`
	TotalThickCapacitySavings         int     `json:"total_thick_capacity_savings"`
	TotalThinCapacitySavings          int64   `json:"total_thin_capacity_savings"`
	TotalVirtualCapacity              int64   `json:"total_virtual_capacity"`
	UsedDynamicSpareBytes             int     `json:"used_dynamic_spare_bytes"`
	UsedDynamicSparePartitions        int     `json:"used_dynamic_spare_partitions"`
	UsedSpareBytes                    int64   `json:"used_spare_bytes"`
	UsedSparePartitions               int     `json:"used_spare_partitions"`
}
type EntityCounts struct {
	Clusters              int `json:"clusters"`
	ConsistencyGroups     int `json:"consistency_groups"`
	FilesystemSnapshots   int `json:"filesystem_snapshots"`
	Filesystems           int `json:"filesystems"`
	FilesystemsUnix       int `json:"filesystems_unix"`
	FilesystemsWindows    int `json:"filesystems_windows"`
	Hosts                 int `json:"hosts"`
	MappedVolumes         int `json:"mapped_volumes"`
	Nfs3Exports           int `json:"nfs3_exports"`
	Nfs3ExportsWinFs      int `json:"nfs3_exports_win_fs"`
	Pools                 int `json:"pools"`
	Replicas              int `json:"replicas"`
	ReplicationGroups     int `json:"replication_groups"`
	RgReplicas            int `json:"rg_replicas"`
	SmbShares             int `json:"smb_shares"`
	SmbSharesUnixFs       int `json:"smb_shares_unix_fs"`
	SnapshotGroups        int `json:"snapshot_groups"`
	SsaExpressFilesystems int `json:"ssa_express_filesystems"`
	SsaExpressVolumes     int `json:"ssa_express_volumes"`
	StandardPools         int `json:"standard_pools"`
	VolumeSnapshots       int `json:"volume_snapshots"`
	Volumes               int `json:"volumes"`
	VvolPools             int `json:"vvol_pools"`
}
type BbuChargeLevel struct {
	Bbu1 int `json:"bbu-1"`
	Bbu2 int `json:"bbu-2"`
	Bbu3 int `json:"bbu-3"`
}
type NodeBbuProtection struct {
	Node1 string `json:"node-1"`
	Node2 string `json:"node-2"`
	Node3 string `json:"node-3"`
}
type HealthState struct {
	ActiveCacheSsdDevices            int               `json:"active_cache_ssd_devices"`
	ActiveDrives                     int               `json:"active_drives"`
	ActiveEncryptedCacheSsdDevices   int               `json:"active_encrypted_cache_ssd_devices"`
	ActiveEncryptedDrives            int               `json:"active_encrypted_drives"`
	BbuAggregateChargePercent        int               `json:"bbu_aggregate_charge_percent"`
	BbuChargeLevel                   BbuChargeLevel    `json:"bbu_charge_level"`
	BbuProtectedNodes                int               `json:"bbu_protected_nodes"`
	EnclosureFailureSafeDistribution bool              `json:"enclosure_failure_safe_distribution"`
	EncryptionEnabled                bool              `json:"encryption_enabled"`
	FailedDrives                     int               `json:"failed_drives"`
	InactiveNodes                    int               `json:"inactive_nodes"`
	MissingDrives                    int               `json:"missing_drives"`
	NodeBbuProtection                NodeBbuProtection `json:"node_bbu_protection"`
	PhasingOutDrives                 int               `json:"phasing_out_drives"`
	RaidGroupsPendingRebuild1        int               `json:"raid_groups_pending_rebuild_1"`
	RaidGroupsPendingRebuild2        int               `json:"raid_groups_pending_rebuild_2"`
	ReadyDrives                      int               `json:"ready_drives"`
	Rebuild1Inprogress               bool              `json:"rebuild_1_inprogress"`
	Rebuild2Inprogress               bool              `json:"rebuild_2_inprogress"`
	TestingDrives                    int               `json:"testing_drives"`
	UnknownDrives                    int               `json:"unknown_drives"`
}
type Localtime struct {
	UtcTime int64 `json:"utc_time"`
}
type OperationalState struct {
	Description    string `json:"description"`
	InitState      any    `json:"init_state"`
	Mode           string `json:"mode"`
	ReadOnlySystem bool   `json:"read_only_system"`
	State          string `json:"state"`
}
type Gui struct {
	BuildMode any    `json:"build_mode"`
	Revision  string `json:"revision"`
	Version   string `json:"version"`
}
type Infinishell struct {
	BuildMode any    `json:"build_mode"`
	Revision  string `json:"revision"`
	Version   string `json:"version"`
}
type System struct {
	BuildMode string `json:"build_mode"`
	Revision  string `json:"revision"`
	Version   string `json:"version"`
}
type Release struct {
	Gui         Gui         `json:"gui"`
	Infinishell Infinishell `json:"infinishell"`
	System      System      `json:"system"`
}
type FipsBestPractice struct {
	CertificateStrength             int    `json:"certificate_strength"`
	IsCertificateStrengthSufficient bool   `json:"is_certificate_strength_sufficient"`
	IsHTTPRedirection               bool   `json:"is_http_redirection"`
	IsLdapConnectionsSecured        bool   `json:"is_ldap_connections_secured"`
	IsLocalUsersDisabled            bool   `json:"is_local_users_disabled"`
	LocalUsersPasswordHash          string `json:"local_users_password_hash"`
	NumUsersPasswordHashNotSecured  int    `json:"num_users_password_hash_not_secured"`
}
type Security struct {
	EncryptionEnabled     bool             `json:"encryption_enabled"`
	FipsBestPractice      FipsBestPractice `json:"fips_best_practice"`
	KmipConnectivityState string           `json:"kmip_connectivity_state"`
	SystemSecurityState   string           `json:"system_security_state"`
}
type SsaExpressInfo struct {
	FreeSsaExpressCapacity  int    `json:"free_ssa_express_capacity"`
	SsaExpressStatus        string `json:"ssa_express_status"`
	TotalSsaExpressCapacity int    `json:"total_ssa_express_capacity"`
	UsedSsaExpressCapacity  int    `json:"used_ssa_express_capacity"`
}
type SystemDetails struct {
	Capacity               Capacity         `json:"capacity"`
	DeploymentID           string           `json:"deployment_id"`
	EntityCounts           EntityCounts     `json:"entity_counts"`
	FullModel              string           `json:"full_model"`
	HealthState            HealthState      `json:"health_state"`
	InstallTimestamp       int64            `json:"install_timestamp"`
	Localtime              Localtime        `json:"localtime"`
	Model                  string           `json:"model"`
	Name                   string           `json:"name"`
	OperationalState       OperationalState `json:"operational_state"`
	ProductID              string           `json:"product_id"`
	Release                Release          `json:"release"`
	Security               Security         `json:"security"`
	SerialNumber           int              `json:"serial_number"`
	SsaExpressInfo         SsaExpressInfo   `json:"ssa_express_info"`
	SystemPowerConsumption float64          `json:"system_power_consumption"`
	UpgradeTimestamp       int64            `json:"upgrade_timestamp"`
	Uptime                 int64            `json:"uptime"`
	Version                string           `json:"version"`
	Wwnn                   string           `json:"wwnn"`
}

type GetSystemResponse struct {
	Metadata Metadata      `json:"metadata"`
	Result   SystemDetails `json:"result"`
	Error    Error         `json:"error"`
}

type GetNtpStatusResponse struct {
	Metadata Metadata    `json:"metadata"`
	Result   []NtpStatus `json:"result"`
	Error    Error       `json:"error"`
}

func (iboxClient *IboxClient) GetSystem() (system *SystemDetails, err error) {
	const functionName = "GetSystem"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/system")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url)

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
	var responseObject GetSystemResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	if responseObject.Error.Code != "" {
		// TODO check for NOT FOUND ?  return ErrNotFound for callers?
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return &responseObject.Result, nil
}

func (iboxClient *IboxClient) GetNtpStatus() (results []NtpStatus, err error) {
	const functionName = "GetNtpStatus"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/system/ntp_status")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url)

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
				iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
		}
		var responseObject GetNtpStatusResponse
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
