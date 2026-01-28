package handler

import (
	"os"
	"strconv"
	"time"
)

// Environment variables for EC2 handler configuration.
const (
	// EC2ControllerIntervalEnv is the environment variable name for configuring
	// the controller's inventory polling interval in seconds.
	EC2ControllerIntervalEnv = "EC2_CONTROLLER_INTERVAL_SECONDS"
)

// Default values.
const (
	// DefaultInventoryPollingInterval is the default interval for controller reconciliation.
	// Can be overridden via EC2_CONTROLLER_INTERVAL_SECONDS environment variable.
	DefaultInventoryPollingInterval = 15 * time.Second
)

// InventoryPollingInterval returns the configured interval for controller reconciliation.
// Reads from EC2_CONTROLLER_INTERVAL_SECONDS environment variable, falls back to default (60s).
//
// Trade-offs:
//   - Lower values: More responsive to inventory changes, higher API load on inventory service
//   - Higher values: Less responsive, lower API load, better for large deployments
//
// Recommended ranges:
//   - Development: 10-30 seconds
//   - Production (small): 30-60 seconds
//   - Production (large): 60-120 seconds
var InventoryPollingInterval = loadInventoryPollingInterval()

func loadInventoryPollingInterval() time.Duration {
	if s, found := os.LookupEnv(EC2ControllerIntervalEnv); found {
		if seconds, err := strconv.Atoi(s); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return DefaultInventoryPollingInterval
}
