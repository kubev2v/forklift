package collector

import (
	"os"
	"strconv"
	"time"
)

const (
	AzureInventoryIntervalEnv = "AZURE_INVENTORY_INTERVAL_SECONDS"
)

const (
	DefaultRefreshInterval = 10 * time.Second
)

var RefreshInterval = loadRefreshInterval()

func loadRefreshInterval() time.Duration {
	if s, found := os.LookupEnv(AzureInventoryIntervalEnv); found {
		if seconds, err := strconv.Atoi(s); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return DefaultRefreshInterval
}
