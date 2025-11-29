package collector

import "time"

const (
	// RefreshInterval defines how frequently the collector fetches fresh inventory data from AWS EC2.
	//
	// Purpose: Controls the polling frequency for AWS API calls to retrieve EC2 instances, volumes,
	// networks, and other inventory resources.
	//
	// Overlap protection: YES - If a new collection is triggered before the previous one finishes,
	// it is SKIPPED. The Collect() method uses a mutex and 'collecting' flag to prevent concurrent
	// executions. You'll see: "Collection already in progress, skipping" in logs.
	//
	// Why protect against overlap?
	// - AWS API calls are expensive (latency, rate limits, cost)
	// - Concurrent collections would duplicate work and stress AWS APIs
	// - Safer to skip than to queue up multiple slow collections
	//
	// Used by: The collector's periodic ticker in Start() method
	// Affects: How quickly the inventory database reflects changes in AWS
	RefreshInterval = 10 * time.Second
)
