/*
 *
 * Copyright © 2020-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

const (
	metricsURL = "metrics"
	mirrorURL  = "volume_mirror_transfer_rate_cma_view"
	limit      = 1
)

func (c *ClientIMPL) metricsRequest(ctx context.Context, response interface{}, entity string, entityID string, interval MetricsIntervalEnum) error {
	_, err := c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:   "POST",
			Endpoint: metricsURL,
			Action:   "generate",
			Body: &MetricsRequest{
				Entity:   entity,
				EntityID: entityID,
				Interval: string(interval),
			},
		},
		response)
	if err != nil {
		err = WrapErr(err)
	}
	return err
}

// mirrorTransferRate - Volume Mirror Transfer Rate
func (c *ClientIMPL) mirrorTransferRate(ctx context.Context, response interface{}, entityID string, limit int) error {
	qp := getFSDefaultQueryParams(c)
	qp.RawArg("id", fmt.Sprintf("eq.%s", entityID))
	qp.Limit(limit)
	qp.RawArg("order", "timestamp.desc")
	qp.RawArg("select", "id,timestamp,synchronization_bandwidth,mirror_bandwidth,data_remaining")

	customHeader := http.Header{}
	customHeader.Add("DELL-VISIBILITY", "Internal")
	apiClient := c.APIClient()
	apiClient.SetCustomHTTPHeaders(customHeader)

	_, err := c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "GET",
			Endpoint:    mirrorURL,
			QueryParams: qp,
		},
		response)
	if err != nil {
		err = WrapErr(err)
	}
	customHeader.Del("DELL-VISIBILITY")
	apiClient.SetCustomHTTPHeaders(customHeader)

	return err
}

// PerformanceMetricsByAppliance - Appliance performance metrics
func (c *ClientIMPL) PerformanceMetricsByAppliance(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByApplianceResponse, error) {
	var resp []PerformanceMetricsByApplianceResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_appliance", entityID, interval)
	return resp, err
}

// PerformanceMetricsByNode - Node performance metrics
func (c *ClientIMPL) PerformanceMetricsByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNodeResponse, error) {
	var resp []PerformanceMetricsByNodeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsByVolume - Volume performance metrics
func (c *ClientIMPL) PerformanceMetricsByVolume(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByVolumeResponse, error) {
	var resp []PerformanceMetricsByVolumeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_volume", entityID, interval)
	return resp, err
}

// VolumeMirrorTransferRate - Volume Mirror Transfer Rate
func (c *ClientIMPL) VolumeMirrorTransferRate(ctx context.Context, entityID string) ([]VolumeMirrorTransferRateResponse, error) {
	var resp []VolumeMirrorTransferRateResponse
	err := c.mirrorTransferRate(ctx, &resp, entityID, limit)
	return resp, err
}

// PerformanceMetricsByCluster - Cluster performance metrics
func (c *ClientIMPL) PerformanceMetricsByCluster(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByClusterResponse, error) {
	var resp []PerformanceMetricsByClusterResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_cluster", entityID, interval)
	return resp, err
}

// PerformanceMetricsByVM - Virtual Machine performance metrics
func (c *ClientIMPL) PerformanceMetricsByVM(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByVMResponse, error) {
	var resp []PerformanceMetricsByVMResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_vm", entityID, interval)
	return resp, err
}

// PerformanceMetricsByVg - Storage performance metrics for all volumes in a volume group
func (c *ClientIMPL) PerformanceMetricsByVg(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByVgResponse, error) {
	var resp []PerformanceMetricsByVgResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_vg", entityID, interval)
	return resp, err
}

// PerformanceMetricsByFeFcPort - Frontend fibre channel port performance metrics
func (c *ClientIMPL) PerformanceMetricsByFeFcPort(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeFcPortResponse, error) {
	var resp []PerformanceMetricsByFeFcPortResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_fe_fc_port", entityID, interval)
	return resp, err
}

// PerformanceMetricsByFeEthPort - Frontend ethernet port performance metrics
func (c *ClientIMPL) PerformanceMetricsByFeEthPort(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeEthPortResponse, error) {
	var resp []PerformanceMetricsByFeEthPortResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_fe_eth_port", entityID, interval)
	return resp, err
}

// PerformanceMetricsByFeEthNode - Frontend ethernet performance metrics for node
func (c *ClientIMPL) PerformanceMetricsByFeEthNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeEthNodeResponse, error) {
	var resp []PerformanceMetricsByFeEthNodeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_fe_eth_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsByFeFcNode - Frontend fibre channel performance metrics for node
func (c *ClientIMPL) PerformanceMetricsByFeFcNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFeFcNodeResponse, error) {
	var resp []PerformanceMetricsByFeFcNodeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_fe_fc_node", entityID, interval)
	return resp, err
}

// WearMetricsByDrive returns the Drive wear metrics
func (c *ClientIMPL) WearMetricsByDrive(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]WearMetricsByDriveResponse, error) {
	var resp []WearMetricsByDriveResponse
	err := c.metricsRequest(ctx, &resp, "wear_metrics_by_drive", entityID, interval)
	return resp, err
}

// SpaceMetricsByCluster returns the Cluster space metrics
func (c *ClientIMPL) SpaceMetricsByCluster(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByClusterResponse, error) {
	var resp []SpaceMetricsByClusterResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_cluster", entityID, interval)
	return resp, err
}

// SpaceMetricsByAppliance returns the  Appliance space metrics
func (c *ClientIMPL) SpaceMetricsByAppliance(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByApplianceResponse, error) {
	var resp []SpaceMetricsByApplianceResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_appliance", entityID, interval)
	return resp, err
}

// SpaceMetricsByVolume returns the Volume space metrics
func (c *ClientIMPL) SpaceMetricsByVolume(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVolumeResponse, error) {
	var resp []SpaceMetricsByVolumeResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_volume", entityID, interval)
	return resp, err
}

// SpaceMetricsByVolumeFamily returns the Volume family space metrics
func (c *ClientIMPL) SpaceMetricsByVolumeFamily(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVolumeFamilyResponse, error) {
	var resp []SpaceMetricsByVolumeFamilyResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_volume_family", entityID, interval)
	return resp, err
}

// SpaceMetricsByVM returns the Virtual Machine space metrics
func (c *ClientIMPL) SpaceMetricsByVM(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVMResponse, error) {
	var resp []SpaceMetricsByVMResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_vm", entityID, interval)
	return resp, err
}

// SpaceMetricsByStorageContainer returns the Storage Container space metrics
func (c *ClientIMPL) SpaceMetricsByStorageContainer(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByStorageContainerResponse, error) {
	var resp []SpaceMetricsByStorageContainerResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_storage_container", entityID, interval)
	return resp, err
}

// SpaceMetricsByVolumeGroup returns the Volume space metrics in a volume group
func (c *ClientIMPL) SpaceMetricsByVolumeGroup(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]SpaceMetricsByVolumeGroupResponse, error) {
	var resp []SpaceMetricsByVolumeGroupResponse
	err := c.metricsRequest(ctx, &resp, "space_metrics_by_vg", entityID, interval)
	return resp, err
}

// CopyMetricsByAppliance returns the Appliance copy metrics
func (c *ClientIMPL) CopyMetricsByAppliance(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByApplianceResponse, error) {
	var resp []CopyMetricsByApplianceResponse
	err := c.metricsRequest(ctx, &resp, "copy_metrics_by_appliance", entityID, interval)
	return resp, err
}

// CopyMetricsByCluster returns the Cluster copy metrics
func (c *ClientIMPL) CopyMetricsByCluster(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByClusterResponse, error) {
	var resp []CopyMetricsByClusterResponse
	err := c.metricsRequest(ctx, &resp, "copy_metrics_by_cluster", entityID, interval)
	return resp, err
}

// CopyMetricsByVolumeGroup returns the Copy metrics for each volume group
func (c *ClientIMPL) CopyMetricsByVolumeGroup(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByVolumeGroupResponse, error) {
	var resp []CopyMetricsByVolumeGroupResponse
	err := c.metricsRequest(ctx, &resp, "copy_metrics_by_vg", entityID, interval)
	return resp, err
}

// CopyMetricsByRemoteSystem returns the Copy metrics for each remote system
func (c *ClientIMPL) CopyMetricsByRemoteSystem(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByRemoteSystemResponse, error) {
	var resp []CopyMetricsByRemoteSystemResponse
	err := c.metricsRequest(ctx, &resp, "copy_metrics_by_remote_system", entityID, interval)
	return resp, err
}

// CopyMetricsByVolume returns the Copy metrics for each remote system
func (c *ClientIMPL) CopyMetricsByVolume(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]CopyMetricsByVolumeResponse, error) {
	var resp []CopyMetricsByVolumeResponse
	err := c.metricsRequest(ctx, &resp, "copy_metrics_by_volume", entityID, interval)
	return resp, err
}

// PerformanceMetricsByFileSystem - Performance metrics for the file system
func (c *ClientIMPL) PerformanceMetricsByFileSystem(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByFileSystemResponse, error) {
	var resp []PerformanceMetricsByFileSystemResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_file_system", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmbByNode - Performance metrics for the SMB protocol global
func (c *ClientIMPL) PerformanceMetricsSmbByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbNodeResponse, error) {
	var resp []PerformanceMetricsBySmbNodeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_by_smb_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmbBuiltinclientByNode - Performance metrics for the SMB protocol built-in client
func (c *ClientIMPL) PerformanceMetricsSmbBuiltinclientByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbClientResponse, error) {
	var resp []PerformanceMetricsBySmbClientResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_smb_builtinclient_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmbBranchCacheByNode - Performance metrics for the SMB protocol Branch-Cache
func (c *ClientIMPL) PerformanceMetricsSmbBranchCacheByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbCacheResponse, error) {
	var resp []PerformanceMetricsBySmbCacheResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_smb_branch_cache_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmb1ByNode - Performance metrics for the SMB1 protocol basic
func (c *ClientIMPL) PerformanceMetricsSmb1ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV1NodeResponse, error) {
	var resp []PerformanceMetricsBySmbV1NodeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_smb1_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmb1BuiltinclientByNode - Performance metrics for the SMB1 protocol built-in client
func (c *ClientIMPL) PerformanceMetricsSmb1BuiltinclientByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV1BuiltinClientResponse, error) {
	var resp []PerformanceMetricsBySmbV1BuiltinClientResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_smb1_builtinclient_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmb2ByNode - Performance metrics for the SMB2 protocol basic
func (c *ClientIMPL) PerformanceMetricsSmb2ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV2NodeResponse, error) {
	var resp []PerformanceMetricsBySmbV2NodeResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_smb2_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsSmb2BuiltinclientByNode - Performance metrics for the SMB2 protocol built-in client
func (c *ClientIMPL) PerformanceMetricsSmb2BuiltinclientByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsBySmbV2BuiltinClientResponse, error) {
	var resp []PerformanceMetricsBySmbV2BuiltinClientResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_smb2_builtinclient_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsNfsByNode - Performance metrics for the NFS protocol
func (c *ClientIMPL) PerformanceMetricsNfsByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNfsResponse, error) {
	var resp []PerformanceMetricsByNfsResponse
	err := c.metricsRequest(ctx, &resp, "performance_metrics_nfs_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsNfsv3ByNode - Performance metrics for the NFSv3 protocol
func (c *ClientIMPL) PerformanceMetricsNfsv3ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNfsv3Response, error) {
	var resp []PerformanceMetricsByNfsv3Response
	err := c.metricsRequest(ctx, &resp, "performance_metrics_nfsv3_by_node", entityID, interval)
	return resp, err
}

// PerformanceMetricsNfsv4ByNode - Performance metrics for the NFSv4 protocol
func (c *ClientIMPL) PerformanceMetricsNfsv4ByNode(ctx context.Context, entityID string, interval MetricsIntervalEnum) ([]PerformanceMetricsByNfsv4Response, error) {
	var resp []PerformanceMetricsByNfsv4Response
	err := c.metricsRequest(ctx, &resp, "performance_metrics_nfsv4_by_node", entityID, interval)
	return resp, err
}

// GetCapacity return capacity of first appliance
func (c *ClientIMPL) GetCapacity(ctx context.Context) (int64, error) {
	var resp []ApplianceMetrics
	qp := c.APIClient().QueryParams().Select("physical_total", "physical_used")
	_, err := c.APIClient().Query(
		ctx,
		RequestConfig{
			Method:      "POST",
			Endpoint:    metricsURL,
			Action:      "generate",
			QueryParams: qp,
			Body: &MetricsRequest{
				Entity:   "space_metrics_by_cluster",
				EntityID: "0",
				Interval: "Five_Mins",
			},
		},
		&resp)
	err = WrapErr(err)
	if err != nil {
		return 0, err
	}
	if len(resp) == 0 {
		return 0, errors.New("can't get space metrics by cluster")
	}
	// Latest information is present in last entry of the response
	lastEntry := len(resp) - 1
	freeSpace := resp[lastEntry].PhysicalTotal - resp[lastEntry].PhysicalUsed
	if freeSpace < 0 {
		return 0, nil
	}
	return freeSpace, nil
}
