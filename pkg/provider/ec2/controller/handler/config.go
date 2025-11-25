package handler

import "time"

// Configuration constants for the EC2 provider handlers.
//
// These constants control the behavior of Kubernetes controllers that manage
// EC2 provider resources like Plans, NetworkMaps, and StorageMaps.
const (
	// InventoryPollingInterval controls how frequently controllers reconcile inventory-dependent resources.
	// Detects changes in EC2 instances, networks, storage, and plan status. Trade-off: responsiveness vs API load.
	InventoryPollingInterval = 60 * time.Second
)
