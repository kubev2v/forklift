package nutanix

// Version-specific API shapes. Normalizers convert these into the canonical
// v3-style entities in resource.go so ApplyTo stays shared across Prism modes.

type storageContainerV2Raw struct {
	ClusterUUID          string                 `json:"cluster_uuid"`
	CompressionEnabled   bool                   `json:"compression_enabled"`
	ErasureCode          string                 `json:"erasure_code"`
	MaxCapacity          int64                  `json:"max_capacity"`
	MaxCapacityBytes     int64                  `json:"max_capacity_bytes"`
	Name                 string                 `json:"name"`
	OnDiskDedup          string                 `json:"on_disk_dedup"`
	ReplicationFactor    int                    `json:"replication_factor"`
	StorageContainerUUID string                 `json:"storage_container_uuid"`
	TotalCapacity        int64                  `json:"total_capacity"`
	UsageStats           map[string]interface{} `json:"usage_stats"`
	UUID                 string                 `json:"uuid"`
}

func (r storageContainerV2Raw) toEntity() storageContainerEntity {
	uuid := coalesce(r.StorageContainerUUID, r.UUID)
	maxCapacity := r.MaxCapacityBytes
	if maxCapacity == 0 {
		maxCapacity = coalesceInt64(r.MaxCapacity, r.TotalCapacity)
	}

	usageBytes := int64(0)
	if r.UsageStats != nil {
		usageBytes = parseNumericString(r.UsageStats["storage.user_usage_bytes"])
		if usageBytes == 0 {
			usageBytes = parseNumericString(r.UsageStats["storage.reserved_usage_bytes"])
		}
	}

	return storageContainerEntity{
		Metadata: metadata{
			Name: r.Name,
			UUID: uuid,
		},
		Status: storageContainerStatus{
			Resources: storageContainerResources{
				ClusterReference:   ref{UUID: r.ClusterUUID},
				CompressionEnabled: r.CompressionEnabled,
				ErasureCode:        r.ErasureCode,
				MaxCapacityBytes:   maxCapacity,
				OnDiskDedup:        r.OnDiskDedup,
				ReplicationFactor:  r.ReplicationFactor,
				UsageBytes:         usageBytes,
			},
		},
	}
}

type storageContainerV4Raw struct {
	ClusterExtID         string `json:"clusterExtId"`
	ClusterExtIDAlt      string `json:"cluster_ext_id"`
	CompressionEnabled   bool   `json:"compression_enabled"`
	ContainerExtID       string `json:"container_ext_id"`
	ContainerExtIDAlt    string `json:"containerExtId"`
	ErasureCode          string `json:"erasure_code"`
	ErasureCodeAlt       string `json:"erasureCode"`
	ExtID                string `json:"extId"`
	IsCompressionEnabled bool   `json:"isCompressionEnabled"`
	MaxCapacityBytes     int64  `json:"max_capacity_bytes"`
	MaxCapacityBytesAlt  int64  `json:"maxCapacityBytes"`
	Name                 string `json:"name"`
	OnDiskDedup          string `json:"on_disk_dedup"`
	OnDiskDedupAlt       string `json:"onDiskDedup"`
	ReplicationFactor    int    `json:"replication_factor"`
	ReplicationFactorAlt int    `json:"replicationFactor"`
	UsageBytes           int64  `json:"usage_bytes"`
	UsageBytesAlt        int64  `json:"usageBytes"`
}

func (r storageContainerV4Raw) toEntity() storageContainerEntity {
	uuid := coalesce(r.ExtID, r.ContainerExtIDAlt, r.ContainerExtID)
	clusterUUID := coalesce(r.ClusterExtIDAlt, r.ClusterExtID)

	return storageContainerEntity{
		Metadata: metadata{
			Name: r.Name,
			UUID: uuid,
		},
		Status: storageContainerStatus{
			Resources: storageContainerResources{
				ClusterReference: ref{UUID: clusterUUID},
				CompressionEnabled: coalesceBool(
					r.IsCompressionEnabled,
					r.CompressionEnabled,
				),
				ErasureCode: coalesce(r.ErasureCodeAlt, r.ErasureCode),
				MaxCapacityBytes: coalesceInt64(
					r.MaxCapacityBytesAlt,
					r.MaxCapacityBytes,
				),
				OnDiskDedup: coalesce(r.OnDiskDedupAlt, r.OnDiskDedup),
				ReplicationFactor: coalesceInt(
					r.ReplicationFactorAlt,
					r.ReplicationFactor,
				),
				UsageBytes: coalesceInt64(r.UsageBytesAlt, r.UsageBytes),
			},
		},
	}
}

type imageV4Raw struct {
	ExtID     string `json:"extId"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	Source    struct {
		URL string `json:"url"`
	} `json:"source"`
	Type string `json:"type"`
}

func (r imageV4Raw) toEntity() imageEntity {
	return imageEntity{
		Metadata: metadata{UUID: r.ExtID},
		Status: imageStatus{
			Name: r.Name,
			Resources: imageResources{
				ImageType: r.Type,
				SizeBytes: r.SizeBytes,
				SourceURI: r.Source.URL,
			},
		},
	}
}

func coalesceInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

func coalesceInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	if len(values) > 0 {
		return values[0]
	}
	return 0
}

func coalesceBool(values ...bool) bool {
	for _, value := range values {
		if value {
			return true
		}
	}
	return false
}
