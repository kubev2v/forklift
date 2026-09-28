package client

import "time"

const (
	snapshotOperationTimeout = 30 * time.Minute
	vmOperationTimeout       = 15 * time.Minute
	vmReadTimeout            = 2 * time.Minute
)
