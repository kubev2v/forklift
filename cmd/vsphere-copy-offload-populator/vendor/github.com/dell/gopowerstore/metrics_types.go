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

import "github.com/go-openapi/strfmt"

// CopySessionTypeEnum is a copy session type
type CopySessionTypeEnum string

/*
Intervals of which metrics can be provided

	Enum:
		[ Twenty_Sec, Five_Mins, One_Hour, One_Day ]
*/
type MetricsIntervalEnum string

const (
	// TwentySec is an interval used when retrieving metrics
	TwentySec MetricsIntervalEnum = "Twenty_Sec"
	// FiveMins is an interval used when retrieving metrics
	FiveMins MetricsIntervalEnum = "Five_Mins"
	// OneHour is an interval used when retrieving metrics
	OneHour MetricsIntervalEnum = "One_Hour"
	// OneDay is an interval used when retrieving metrics
	OneDay MetricsIntervalEnum = "One_Day"
)

// MetricsRequest parameters to make metrics request
type MetricsRequest struct {
	Entity   string `json:"entity"`
	EntityID string `json:"entity_id"`
	Interval string `json:"interval"`
}

// ApplianceMetrics is returned by space_metrics_by_appliance metrics request
type ApplianceMetrics struct {
	// Unique identifier of the appliance.
	ApplianceID string `json:"appliance_id"`
	// Total amount of space
	PhysicalTotal int64 `json:"physical_total"`
	// Amount of space currently used
	PhysicalUsed int64 `json:"physical_used"`
}

// CommonMetricsFields contains fields common across all metrics responses
type CommonMetricsFields struct {
	Entity string `json:"entity,omitempty"`
	// Number of times the metrics is repeated.
	// Maximum: 2.147483647e+09
	// Minimum: 0
	RepeatCount *int32 `json:"repeat_count,omitempty"`

	// End of sample period.
	// Format: date-time
	Timestamp strfmt.DateTime `json:"timestamp,omitempty"`
}

// CommonMaxAvgIopsBandwidthFields contains common fiels for max/avg I/O, latency, size, and bandwith fields for metrics responses
type CommonMaxAvgIopsBandwidthFields struct {
	// Maximum average size of input and output operations in bytes.
	MaxAvgIoSize float32 `json:"max_avg_io_size,omitempty"`

	// Maximum of average latency in microseconds.
	MaxAvgLatency float32 `json:"max_avg_latency,omitempty"`

	// Maximum read latency in microseconds.
	MaxAvgReadLatency float32 `json:"max_avg_read_latency,omitempty"`

	// Maximum of average read size in bytes.
	MaxAvgReadSize float32 `json:"max_avg_read_size,omitempty"`

	// Maximum of average write latency in microseconds.
	MaxAvgWriteLatency float32 `json:"max_avg_write_latency,omitempty"`

	// Maximum of average write size in bytes.
	MaxAvgWriteSize float32 `json:"max_avg_write_size,omitempty"`

	// Maximum read bandwidth in bytes per second.
	MaxReadBandwidth float32 `json:"max_read_bandwidth,omitempty"`

	// Maximum reads per second.
	MaxReadIops float32 `json:"max_read_iops,omitempty"`

	// Maximum total bandwidth in bytes per second.
	MaxTotalBandwidth float32 `json:"max_total_bandwidth,omitempty"`

	// Maximum totals per second.
	MaxTotalIops float64 `json:"max_total_iops,omitempty"`

	// Maximum write bandwidth in bytes per second.
	MaxWriteBandwidth float32 `json:"max_write_bandwidth,omitempty"`

	// Maximum writes per second.
	MaxWriteIops float32 `json:"max_write_iops,omitempty"`

	// Read rate in bytes per second.
	ReadBandwidth float32 `json:"read_bandwidth,omitempty"`

	// Total read operations per second.
	ReadIops float32 `json:"read_iops,omitempty"`

	// Total data transfer rate in bytes per second.
	TotalBandwidth float32 `json:"total_bandwidth,omitempty"`

	// Total read and write operations per second.
	TotalIops float64 `json:"total_iops,omitempty"`

	// Write rate in byte/sec.
	WriteBandwidth float32 `json:"write_bandwidth,omitempty"`

	// Total write operations per second.
	WriteIops float32 `json:"write_iops,omitempty"`
}

// CommonSMBFields contains common fields for SMB metrics responses
type CommonSMBFields struct {
	// Average calls.
	AvgCalls float32 `json:"avg_calls,omitempty"`

	// Average read and write operations per second.
	AvgIops float32 `json:"avg_iops,omitempty"`

	// Average read and write size in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Average read operations per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Average write operations per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// Maximum of average read and write latency in microseconds.
	MaxAvgLatency float32 `json:"max_avg_latency,omitempty"`

	// Maximum of average read latency in microseconds.
	MaxAvgReadLatency float32 `json:"max_avg_read_latency,omitempty"`

	// Maximum of average read size in bytes.
	MaxAvgReadSize float32 `json:"max_avg_read_size,omitempty"`

	// Maximum of average read and write size in bytes.
	MaxAvgSize float32 `json:"max_avg_size,omitempty"`

	// Maximum of average write latency in microseconds.
	MaxAvgWriteLatency float32 `json:"max_avg_write_latency,omitempty"`

	// Maximum of average write size in bytes.
	MaxAvgWriteSize float32 `json:"max_avg_write_size,omitempty"`

	// Maximum calls.
	MaxCalls float32 `json:"max_calls,omitempty"`

	// Maximum read and write operations per second.
	MaxIops float32 `json:"max_iops,omitempty"`

	// Maximum read operations per second.
	MaxReadIops float32 `json:"max_read_iops,omitempty"`

	// Maximum write operations per second.
	MaxWriteIops float32 `json:"max_write_iops,omitempty"`

	// Unique identifier of the node.
	NodeID string `json:"node_id,omitempty"`

	// Total read operations per second.
	ReadIops float32 `json:"read_iops,omitempty"`

	// Total calls.
	TotalCalls float32 `json:"total_calls,omitempty"`

	// Total read and write operations per second.
	TotalIops float32 `json:"total_iops,omitempty"`

	// Total write operations per second.
	WriteIops float32 `json:"write_iops,omitempty"`
}

// CommonUnalignedFields contains common unaligned fields from metrics responses
type CommonUnalignedFields struct {
	// Average unaligned read/write rate in bytes per second.
	AvgUnalignedBandwidth float32 `json:"avg_unaligned_bandwidth,omitempty"`

	// Average unaligned total input/output per second.
	AvgUnalignedIops float32 `json:"avg_unaligned_iops,omitempty"`

	// Average unaligned read rate in bytes per second.
	AvgUnalignedReadBandwidth float32 `json:"avg_unaligned_read_bandwidth,omitempty"`

	// Average unaligned read input/output per second.
	AvgUnalignedReadIops float32 `json:"avg_unaligned_read_iops,omitempty"`

	// Average unaligned write rate in bytes per second.
	AvgUnalignedWriteBandwidth float32 `json:"avg_unaligned_write_bandwidth,omitempty"`

	// Average unaligned write input/output per second.
	AvgUnalignedWriteIops float32 `json:"avg_unaligned_write_iops,omitempty"`

	// Maximum unaligned read/write rate in bytes per second.
	MaxUnalignedBandwidth float32 `json:"max_unaligned_bandwidth,omitempty"`

	// Maximum unaligned total input/output per second.
	MaxUnalignedIops float32 `json:"max_unaligned_iops,omitempty"`

	// Maximum unaligned read rate in bytes per second.
	MaxUnalignedReadBandwidth float32 `json:"max_unaligned_read_bandwidth,omitempty"`

	// Maximum unaligned read input/output per second.
	MaxUnalignedReadIops float32 `json:"max_unaligned_read_iops,omitempty"`

	// Maximum unaligned write rate in bytes per second.
	MaxUnalignedWriteBandwidth float32 `json:"max_unaligned_write_bandwidth,omitempty"`

	// Maximum unaligned write input/output per second.
	MaxUnalignedWriteIops float32 `json:"max_unaligned_write_iops,omitempty"`

	// Unaligned read/write rate in bytes per second.
	UnalignedBandwidth float32 `json:"unaligned_bandwidth,omitempty"`

	// Unaligned total input/output per second.
	UnalignedIops float32 `json:"unaligned_iops,omitempty"`

	// Unaligned read rate in bytes per second.
	UnalignedReadBandwidth float32 `json:"unaligned_read_bandwidth,omitempty"`

	// Unaligned read input/output per second.
	UnalignedReadIops float32 `json:"unaligned_read_iops,omitempty"`

	// Unaligned write rate in bytes per second.
	UnalignedWriteBandwidth float32 `json:"unaligned_write_bandwidth,omitempty"`

	// Unaligned write input/output per second.
	UnalignedWriteIops float32 `json:"unaligned_write_iops,omitempty"`
}

// CommonEthPortFields contains fields common across all ethernet port metrics responses
type CommonEthPortFields struct {
	// The average total bytes received per second.
	AvgBytesRxPs float32 `json:"avg_bytes_rx_ps,omitempty"`

	// The average total bytes transmitted per second.
	AvgBytesTxPs float32 `json:"avg_bytes_tx_ps,omitempty"`

	// The average number of packets received with CRC error (and thus dropped) per second.
	AvgPktRxCrcErrorPs float32 `json:"avg_pkt_rx_crc_error_ps,omitempty"`

	// The average number of packets discarded per second due to lack of buffer space.
	AvgPktRxNoBufferErrorPs float32 `json:"avg_pkt_rx_no_buffer_error_ps,omitempty"`

	// The average number of packets received per second.
	AvgPktRxPs float32 `json:"avg_pkt_rx_ps,omitempty"`

	// The average number of packets that failed to be transmitted per second due to error.
	AvgPktTxErrorPs float32 `json:"avg_pkt_tx_error_ps,omitempty"`

	// The average number of packets transmitted per second.
	AvgPktTxPs float32 `json:"avg_pkt_tx_ps,omitempty"`

	// The total bytes received per second.
	BytesRxPs float32 `json:"bytes_rx_ps,omitempty"`

	// The total bytes transmitted per second.
	BytesTxPs float32 `json:"bytes_tx_ps,omitempty"`

	// The maximum total bytes received per second.
	MaxBytesRxPs float32 `json:"max_bytes_rx_ps,omitempty"`

	// The maximum total bytes transmitted per second.
	MaxBytesTxPs float32 `json:"max_bytes_tx_ps,omitempty"`

	// The maximum number of packets received with CRC error (and thus dropped) per second.
	MaxPktRxCrcErrorPs float32 `json:"max_pkt_rx_crc_error_ps,omitempty"`

	// The maximum number of packets discarded per second due to lack of buffer space.
	MaxPktRxNoBufferErrorPs float32 `json:"max_pkt_rx_no_buffer_error_ps,omitempty"`

	// The maximum number of packets received per second.
	MaxPktRxPs float32 `json:"max_pkt_rx_ps,omitempty"`

	// The maximum number of packets that failed to be transmitted per second due to error.
	MaxPktTxErrorPs float32 `json:"max_pkt_tx_error_ps,omitempty"`

	// The maximum number of packets transmitted per second.
	MaxPktTxPs float32 `json:"max_pkt_tx_ps,omitempty"`

	// Reference to the associated node on which these metrics were recorded.
	NodeID string `json:"node_id,omitempty"`

	// The number of packets received with CRC error (and thus dropped) per second.
	PktRxCrcErrorPs float32 `json:"pkt_rx_crc_error_ps,omitempty"`

	// The number of packets discarded per second due to lack of buffer space.
	PktRxNoBufferErrorPs float32 `json:"pkt_rx_no_buffer_error_ps,omitempty"`

	// The number of packets received per second.
	PktRxPs float32 `json:"pkt_rx_ps,omitempty"`

	// The number of packets that failed to be transmitted per second due to error.
	PktTxErrorPs float32 `json:"pkt_tx_error_ps,omitempty"`

	// The number of packets transmitted per second.
	PktTxPs float32 `json:"pkt_tx_ps,omitempty"`
}

// CommonNfsv34ResponseFields contains common fields from Nfs v3/4 metrics responses
type CommonNfsv34ResponseFields struct {
	// Average failed operations per second.
	AvgFailedMdOps float32 `json:"avg_failed_md_ops,omitempty"`

	// Average md latency operations per second.
	AvgMdLatency float32 `json:"avg_md_latency,omitempty"`

	// Average md operations per second.
	AvgMdOps float32 `json:"avg_md_ops,omitempty"`

	// Average read operations per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read and write operations per second.
	AvgTotalIops float32 `json:"avg_total_iops,omitempty"`

	// Average write operations per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Total failed md operations per second.
	FailedMdOps float32 `json:"failed_md_ops,omitempty"`

	// Maximum average md latency per second.
	MaxAvgMdLatency float32 `json:"max_avg_md_latency,omitempty"`

	// Max failed operations per second.
	MaxFailedMdOps float32 `json:"max_failed_md_ops,omitempty"`

	// Maximum read operations per second.
	MaxReadIops float32 `json:"max_read_iops,omitempty"`

	// Maximum read and write operations per second.
	MaxTotalIops float32 `json:"max_total_iops,omitempty"`

	// Maximum write operations per second.
	MaxWriteIops float32 `json:"max_write_iops,omitempty"`

	// Total md operations per second.
	MdOps float32 `json:"md_ops,omitempty"`

	// Unique identifier of the nfs.
	NodeID string `json:"node_id,omitempty"`

	// Total read iops in microseconds.
	ReadIops float32 `json:"read_iops,omitempty"`

	// Total read and write iops in microseconds.
	TotalIops float32 `json:"total_iops,omitempty"`

	// Total write iops in microseconds.
	WriteIops float32 `json:"write_iops,omitempty"`
}

// CopyMetricsCommonFields is the filed common to all copy metrics
type CopyMetricsCommonFields struct {
	// Number of bytes remaining to be copied at the end of this sampling period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	DataRemaining *int64 `json:"data_remaining,omitempty"`

	// Number of bytes transferred during this sampling period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	DataTransferred *int64 `json:"data_transferred,omitempty"`

	// Time (in milliseconds) spent doing reads during this sampling period.
	//
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	ReadTime *int64 `json:"read_time,omitempty"`

	// session type
	SessionType CopySessionTypeEnum `json:"session_type,omitempty"`

	// Localized message string corresponding to session_type
	SessionTypeL10n string `json:"session_type_l10n,omitempty"`

	// Data transfer rate (in bytes/second) computed using data_transferred and transfer_time.
	//
	TransferRate float32 `json:"transfer_rate,omitempty"`

	// The time (in milliseconds) spent in copy activity during this sampling period.
	//
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	TransferTime *int64 `json:"transfer_time,omitempty"`

	// Time (in milliseconds) spent doing writes during this sampling period.
	//
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	WriteTime *int64 `json:"write_time,omitempty"`
}

// WearMetricsByDriveResponse is returned by wear_metrics_by_drive request
type WearMetricsByDriveResponse struct {
	CommonMetricsFields

	// Reference to the associated drive which these metrics were recorded.
	DriveID string `json:"drive_id,omitempty"`

	// The percentage of drive wear remaining.
	PercentEnduranceRemaining float32 `json:"percent_endurance_remaining,omitempty"`
}

// SpaceMetricsByClusterResponse is returned by space_metrics_by_cluster request
type SpaceMetricsByClusterResponse struct {
	CommonMetricsFields

	// Identifier of the cluster.
	ClusterID string `json:"cluster_id,omitempty"`

	// This metric represents total amount of physical space user data occupies after deduplication and compression.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	DataPhysicalUsed *int64 `json:"data_physical_used,omitempty"`

	// Ratio of the logical used space to data physical used space which is after deduplication and compression.
	DataReduction float32 `json:"data_reduction,omitempty"`

	// The overall efficiency is computed as a ratio of the total space provisioned to physical used space. For example, ten 2 GB volumes were provisioned and 1 GB of data is written to each of them.
	// Each of the volumes has one snapshot as well, for another ten 2 GB volumes. All volumes are thinly provisioned with deduplication and compression applied, there is 4 GB of physical space used.
	// Overall efficiency would be (20 * 2 GB) / 4 GB or 10:1. The efficiency_ratio value will be 10 in this example.
	EfficiencyRatio float32 `json:"efficiency_ratio,omitempty"`

	// Total configured size of all storage ojects within the cluster. This metric includes all primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Amount of data in bytes written to all storage objects within the cluster, without any deduplication and/or compression. This metric includes all primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// The total combined space on the physical drives of the cluster available for data.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	PhysicalTotal *int64 `json:"physical_total,omitempty"`

	// The total physical space consumed in the cluster, accounting for all efficiency mechanisms, as well as all data protection.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	PhysicalUsed *int64 `json:"physical_used,omitempty"`

	// Cluster shared logical used is sum of appliances' shared logical used in the cluster.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	SharedLogicalUsed *int64 `json:"shared_logical_used,omitempty"`

	// Ratio of the amount of space that would have been used by snapshots if space efficiency was not applied to logical space used solely by snapshots.
	// For example, an object is provisioned as 1 GB and it has two snapshots.
	// Each snapshot has 200 MB of data. Snapshot savings will be (1 GB + 1 GB) / (0.2 GB + 0.2 GB) or 5:1. The snapshot_savings value will be 5 in this case.
	SnapshotSavings float32 `json:"snapshot_savings,omitempty"`

	// Ratio of all the vVol provisioned to data they contain. This is the ratio of logical_provisioned to logical_used.
	// For example, a cluster has two 2 GB objects and have written 500 MB bytes of data to them.
	// he thin savings would be (2 * 2 GB) / (2 * 0.5 GB) or 4:1, so the thin_savings value would be 4.0.
	ThinSavings float32 `json:"thin_savings,omitempty"`

	// Last physical used space for data during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastDataPhysicalUsed *int64 `json:"last_data_physical_used,omitempty"`

	// Last data reduction space during the period.
	LastDataReduction float32 `json:"last_data_reduction,omitempty"`

	// Last efficiency ratio during the period.
	LastEfficiencyRatio float32 `json:"last_efficiency_ratio,omitempty"`

	// Last logical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last physical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastPhysicalTotal *int64 `json:"last_physical_total,omitempty"`

	// Last physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastPhysicalUsed *int64 `json:"last_physical_used,omitempty"`

	// Last shared logical used during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastSharedLogicalUsed *int64 `json:"last_shared_logical_used,omitempty"`

	// Last snapshot savings space during the period.
	LastSnapshotSavings float32 `json:"last_snapshot_savings,omitempty"`

	// Last thin savings ratio during the period.
	LastThinSavings float32 `json:"last_thin_savings,omitempty"`

	// Maximum physical used space for data during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxDataPhysicalUsed *int64 `json:"max_data_physical_used,omitempty"`

	// Maximum data reduction space during the period.
	MaxDataReduction float32 `json:"max_data_reduction,omitempty"`

	// Maximum efficiency ratio during the period.
	MaxEfficiencyRatio float32 `json:"max_efficiency_ratio,omitempty"`

	// Maximum logical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Maximum logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Maximum physical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxPhysicalTotal *int64 `json:"max_physical_total,omitempty"`

	// Maximum physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxPhysicalUsed *int64 `json:"max_physical_used,omitempty"`

	// Maximum shared logical used during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxSharedLogicalUsed *int64 `json:"max_shared_logical_used,omitempty"`

	// Maximum snapshot savings space during the period.
	MaxSnapshotSavings float32 `json:"max_snapshot_savings,omitempty"`

	// Maximum thin savings ratio during the period.
	MaxThinSavings float32 `json:"max_thin_savings,omitempty"`
}

// SpaceMetricsByApplianceResponse is returned by space_metrics_by_appliance  request
type SpaceMetricsByApplianceResponse struct {
	CommonMetricsFields

	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	// This metric represents amount of physical space user data occupies after deduplication and compression.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	DataPhysicalUsed *int64 `json:"data_physical_used,omitempty"`

	// Ratio of the logical used space to data physical used space which is after deduplication and compression.
	DataReduction float32 `json:"data_reduction,omitempty"`

	// The overall efficiency is computed as a ratio of the total space provisioned to physical used space.
	// For example, ten 2 GB volumes were provisioned and 1 GB of data is written to each of them.
	// Each of the volumes has one snapshot as well, for another ten 2 GB volumes.
	// All volumes are thinly provisioned with deduplication and compression applied, there is 4 GB of physical space used. Overall efficiency would be (20 * 2 GB) / 4 GB or 10:1.
	// The efficiency_ratio value will be 10 in this example.
	EfficiencyRatio float32 `json:"efficiency_ratio,omitempty"`

	// Last physical used space for data during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastDataPhysicalUsed *int64 `json:"last_data_physical_used,omitempty"`

	// Last data reduction space during the period.
	LastDataReduction float32 `json:"last_data_reduction,omitempty"`

	// Last efficiency ratio during the period.
	LastEfficiencyRatio float32 `json:"last_efficiency_ratio,omitempty"`

	// Last logical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last physical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastPhysicalTotal *int64 `json:"last_physical_total,omitempty"`

	// Last physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastPhysicalUsed *int64 `json:"last_physical_used,omitempty"`

	// Last shared logical used during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastSharedLogicalUsed *int64 `json:"last_shared_logical_used,omitempty"`

	// Last snapshot savings space during the period.
	LastSnapshotSavings float32 `json:"last_snapshot_savings,omitempty"`

	// Last thin savings ratio during the period.
	LastThinSavings float32 `json:"last_thin_savings,omitempty"`

	// Total configured size of all storage objects on an appliance. This metric includes all primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Amount of data in bytes written to all storage objects on an appliance, without any deduplication and/or compression. This metric includes all primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// Maximum physical used space for data during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxDataPhysicalUsed *int64 `json:"max_data_physical_used,omitempty"`

	// Maximum data reduction space during the period.
	MaxDataReduction float32 `json:"max_data_reduction,omitempty"`

	// Maximum efficiency ratio during the period.
	MaxEfficiencyRatio float32 `json:"max_efficiency_ratio,omitempty"`

	// Maxiumum logical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Maxiumum logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Maximum physical total space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxPhysicalTotal *int64 `json:"max_physical_total,omitempty"`

	// Maximum physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxPhysicalUsed *int64 `json:"max_physical_used,omitempty"`

	// Max shared logical used during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxSharedLogicalUsed *int64 `json:"max_shared_logical_used,omitempty"`

	// Maximum snapshot savings space during the period.
	MaxSnapshotSavings float32 `json:"max_snapshot_savings,omitempty"`

	// Maximum thin savings ratio during the period.
	MaxThinSavings float32 `json:"max_thin_savings,omitempty"`

	// Total combined space on the physical drives of the appliance available for data.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	PhysicalTotal *int64 `json:"physical_total,omitempty"`

	// Total physical space consumed in the appliance, accounting for all efficiency mechanisms, as well as all data protection.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	PhysicalUsed *int64 `json:"physical_used,omitempty"`

	// Amount of space the volume family needs to hold the data written by host and shared by snaps and fast-clones in the family. This does not include deduplication or compression.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	SharedLogicalUsed *int64 `json:"shared_logical_used,omitempty"`

	// Ratio of the amount of space that would have been used by snapshots if space efficiency was not applied to logical space used solely by snapshots.
	// For example, an object is provisioned as 1 GB and it has two snapshots. Each snapshot has 200 MB of data.
	// Snapshot savings will be (1 GB + 1 GB) / (0.2 GB + 0.2 GB) or 5:1. The snapshot_savings value will be 5 in this case.
	SnapshotSavings float32 `json:"snapshot_savings,omitempty"`

	// Ratio of all the vVol provisioned to data they contain. This is the ratio of logical_provisioned to logical_used.
	// For example, a cluster has two 2 GB objects and have written 500 MB bytes of data to them.
	// The thin savings would be (2 * 2 GB) / (2 * 0.5 GB) or 4:1, so the thin_savings value would be 4.0.
	ThinSavings float32 `json:"thin_savings,omitempty"`
}

// SpaceMetricsByVolumeResponse is returned by  space_metrics_by_volume
type SpaceMetricsByVolumeResponse struct {
	CommonMetricsFields
	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	// Last logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last thin savings ratio during the period.
	LastThinSavings float32 `json:"last_thin_savings,omitempty"`

	// Configured size in bytes of a volume which amount of data can be written to. This metric includes primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Amount of data in bytes host has written to a volume without any deduplication, compression or sharing.
	// This metric includes primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// Max logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Max logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Max thin savings ratio during the period.
	MaxThinSavings float32 `json:"max_thin_savings,omitempty"`

	// Ratio of all the volumes provisioned to data being written to them. For example, an appliance has two 2 GB volumes and have written 500 MB of data to them.
	// The thin savings would be (2 GB * 2) / (0.5 GB * 2) or 4:1, so the thin_savings value would be 4.0.
	ThinSavings float32 `json:"thin_savings,omitempty"`

	// ID of the volume.
	VolumeID string `json:"volume_id,omitempty"`
}

// SpaceMetricsByVolumeFamilyResponse is returned by  space_metrics_by_volume_family
type SpaceMetricsByVolumeFamilyResponse struct {
	CommonMetricsFields
	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	// ID of the family.
	FamilyID string `json:"family_id,omitempty"`

	// Last logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last shared logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastSharedLogicalUsed *int64 `json:"last_shared_logical_used,omitempty"`

	// Last snap and clone logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastSnapCloneLogicalUsed *int64 `json:"last_snap_clone_logical_used,omitempty"`

	// Last snapshot savings space during the period.
	LastSnapshotSavings float32 `json:"last_snapshot_savings,omitempty"`

	// Last unique physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastUniquePhysicalUsed *int64 `json:"last_unique_physical_used,omitempty"`

	// Configured size in bytes of a volume which amount of data can be written to. This metric includes primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Amount of data in bytes host has written to a volume family without any deduplication, compression or sharing. This metric includes primaries, snaps and clones.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// Max logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Max logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Max shared logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxSharedLogicalUsed *int64 `json:"max_shared_logical_used,omitempty"`

	// Max snap and clone logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxSnapCloneLogicalUsed *int64 `json:"max_snap_clone_logical_used,omitempty"`

	// Max snapshot savings space during the period.
	MaxSnapshotSavings float32 `json:"max_snapshot_savings,omitempty"`

	// Max unique physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxUniquePhysicalUsed *int64 `json:"max_unique_physical_used,omitempty"`

	// Amount of space the volume family needs to hold the data written by host and shared by snaps and fast-clones in the family. This does not include deduplication or compression.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	SharedLogicalUsed *int64 `json:"shared_logical_used,omitempty"`

	// Total Amount of data in bytes host has written to all volumes in the volume family without any deduplication, compression or sharing.
	// This metric includes snaps and clones in the volume family.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	SnapCloneLogicalUsed *int64 `json:"snap_clone_logical_used,omitempty"`

	// Ratio of the amount of space that would have been used by snapshots if space efficiency was not applied to logical space used solely by snapshots.
	// For example, a volume is provisioned as 1 GB bytes and it has two snapshots. Each snapshot has 200 MB of data.
	// Snapshot savings will be (1 GB + 1 GB) / (0.2 GB + 0.2 GB) or 5:1. The snapshot_savings value will be 5 in this case.
	SnapshotSavings float32 `json:"snapshot_savings,omitempty"`

	// Amount of physical space volume family used after compression and deduplication. This is the space to be freed up if a volume family is removed from the appliance.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	UniquePhysicalUsed *int64 `json:"unique_physical_used,omitempty"`
}

// SpaceMetricsByVMResponse is returned by  space_metrics_by_vm
type SpaceMetricsByVMResponse struct {
	CommonMetricsFields

	// Last logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last snap and clone logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastSnapCloneLogicalUsed *int64 `json:"last_snap_clone_logical_used,omitempty"`

	// Last snapshot savings space during the period.
	LastSnapshotSavings float32 `json:"last_snapshot_savings,omitempty"`

	// Last thin savings ratio during the period.
	LastThinSavings float32 `json:"last_thin_savings,omitempty"`

	// Last unique physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastUniquePhysicalUsed *int64 `json:"last_unique_physical_used,omitempty"`

	// Total configured size in bytes of all virtual volumes used by virtual machine.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Total amount of data in bytes written to all virtual volumes used by virtual machine.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// Max logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Max logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Max snap and clone logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxSnapCloneLogicalUsed *int64 `json:"max_snap_clone_logical_used,omitempty"`

	// Max snapshot savings space during the period.
	MaxSnapshotSavings float32 `json:"max_snapshot_savings,omitempty"`

	// Max thin savings ratio during the period.
	MaxThinSavings float32 `json:"max_thin_savings,omitempty"`

	// Max unique physical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxUniquePhysicalUsed *int64 `json:"max_unique_physical_used,omitempty"`

	// Total Amount of data in bytes host has written to all volumes used by virtual machine without any deduplication, compression or sharing.
	// This metric includes snaps and clones in the volume family used by virtual machine.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	SnapCloneLogicalUsed *int64 `json:"snap_clone_logical_used,omitempty"`

	// Ratio of the amount of space that would have been used by snapshots if space efficiency was not applied to logical space used solely by snapshots of vVols used by virtual machine.
	// For example, a vVol is provisioned as 1 GB and it has two snapshots.
	// Each snapshot has 200 MB of data. Snapshot savings will be (1 GB + 1 GB) / (0.2 GB + 0.2 GB) or 5:1. The snapshot_savings value will be 5 in this case.
	SnapshotSavings float32 `json:"snapshot_savings,omitempty"`

	// Ratio of all the vVol provisioned to data they contain. This is the ratio of logical_provisioned to logical_used.
	// For example, a VM has two 2 GB vVol's and have written 500 MB of data to them. The thin savings would be (2 * 2GB) / (2 * 0.5 GB) or 4:1, so the thin_savings value would be 4.0.
	ThinSavings float32 `json:"thin_savings,omitempty"`

	// Amount of physical space virtual machine used after compression and deduplication. This is the space to be freed up if a virtual machine is removed.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	UniquePhysicalUsed *int64 `json:"unique_physical_used,omitempty"`

	// Unique identifier representing a specific virtual machine.
	VMID string `json:"vm_id,omitempty"`
}

// SpaceMetricsByStorageContainerResponse is returned by  space_metrics_by_storage_container
type SpaceMetricsByStorageContainerResponse struct {
	CommonMetricsFields

	// Internal ID of the storage container.
	StorageContainerID string `json:"storage_container_id"`

	// Last logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last snapshot savings during the period.
	LastSnapshotSavings float32 `json:"last_snapshot_savings,omitempty"`

	// Total configured size in bytes of the primary and clone virtual volumes within the storage container.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Amount of data in bytes written to primary and clone virtual volumes within the storage container.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// Maximum logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Maximum logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Maximum snapshot savings during the period.
	MaxSnapshotSavings float32 `json:"max_snapshot_savings,omitempty"`

	// Ratio of the amount of space that would have been used by snapshots if space efficiency was not applied to logical space used solely by snapshots.
	// For example, a volume is provisioned as 1 GB and it has two snapshots. Each snapshot has 200 MB of data.
	// Snapshot savings will be (1 GB + 1 GB) / (0.2 GB + 0.2 GB) or 5:1. The snapshot_savings value will be 5 in this case.
	SnapshotSavings float32 `json:"snapshot_savings,omitempty"`
}

// SpaceMetricsByVolumeGroupResponse is returned by  space_metrics_by_vg
type SpaceMetricsByVolumeGroupResponse struct {
	CommonMetricsFields

	// Last logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalProvisioned *int64 `json:"last_logical_provisioned,omitempty"`

	// Last logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastLogicalUsed *int64 `json:"last_logical_used,omitempty"`

	// Last snap and clone logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LastSnapCloneLogicalUsed *int64 `json:"last_snap_clone_logical_used,omitempty"`

	// Last snapshot savings space during the period.
	LastSnapshotSavings float32 `json:"last_snapshot_savings,omitempty"`

	// Last thin savings ratio during the period.
	LastThinSavings float32 `json:"last_thin_savings,omitempty"`

	// Total configured size in bytes of all member volumes in a volume group.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalProvisioned *int64 `json:"logical_provisioned,omitempty"`

	// Total amount of data in bytes written to all member volumes in a volume group.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	LogicalUsed *int64 `json:"logical_used,omitempty"`

	// Max logical provisioned space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalProvisioned *int64 `json:"max_logical_provisioned,omitempty"`

	// Maximum logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxLogicalUsed *int64 `json:"max_logical_used,omitempty"`

	// Max snap and clone logical used space during the period.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxSnapCloneLogicalUsed *int64 `json:"max_snap_clone_logical_used,omitempty"`

	// Max snapshot savings space during the period.
	MaxSnapshotSavings float32 `json:"max_snapshot_savings,omitempty"`

	// Max thin savings ratio during the period.
	MaxThinSavings float32 `json:"max_thin_savings,omitempty"`

	// Total amount of data in bytes host has written to all volumes in the volume group without any deduplication, compression or sharing.
	// This metric includes used snaps and clones in the volume group.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	SnapCloneLogicalUsed *int64 `json:"snap_clone_logical_used,omitempty"`

	// Ratio of the amount of space that would have been used by snapshots in the volume group if space efficiency was not applied to logical space used solely by snapshots.
	// For example, two volumes are provisioned as 1 GB and each has two snapshots.
	// Each snapshot has 200 MB of data. Snapshot savings will be (1 GB * 2 + 1 GB * 2) / (0.2 GB * 2 + 0.2 GB * 2) or 5:1. The snapshot_savings value will be 5 in this case.
	SnapshotSavings float32 `json:"snapshot_savings,omitempty"`

	// Ratio of all the volumes provisioned to data being written to them. For example, a volume group has two 2 GB volumes and have written 500 MB of data to them.
	// The thin savings would be (2 * 2 GB) / (2 * 0.5 GB) or 4:1, so the thin_savings value would be 4.0.
	ThinSavings float32 `json:"thin_savings,omitempty"`

	// Unique identifier representing a volume group.
	VgID string `json:"vg_id,omitempty"`
}

// PerformanceMetricsByApplianceResponse is returned from performance_metrics_by_appliance
type PerformanceMetricsByApplianceResponse struct {
	CommonMetricsFields

	// Unique identifier representing a specific appliance.
	ApplianceID string `json:"appliance_id,omitempty"`

	// The average percentage of CPU Utilization on the cores dedicated to servicing storage I/O requests. Calculated over time interval across appliance. Derived from an average of utilization across all nodes within the appliance.
	AvgIoWorkloadCPUUtilization float32 `json:"avg_io_workload_cpu_utilization,omitempty"`

	CommonAvgFields

	// The percentage of CPU Utilization on the cores dedicated to servicing storage I/O requests.
	IoWorkloadCPUUtilization float32 `json:"io_workload_cpu_utilization,omitempty"`

	// The maximum percentage of CPU Utilization on the cores dedicated to servicing storage I/O requests. Calculated over time interval across appliance. Derived from an average of utilization across all nodes within the appliance.
	MaxIoWorkloadCPUUtilization float32 `json:"max_io_workload_cpu_utilization,omitempty"`

	CommonMaxAvgIopsBandwidthFields
}

// PerformanceMetricsByNodeResponse is returned by performance_metrics_by_node
type PerformanceMetricsByNodeResponse struct {
	CommonMetricsFields

	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	// Average number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	AvgCurrentLogins *int64 `json:"avg_current_logins,omitempty"`

	// The average percentage of CPU Utilization on the cores dedicated to servicing storage I/O requests. Calculated over time across appliance. Derived from an average of utilization across all nodes within the appliance.
	AvgIoWorkloadCPUUtilization float32 `json:"avg_io_workload_cpu_utilization,omitempty"`

	// Average size of read and write operations in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Weighted average read bandwidth in bytes per second.
	AvgReadBandwidth float32 `json:"avg_read_bandwidth,omitempty"`

	// Average reads per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Weighted average total bandwidth in bytes per second.
	AvgTotalBandwidth float32 `json:"avg_total_bandwidth,omitempty"`

	// Average total input and output operations per second.
	AvgTotalIops float64 `json:"avg_total_iops,omitempty"`

	// Weighted average write bandwidth in bytes per second.
	AvgWriteBandwidth float32 `json:"avg_write_bandwidth,omitempty"`

	// Average writes per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// The number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	CurrentLogins *int64 `json:"current_logins,omitempty"`

	// The percentage of CPU Utilization on the cores dedicated to servicing storage I/O requests.
	IoWorkloadCPUUtilization float32 `json:"io_workload_cpu_utilization,omitempty"`

	// Maximum number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxCurrentLogins *int64 `json:"max_current_logins,omitempty"`

	// The maximum percentage of CPU Utilization on the cores dedicated to servicing storage I/O requests. Calculated over time across appliance. Derived from an average of utilization across all nodes within the appliance.
	MaxIoWorkloadCPUUtilization float32 `json:"max_io_workload_cpu_utilization,omitempty"`

	// Unique identifier representing a specific node.
	NodeID string `json:"node_id,omitempty"`

	CommonUnalignedFields

	CommonMaxAvgIopsBandwidthFields
}

// PerformanceMetricsByVolumeResponse is returned by performance_metrics_by_volume
type PerformanceMetricsByVolumeResponse struct {
	CommonMetricsFields

	// Unique identifier representing a specific volume.
	VolumeID string `json:"volume_id,omitempty"`

	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	CommonAvgFields

	CommonMaxAvgIopsBandwidthFields
}

// VolumeMirrorTransferRateResponse is returned by volume_mirror_transfer_rate
type VolumeMirrorTransferRateResponse struct {
	// Unique identifier representing a specific volume.
	ID string `json:"id,omitempty"`

	// The timestamp of the last read or write operation.
	Timestamp strfmt.DateTime `json:"timestamp,omitempty"`

	// The read or write bandwidth in bytes per second.
	SynchronizationBandwidth float32 `json:"synchronization_bandwidth,omitempty"`

	// The read or write bandwidth in bytes per second.
	MirrorBandwidth float32 `json:"mirror_bandwidth,omitempty"`

	// The amount of data remaining in the bandwidth
	DataRemaining float32 `json:"data_remaining,omitempty"`
}

type CommonAvgFields struct {
	// Average size of read and write operations in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Weighted average read bandwidth in bytes per second.
	AvgReadBandwidth float32 `json:"avg_read_bandwidth,omitempty"`

	// Average reads per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Weighted average total bandwidth in bytes per second.
	AvgTotalBandwidth float32 `json:"avg_total_bandwidth,omitempty"`

	// Average total input and output operations per second.
	AvgTotalIops float64 `json:"avg_total_iops,omitempty"`

	// Weighted average write bandwidth in bytes per second.
	AvgWriteBandwidth float32 `json:"avg_write_bandwidth,omitempty"`

	// Average writes per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`
}

// PerformanceMetricsByClusterResponse is returned by performance_metrics_by_cluster
type PerformanceMetricsByClusterResponse struct {
	CommonMetricsFields

	// Identifier of the cluster.
	ClusterID string `json:"cluster_id,omitempty"`

	// Average size of read and write operations in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Weighted average  read bandwidth in bytes per second.
	AvgReadBandwidth float32 `json:"avg_read_bandwidth,omitempty"`

	// Average reads per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Weighted average total bandwidth in bytes per second.
	AvgTotalBandwidth float32 `json:"avg_total_bandwidth,omitempty"`

	// Average total input and output operations per second.
	AvgTotalIops float64 `json:"avg_total_iops,omitempty"`

	// Weighted average write bandwidth in bytes per second.
	AvgWriteBandwidth float32 `json:"avg_write_bandwidth,omitempty"`

	// Average writes per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	CommonMaxAvgIopsBandwidthFields
}

// PerformanceMetricsByVMResponse is returned by performance_metrics_by_vm
type PerformanceMetricsByVMResponse struct {
	CommonMetricsFields

	// Unique identifier representing a specific virtual machine.
	VMID string `json:"vm_id,omitempty"`

	CommonAvgFields

	CommonMaxAvgIopsBandwidthFields
}

// PerformanceMetricsByVgResponse is returned by performance_metrics_by_vg
type PerformanceMetricsByVgResponse struct {
	CommonMetricsFields

	// Unique identifier representing a volume group.
	VgID string `json:"vg_id,omitempty"`

	// Average size of read and write operations in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// Read rate in byte/sec.
	ReadBandwidth float32 `json:"read_bandwidth,omitempty"`

	// Total read operations per second.
	ReadIops float32 `json:"read_iops,omitempty"`

	// Total data transfer rate in bytes per second.
	TotalBandwidth float32 `json:"total_bandwidth,omitempty"`

	// Total read and write operations per second.
	TotalIops float32 `json:"total_iops,omitempty"`

	// Write rate in byte/sec.
	WriteBandwidth float32 `json:"write_bandwidth,omitempty"`

	// Total write operations per second.
	WriteIops float32 `json:"write_iops,omitempty"`
}

// PerformanceMetricsByFeFcPortResponse is returned by performance_metrics_by_fe_fc_port
type PerformanceMetricsByFeFcPortResponse struct {
	CommonMetricsFields

	// Reference to the associated frontend fibre channel port (fc_port) on which these metrics were recorded.
	FePortID string `json:"fe_port_id,omitempty"`

	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	// Average number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	AvgCurrentLogins *int64 `json:"avg_current_logins,omitempty"`

	// Average dumped frames per second.
	AvgDumpedFramesPs float32 `json:"avg_dumped_frames_ps,omitempty"`

	// Average invalid crc count per second.
	AvgInvalidCrcCountPs float32 `json:"avg_invalid_crc_count_ps,omitempty"`

	// Average invalid transmission word count per second.
	AvgInvalidTxWordCountPs float32 `json:"avg_invalid_tx_word_count_ps,omitempty"`

	// Average size of read and write operations in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Average link failure count per second.
	AvgLinkFailureCountPs float32 `json:"avg_link_failure_count_ps,omitempty"`

	// Average loss of signal count per second.
	AvgLossOfSignalCountPs float32 `json:"avg_loss_of_signal_count_ps,omitempty"`

	// Average loss of sync count per second.
	AvgLossOfSyncCountPs float32 `json:"avg_loss_of_sync_count_ps,omitempty"`

	// Average primitive sequence protocol error count per second.
	AvgPrimSeqProtErrCountPs float32 `json:"avg_prim_seq_prot_err_count_ps,omitempty"`

	// Weighted average read bandwidth in bytes per second.
	AvgReadBandwidth float32 `json:"avg_read_bandwidth,omitempty"`

	// Average reads per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Weighted average total bandwidth in bytes per second.
	AvgTotalBandwidth float32 `json:"avg_total_bandwidth,omitempty"`

	// Average total input and output operations per second.
	AvgTotalIops float64 `json:"avg_total_iops,omitempty"`

	// Weighted average write bandwidth in bytes per second.
	AvgWriteBandwidth float32 `json:"avg_write_bandwidth,omitempty"`

	// Average writes per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// The number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	CurrentLogins *int64 `json:"current_logins,omitempty"`

	// Dumped frames per second.
	DumpedFramesPs float32 `json:"dumped_frames_ps,omitempty"`

	// Invalid crc count per second.
	InvalidCrcCountPs float32 `json:"invalid_crc_count_ps,omitempty"`

	// Invalid transmission word count per second.
	InvalidTxWordCountPs float32 `json:"invalid_tx_word_count_ps,omitempty"`

	// Link failure count per second.
	LinkFailureCountPs float32 `json:"link_failure_count_ps,omitempty"`

	// Loss of signal count per second.
	LossOfSignalCountPs float32 `json:"loss_of_signal_count_ps,omitempty"`

	// Loss of sync count per second.
	LossOfSyncCountPs float32 `json:"loss_of_sync_count_ps,omitempty"`

	// Maximum number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxCurrentLogins *int64 `json:"max_current_logins,omitempty"`

	// The maximum dumped frames per second.
	MaxDumpedFramesPs float32 `json:"max_dumped_frames_ps,omitempty"`

	// The maximum invalid crc count per second.
	MaxInvalidCrcCountPs float32 `json:"max_invalid_crc_count_ps,omitempty"`

	// The maximum invalid transmission word count per second.
	MaxInvalidTxWordCountPs float32 `json:"max_invalid_tx_word_count_ps,omitempty"`

	// The maximum link failure count per second.
	MaxLinkFailureCountPs float32 `json:"max_link_failure_count_ps,omitempty"`

	// The maximum loss of signal count per second.
	MaxLossOfSignalCountPs float32 `json:"max_loss_of_signal_count_ps,omitempty"`

	// The maximum loss of sync count per second.
	MaxLossOfSyncCountPs float32 `json:"max_loss_of_sync_count_ps,omitempty"`

	// The maximum primitive sequence protocol error count per second.
	MaxPrimSeqProtErrCountPs float32 `json:"max_prim_seq_prot_err_count_ps,omitempty"`

	// Reference to the node the port belongs to.
	NodeID string `json:"node_id,omitempty"`

	// Primitive sequence protocol error count per second.
	PrimSeqProtErrCountPs float32 `json:"prim_seq_prot_err_count_ps,omitempty"`

	CommonUnalignedFields

	CommonMaxAvgIopsBandwidthFields
}

// PerformanceMetricsByFeEthPortResponse is returned by performance_metrics_by_fe_eth_port
type PerformanceMetricsByFeEthPortResponse struct {
	CommonMetricsFields

	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	// Reference to the associated frontend ethernet port (eth_port) on which these metrics were recorded.
	FePortID string `json:"fe_port_id,omitempty"`

	CommonEthPortFields
}

// PerformanceMetricsByFeEthNodeResponse is returned by performance_metrics_by_fe_eth_node
type PerformanceMetricsByFeEthNodeResponse struct {
	CommonMetricsFields

	// Reference to the associated appliance on which these metrics were recorded.
	ApplianceID string `json:"appliance_id,omitempty"`

	CommonEthPortFields
}

// PerformanceMetricsByFeFcNodeResponse is returned by performance_metrics_by_fe_fc_node
type PerformanceMetricsByFeFcNodeResponse struct {
	CommonMetricsFields
	// Average number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	AvgCurrentLogins *int64 `json:"avg_current_logins,omitempty"`

	// Average dumped frames per second.
	AvgDumpedFramesPs float32 `json:"avg_dumped_frames_ps,omitempty"`

	// Average invalid crc count per second.
	AvgInvalidCrcCountPs float32 `json:"avg_invalid_crc_count_ps,omitempty"`

	// Average invalid transmission word count per second.
	AvgInvalidTxWordCountPs float32 `json:"avg_invalid_tx_word_count_ps,omitempty"`

	// Average size of read and write operations in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Average link failure count per second.
	AvgLinkFailureCountPs float32 `json:"avg_link_failure_count_ps,omitempty"`

	// Average loss of signal count per second.
	AvgLossOfSignalCountPs float32 `json:"avg_loss_of_signal_count_ps,omitempty"`

	// Average loss of sync count per second.
	AvgLossOfSyncCountPs float32 `json:"avg_loss_of_sync_count_ps,omitempty"`

	// Average primitive sequence protocol error count per second.
	AvgPrimSeqProtErrCountPs float32 `json:"avg_prim_seq_prot_err_count_ps,omitempty"`

	// Weighted average read bandwidth in bytes per second.
	AvgReadBandwidth float32 `json:"avg_read_bandwidth,omitempty"`

	// Average reads per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Weighted average total bandwidth in bytes per second.
	AvgTotalBandwidth float32 `json:"avg_total_bandwidth,omitempty"`

	// Average total input and output operations per second.
	AvgTotalIops float64 `json:"avg_total_iops,omitempty"`

	// Weighted average write bandwidth in bytes per second.
	AvgWriteBandwidth float32 `json:"avg_write_bandwidth,omitempty"`

	// Average writes per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// The number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	CurrentLogins *int64 `json:"current_logins,omitempty"`

	// Dumped frames per second.
	DumpedFramesPs float32 `json:"dumped_frames_ps,omitempty"`

	// Invalid crc count per second.
	InvalidCrcCountPs float32 `json:"invalid_crc_count_ps,omitempty"`

	// Invalid transmission word count per second.
	InvalidTxWordCountPs float32 `json:"invalid_tx_word_count_ps,omitempty"`

	// Link failure count per second.
	LinkFailureCountPs float32 `json:"link_failure_count_ps,omitempty"`

	// Loss of signal count per second.
	LossOfSignalCountPs float32 `json:"loss_of_signal_count_ps,omitempty"`

	// Loss of sync count per second.
	LossOfSyncCountPs float32 `json:"loss_of_sync_count_ps,omitempty"`

	// Maximum number of logins to the target from initiators.
	// Maximum: 9.223372036854776e+18
	// Minimum: 0
	MaxCurrentLogins *int64 `json:"max_current_logins,omitempty"`

	// The maximum dumped frames per second.
	MaxDumpedFramesPs float32 `json:"max_dumped_frames_ps,omitempty"`

	// The maximum invalid crc count per second.
	MaxInvalidCrcCountPs float32 `json:"max_invalid_crc_count_ps,omitempty"`

	// The maximum invalid transmission word count per second.
	MaxInvalidTxWordCountPs float32 `json:"max_invalid_tx_word_count_ps,omitempty"`

	// The maximum link failure count per second.
	MaxLinkFailureCountPs float32 `json:"max_link_failure_count_ps,omitempty"`

	// The maximum loss of signal count per second.
	MaxLossOfSignalCountPs float32 `json:"max_loss_of_signal_count_ps,omitempty"`

	// The maximum loss of sync count per second.
	MaxLossOfSyncCountPs float32 `json:"max_loss_of_sync_count_ps,omitempty"`

	// The maximum primitive sequence protocol error count per second.
	MaxPrimSeqProtErrCountPs float32 `json:"max_prim_seq_prot_err_count_ps,omitempty"`

	// Reference to the associated node on which these metrics were recorded.
	NodeID string `json:"node_id,omitempty"`

	// Primitive sequence protocol error count per second.
	PrimSeqProtErrCountPs float32 `json:"prim_seq_prot_err_count_ps,omitempty"`

	CommonUnalignedFields

	CommonMaxAvgIopsBandwidthFields
}

// PerformanceMetricsByFileSystemResponse is returned by performance_metrics_by_file_system
type PerformanceMetricsByFileSystemResponse struct {
	CommonMetricsFields

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Average read rate in bytes per second.
	AvgReadBandwidth float32 `json:"avg_read_bandwidth,omitempty"`

	// Average read operations per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Average read and write size in bytes.
	AvgSize float32 `json:"avg_size,omitempty"`

	// Average data transfer rate in bytes per second.
	AvgTotalBandwidth float32 `json:"avg_total_bandwidth,omitempty"`

	// Average read and write operations per second.
	AvgTotalIops float32 `json:"avg_total_iops,omitempty"`

	// Average write rate in bytes per second.
	AvgWriteBandwidth float32 `json:"avg_write_bandwidth,omitempty"`

	// Average write operations per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// Unique identifier of the file system.
	FileSystemID string `json:"file_system_id,omitempty"`

	// Maximum of average read and write latency in microseconds.
	MaxAvgLatency float32 `json:"max_avg_latency,omitempty"`

	// Maximum of average read latency in microseconds.
	MaxAvgReadLatency float32 `json:"max_avg_read_latency,omitempty"`

	// Maximum of average read size in bytes.
	MaxAvgReadSize float32 `json:"max_avg_read_size,omitempty"`

	// Maximum of average read and write size in bytes.
	MaxAvgSize float32 `json:"max_avg_size,omitempty"`

	// Maximum of average write latency in microseconds.
	MaxAvgWriteLatency float32 `json:"max_avg_write_latency,omitempty"`

	// Maximum of average write size in bytes.
	MaxAvgWriteSize float32 `json:"max_avg_write_size,omitempty"`

	// Maximum read and write operations per second.
	MaxIops float32 `json:"max_iops,omitempty"`

	// Maximum read rate in bytes per second.
	MaxReadBandwidth float32 `json:"max_read_bandwidth,omitempty"`

	// Maximum read operations per second.
	MaxReadIops float32 `json:"max_read_iops,omitempty"`

	// Maximum data transfer rate in bytes per second.
	MaxTotalBandwidth float32 `json:"max_total_bandwidth,omitempty"`

	// Maximum write rate in bytes per second.
	MaxWriteBandwidth float32 `json:"max_write_bandwidth,omitempty"`

	// Maximum write operations per second.
	MaxWriteIops float32 `json:"max_write_iops,omitempty"`

	// Read rate in bytes per second.
	ReadBandwidth float32 `json:"read_bandwidth,omitempty"`

	// Total read operations per second.
	ReadIops float32 `json:"read_iops,omitempty"`

	// Total data transfer rate in bytes per second.
	TotalBandwidth float32 `json:"total_bandwidth,omitempty"`

	// Total read and write operations per second.
	TotalIops float32 `json:"total_iops,omitempty"`

	// Write rate in bytes per second.
	WriteBandwidth float32 `json:"write_bandwidth,omitempty"`

	// Total write operations per second.
	WriteIops float32 `json:"write_iops,omitempty"`
}

// PerformanceMetricsBySmbNodeResponse is returned by performance_metrics_smb_by_node
type PerformanceMetricsBySmbNodeResponse struct {
	CommonMetricsFields
	CommonSMBFields
}

// PerformanceMetricsBySmbClientResponse is returned by performance_metrics_smb_builtinclient_by_node
type PerformanceMetricsBySmbClientResponse struct {
	CommonMetricsFields
	CommonSMBFields
}

// PerformanceMetricsBySmbCacheResponse is returned by performance_metrics_smb_branch_cache_by_node
type PerformanceMetricsBySmbCacheResponse struct {
	CommonMetricsFields
	// Average hash latency.
	HashAvgLatency float32 `json:"hash_avg_latency,omitempty"`

	// Average hash size.
	HashAvgSize float32 `json:"hash_avg_size,omitempty"`

	// Average max hash latency.
	HashMaxAvgLatency float32 `json:"hash_max_avg_latency,omitempty"`

	// Average max hash size.
	HashMaxAvgSize float32 `json:"hash_max_avg_size,omitempty"`

	// Max hash latency.
	HashMaxLatency float32 `json:"hash_max_latency,omitempty"`

	// Max hash size.
	HashMaxSize float32 `json:"hash_max_size,omitempty"`

	// Min hash latency.
	HashMinLatency float32 `json:"hash_min_latency,omitempty"`

	// Max hash size.
	HashMinSize float32 `json:"hash_min_size,omitempty"`

	// Max used threads
	MaxUsedThreads float32 `json:"max_used_threads,omitempty"`

	// Unique identifier of the node.
	NodeID string `json:"node_id,omitempty"`

	// Total rejected task.
	TotalRejectedTasks float32 `json:"total_rejected_tasks,omitempty"`

	// Total tasks.
	TotalTasks float32 `json:"total_tasks,omitempty"`
}

// PerformanceMetricsBySmbV1NodeResponse is returned by performance_metrics_smb1_by_node
type PerformanceMetricsBySmbV1NodeResponse struct {
	CommonMetricsFields
	CommonSMBFields
}

// PerformanceMetricsBySmbV1BuiltinClientResponse is returned by performance_metrics_smb1_builtinclient_by_node
type PerformanceMetricsBySmbV1BuiltinClientResponse struct {
	CommonMetricsFields
	CommonSMBFields
}

// PerformanceMetricsBySmbV2NodeResponse is returned by performance_metrics_smb2_by_node
type PerformanceMetricsBySmbV2NodeResponse struct {
	CommonMetricsFields
	CommonSMBFields
}

// PerformanceMetricsBySmbV2BuiltinClientResponse is returned by performance_metrics_smb2_builtinclient_by_node
type PerformanceMetricsBySmbV2BuiltinClientResponse struct {
	CommonMetricsFields
	CommonSMBFields
}

// PerformanceMetricsByNfsResponse is returned by performance_metrics_nfs_by_node
type PerformanceMetricsByNfsResponse struct {
	CommonMetricsFields

	// Average read and write size in bytes.
	AvgIoSize float32 `json:"avg_io_size,omitempty"`

	// Average read and write latency in microseconds.
	AvgLatency float32 `json:"avg_latency,omitempty"`

	// Average read operations per second.
	AvgReadIops float32 `json:"avg_read_iops,omitempty"`

	// Average read latency in microseconds.
	AvgReadLatency float32 `json:"avg_read_latency,omitempty"`

	// Average read size in bytes.
	AvgReadSize float32 `json:"avg_read_size,omitempty"`

	// Average write latency in microseconds.
	AvgWriteLatency float32 `json:"avg_write_latency,omitempty"`

	// Average read and write operations per second.
	AvgTotalIops float32 `json:"avg_total_iops,omitempty"`

	// Average write operations per second.
	AvgWriteIops float32 `json:"avg_write_iops,omitempty"`

	// Average write size in bytes.
	AvgWriteSize float32 `json:"avg_write_size,omitempty"`

	// Maximum of average read and write size in bytes.
	MaxAvgIoSize float32 `json:"max_avg_io_size,omitempty"`

	// Maximum of average read and write latency in microseconds.
	MaxAvgLatency float32 `json:"max_avg_latency,omitempty"`

	// Maximum of average read latency in microseconds.
	MaxAvgReadLatency float32 `json:"max_avg_read_latency,omitempty"`

	// Maximum of average read size in bytes.
	MaxAvgReadSize float32 `json:"max_avg_read_size,omitempty"`

	// Maximum of average write latency in microseconds.
	MaxAvgWriteLatency float32 `json:"max_avg_write_latency,omitempty"`

	// Maximum of average write size in bytes.
	MaxAvgWriteSize float32 `json:"max_avg_write_size,omitempty"`

	// Maximum read and write operations per second.
	MaxIops float32 `json:"max_iops,omitempty"`

	// Maximum read operations per second.
	MaxReadIops float32 `json:"max_read_iops,omitempty"`

	// Maximum write operations per second.
	MaxWriteIops float32 `json:"max_write_iops,omitempty"`

	// Unique identifier of the node.
	NodeID string `json:"node_id,omitempty"`

	// Total read operations per second.
	ReadIops float32 `json:"read_iops,omitempty"`

	// Total read and write operations per second.
	TotalIops float32 `json:"total_iops,omitempty"`

	// Total write operations per second.
	WriteIops float32 `json:"write_iops,omitempty"`
}

// PerformanceMetricsByNfsv3Response is returned by performance_metrics_nfsv3_by_node
type PerformanceMetricsByNfsv3Response struct {
	CommonMetricsFields
	CommonNfsv34ResponseFields
}

// PerformanceMetricsByNfsv4Response is returned by performance_metrics_nfsv4_by_node
type PerformanceMetricsByNfsv4Response struct {
	CommonMetricsFields
	CommonNfsv34ResponseFields
}

// CopyMetricsByApplianceResponse is returned by  copy_metrics_by_appliance
type CopyMetricsByApplianceResponse struct {
	CommonMetricsFields
	CopyMetricsCommonFields
	// Unique identifier of the appliance.
	ApplianceID string `json:"appliance_id,omitempty"`
}

// CopyMetricsByClusterResponse is returned by  copy_metrics_by_cluster
type CopyMetricsByClusterResponse struct {
	CommonMetricsFields
	CopyMetricsCommonFields
}

// CopyMetricsByVolumeGroupResponse is returned by  copy_metrics_by_vg
type CopyMetricsByVolumeGroupResponse struct {
	CommonMetricsFields
	CopyMetricsCommonFields
	// Unique identifier of the volume group.
	VgID string `json:"vg_id,omitempty"`
}

// CopyMetricsByRemoteSystemResponse is returned by  copy_metrics_by_remote_system
type CopyMetricsByRemoteSystemResponse struct {
	CommonMetricsFields
	CopyMetricsCommonFields
	// Unique identifier of the remote system.
	RemoteSystemID string `json:"remote_system_id,omitempty"`
}

// CopyMetricsByVolumeResponse is returned by copy_metrics_by_volume
type CopyMetricsByVolumeResponse struct {
	CommonMetricsFields
	CopyMetricsCommonFields
	// Unique identifier of the volume.
	VolumeID string `json:"volume_id,omitempty"`
}

// Fields returns fields which must be requested to fill struct
func (h *ApplianceMetrics) Fields() []string {
	return []string{"appliance_id", "physical_total", "physical_used"}
}
