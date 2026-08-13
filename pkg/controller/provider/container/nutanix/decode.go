package nutanix

import (
	"fmt"
	"strconv"
	"strings"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"k8s.io/apimachinery/pkg/runtime"
)

type ref struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type metadata struct {
	UUID       string            `json:"uuid"`
	Name       string            `json:"name"`
	Categories map[string]string `json:"categories"`
}

type v3ListMetadata struct {
	TotalMatches int `json:"total_matches"`
}

type v3ListResponse[T any] struct {
	Metadata v3ListMetadata `json:"metadata"`
	Entities []T            `json:"entities"`
}

type v4ListMetadata struct {
	TotalAvailableResults int `json:"totalAvailableResults"`
}

type v4ListResponse[T any] struct {
	Metadata v4ListMetadata `json:"metadata"`
	Data     []T            `json:"data"`
}

func decodeEntity(entity map[string]interface{}, out any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(entity, out)
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// nutanixBool coerces Nutanix JSON booleans that may arrive as native booleans
// or string values such as flash_mode "ENABLED"/"DISABLED".
func nutanixBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToUpper(strings.TrimSpace(typed)) {
		case "TRUE", "1", "YES", "ON", "ENABLED":
			return true
		case "FALSE", "0", "NO", "OFF", "DISABLED":
			return false
		}
		parsed, err := strconv.ParseBool(typed)
		return err == nil && parsed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	default:
		return false
	}
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

func filterByMatch[T any](entities []T, clusterUUID string, match func(T) string) []T {
	if clusterUUID == "" {
		return entities
	}

	filtered := make([]T, 0, len(entities))
	for _, entity := range entities {
		if match(entity) == clusterUUID {
			filtered = append(filtered, entity)
		}
	}
	return filtered
}

func excludeByMatch[T any](entities []T, excludedUUIDs map[string]bool, match func(T) string) []T {
	if len(excludedUUIDs) == 0 {
		return entities
	}

	filtered := make([]T, 0, len(entities))
	for _, entity := range entities {
		if !excludedUUIDs[match(entity)] {
			filtered = append(filtered, entity)
		}
	}
	return filtered
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
