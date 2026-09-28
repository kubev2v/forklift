package nutanix

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
