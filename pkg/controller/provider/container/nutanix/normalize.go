package nutanix

import (
	"fmt"
	"strconv"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

func storageContainerEntityFromV2(raw map[string]interface{}) map[string]interface{} {
	uuid := firstString(raw, "storage_container_uuid", "uuid")
	name := getString(raw, "name")
	clusterUUID := getString(raw, "cluster_uuid")

	usageBytes := int64(0)
	if usageStats, ok := raw["usage_stats"].(map[string]interface{}); ok {
		usageBytes = parseNumericString(usageStats["storage.user_usage_bytes"])
		if usageBytes == 0 {
			usageBytes = parseNumericString(usageStats["storage.reserved_usage_bytes"])
		}
	}

	maxCapacity := firstInt64(raw, "max_capacity_bytes", "max_capacity", "total_capacity")

	resources := map[string]interface{}{
		"replication_factor":  firstInt(raw, "replication_factor"),
		"max_capacity_bytes":  maxCapacity,
		"usage_bytes":         usageBytes,
		"compression_enabled": getBool(raw, "compression_enabled"),
		"on_disk_dedup":       getString(raw, "on_disk_dedup"),
		"erasure_code":        getString(raw, "erasure_code"),
		"cluster_reference": map[string]interface{}{
			"uuid": clusterUUID,
		},
	}

	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
			"name": name,
		},
		"status": map[string]interface{}{
			"resources": resources,
		},
	}
}

func storageContainerEntityFromV4(raw map[string]interface{}) map[string]interface{} {
	uuid := firstString(raw, "extId", "container_ext_id", "containerExtId")
	name := getString(raw, "name")
	clusterUUID := firstString(raw, "clusterExtId", "cluster_ext_id")

	usageBytes := firstInt64(raw, "usageBytes", "usage_bytes")
	maxCapacity := firstInt64(raw, "maxCapacityBytes", "max_capacity_bytes")

	resources := map[string]interface{}{
		"replication_factor":  firstInt(raw, "replicationFactor", "replication_factor"),
		"max_capacity_bytes":  maxCapacity,
		"usage_bytes":         usageBytes,
		"compression_enabled": firstBool(raw, "isCompressionEnabled", "compression_enabled"),
		"on_disk_dedup":       firstString(raw, "onDiskDedup", "on_disk_dedup"),
		"erasure_code":        firstString(raw, "erasureCode", "erasure_code"),
		"cluster_reference": map[string]interface{}{
			"uuid": clusterUUID,
		},
	}

	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": uuid,
			"name": name,
		},
		"status": map[string]interface{}{
			"resources": resources,
		},
	}
}

// filterEntitiesByCluster keeps only the entities whose cluster UUID -- read
// from entity via the given dot-separated field path -- matches clusterUUID.
// If clusterUUID is empty (Prism Element, or no clusterUuid setting
// configured on the Provider), every entity is returned unfiltered.
//
// This filters client-side on data we've already fetched, rather than
// relying on the v3 API's "filter" (FIQL) query parameter, whose supported
// attributes vary and are inconsistently documented/implemented across
// entity kinds.
func filterEntitiesByCluster(
	entities []map[string]interface{},
	clusterUUID string,
	clusterUUIDPath string,
) []map[string]interface{} {
	if clusterUUID == "" {
		return entities
	}

	filtered := make([]map[string]interface{}, 0, len(entities))
	for _, entity := range entities {
		if getString(entity, clusterUUIDPath) == clusterUUID {
			filtered = append(filtered, entity)
		}
	}

	return filtered
}

func filterStorageContainersByCluster(
	entities []map[string]interface{},
	clusterUUID string,
) []map[string]interface{} {
	return filterEntitiesByCluster(entities, clusterUUID, "status.resources.cluster_reference.uuid")
}

func extractMapList(result map[string]interface{}, key string) ([]map[string]interface{}, error) {
	raw, ok := result[key]
	if !ok {
		return nil, liberr.New(fmt.Sprintf("missing %q in response", key))
	}

	list, ok := raw.([]interface{})
	if !ok {
		return nil, liberr.New(fmt.Sprintf("invalid %q list in response", key))
	}

	entities := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		entity, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := getString(values, key); value != "" {
			return value
		}
	}
	return ""
}

func firstInt(values map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value := getInt(values, key); value != 0 {
			return value
		}
	}
	return getInt(values, keys[0])
}

func firstInt64(values map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		if value := getInt64(values, key); value != 0 {
			return value
		}
	}
	if len(keys) > 0 {
		return getInt64(values, keys[0])
	}
	return 0
}

func firstBool(values map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if boolean, ok := value.(bool); ok {
				return boolean
			}
		}
	}
	return false
}

func parseNumericString(value interface{}) int64 {
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed
		}
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	}
	return 0
}
