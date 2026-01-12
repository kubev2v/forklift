// Package inventory provides shared utilities for accessing EC2 provider inventory data.
// Used by builder, validator, and migrator packages to avoid code duplication.
package inventory

import "errors"

// Common errors for inventory operations.
var (
	// ErrNoAWSInstanceObject is returned when inventory data doesn't contain the AWS instance object.
	ErrNoAWSInstanceObject = errors.New("no AWS instance object found in inventory data")

	// ErrNoEBSVolumes is returned when a VM has no EBS volumes to migrate.
	ErrNoEBSVolumes = errors.New("no EBS volumes found for VM")

	// ErrNoBlockDevices is returned when a VM has no block devices attached.
	ErrNoBlockDevices = errors.New("VM has no block devices attached")
)
