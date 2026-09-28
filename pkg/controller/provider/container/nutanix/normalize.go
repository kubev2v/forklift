package nutanix

// filterStorageContainersByCluster keeps only storage containers whose cluster
// UUID matches clusterUUID. When clusterUUID is empty every entity is returned.
func filterStorageContainersByCluster(
	entities []storageContainerEntity,
	clusterUUID string,
) []storageContainerEntity {
	return filterByMatch(entities, clusterUUID, func(entity storageContainerEntity) string {
		return entity.clusterUUID()
	})
}

// isPrismCentralCluster reports whether a v3 cluster entity is Prism
// Central's own self-registered pseudo-cluster entry, rather than a real
// AHV/Prism Element cluster it manages.
func isPrismCentralCluster(entity clusterEntity) bool {
	return entity.isPrismCentralCluster()
}

// withoutPrismCentralClusters drops Prism Central's own self-registered
// pseudo-cluster entry from a list of cluster entities.
func withoutPrismCentralClusters(entities []clusterEntity) []clusterEntity {
	filtered := make([]clusterEntity, 0, len(entities))
	for _, entity := range entities {
		if !entity.isPrismCentralCluster() {
			filtered = append(filtered, entity)
		}
	}
	return filtered
}

// excludedClusterUUIDs returns the UUIDs of any Prism Central pseudo-
// cluster entries found among clusterEntities, so callers can filter out
// hosts (and similar) that reference them.
func excludedClusterUUIDs(clusterEntities []clusterEntity) map[string]bool {
	excluded := map[string]bool{}
	for _, entity := range clusterEntities {
		if !entity.isPrismCentralCluster() {
			continue
		}
		if entity.Metadata.UUID != "" {
			excluded[entity.Metadata.UUID] = true
		}
	}
	return excluded
}

// excludeHostsByCluster drops hosts whose cluster UUID is in excludedUUIDs.
func excludeHostsByCluster(
	entities []hostEntity,
	excludedUUIDs map[string]bool,
) []hostEntity {
	return excludeByMatch(entities, excludedUUIDs, func(entity hostEntity) string {
		return entity.clusterUUID()
	})
}
