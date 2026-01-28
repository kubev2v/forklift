package collector

import (
	"os"
	"strconv"
	"time"
)

// Environment variables for EC2 collector configuration.
const (
	// EC2InventoryIntervalEnv is the environment variable name for configuring
	// the inventory collector's AWS polling interval in seconds.
	EC2InventoryIntervalEnv = "EC2_INVENTORY_INTERVAL_SECONDS"
)

// Default values.
const (
	// DefaultRefreshInterval is the default interval for AWS API polling.
	// Can be overridden via EC2_INVENTORY_INTERVAL_SECONDS environment variable.
	DefaultRefreshInterval = 10 * time.Second
)

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
//   - AWS API calls are expensive (latency, rate limits, cost)
//   - Concurrent collections would duplicate work and stress AWS APIs
//   - Safer to skip than to queue up multiple slow collections
//
// Trade-offs:
//   - Lower values: Faster detection of AWS changes, higher AWS API usage/cost
//   - Higher values: Slower detection, lower AWS API usage/cost
//
// Recommended ranges:
//   - Development: 5-10 seconds
//   - Production (small): 10-30 seconds
//   - Production (large): 30-60 seconds
//
// Used by: The collector's periodic ticker in Start() method
// Affects: How quickly the inventory database reflects changes in AWS
var RefreshInterval = loadRefreshInterval()

func loadRefreshInterval() time.Duration {
	if s, found := os.LookupEnv(EC2InventoryIntervalEnv); found {
		if seconds, err := strconv.Atoi(s); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return DefaultRefreshInterval
}
