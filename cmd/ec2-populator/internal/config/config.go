package config

import (
	"fmt"
)

// Config holds EC2 populator configuration.
type Config struct {
	Region                 string // Required - AWS region (where snapshot exists and volume will be created)
	TargetAvailabilityZone string // Required - AZ where to create volume (where OpenShift workers are)
	SnapshotID             string
	SecretName             string
	CRName                 string
	CRNamespace            string
	OwnerUID               string // Required - used to identify prime PVC (prime-{uid})
	PVCSize                int64  // Required - PVC size in bytes (with overhead), passed by populator-machinery
}

// Validate checks required configuration.
func (c *Config) Validate() error {
	// Volume path is NOT needed - we create PVs via AWS API, no filesystem access

	if c.Region == "" {
		return fmt.Errorf("region is required (AWS region for snapshot and volume)")
	}
	if c.TargetAvailabilityZone == "" {
		return fmt.Errorf("target-availability-zone is required (where to create volume)")
	}
	if c.SnapshotID == "" {
		return fmt.Errorf("snapshot-id is required")
	}
	if c.OwnerUID == "" {
		return fmt.Errorf("owner-uid is required (to identify prime PVC)")
	}
	if c.PVCSize <= 0 {
		return fmt.Errorf("pvc-size is required and must be greater than 0 (passed by populator-machinery)")
	}
	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{SnapshotID=%s, Region=%s, TargetAZ=%s}",
		c.SnapshotID, c.Region, c.TargetAvailabilityZone)
}
