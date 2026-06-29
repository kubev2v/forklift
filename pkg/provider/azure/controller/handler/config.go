package handler

import (
	"os"
	"strconv"
	"time"
)

const (
	AzureControllerIntervalEnv = "AZURE_CONTROLLER_INTERVAL_SECONDS"
)

const (
	DefaultInventoryPollingInterval = 15 * time.Second
)

var InventoryPollingInterval = loadInventoryPollingInterval()

func loadInventoryPollingInterval() time.Duration {
	if s, found := os.LookupEnv(AzureControllerIntervalEnv); found {
		if seconds, err := strconv.Atoi(s); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return DefaultInventoryPollingInterval
}
