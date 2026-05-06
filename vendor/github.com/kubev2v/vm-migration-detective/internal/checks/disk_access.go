package checks

import (
	"strings"
)

// DiskAccessCheck validates that the disk is accessible (not encrypted)
type DiskAccessCheck struct{}

// NewDiskAccessCheck creates a new DiskAccessCheck instance
func NewDiskAccessCheck() *DiskAccessCheck {
	return &DiskAccessCheck{}
}

// Run executes the disk access validation check using the shared inspector
// It tries to run virt-inspector and checks if the disk is encrypted
// Returns concerns if disk is encrypted or not accessible
func (c *DiskAccessCheck) Run(params InspectionParams) CheckResult {
	// Try to run the inspection using the shared inspector
	_, err := params.Inspector.InspectWithVirt(
		params.Ctx,
		params.VMMoref,
		params.SnapshotMoref,
		params.DiskInfo,
	)
	if err != nil {
		// Check if the error message indicates an encrypted disk
		// The inspection function already detects encrypted disks and returns a specific error message
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "disk encryption detected") ||
			strings.Contains(errStr, "encrypted disk") ||
			strings.Contains(errStr, "cannot access encrypted") {
			// Encrypted disk is a known validation failure, not an unexpected error
			return CheckResult{
				Passed: false,
				Concerns: []Concern{
					{
						ID:       "disk-encrypted",
						Category: ConcernCategoryCritical,
						Label:    "Disk encryption detected",
						Message:  err.Error(),
					},
				},
				Error: nil,
			}
		}

		// Other errors are unexpected errors
		errMsg := err.Error()
		return CheckResult{
			Passed:   false,
			Concerns: nil,
			Error:    &errMsg,
		}
	}

	// Inspection succeeded - disk is accessible
	return CheckResult{
		Passed:   true,
		Concerns: nil,
	}
}
