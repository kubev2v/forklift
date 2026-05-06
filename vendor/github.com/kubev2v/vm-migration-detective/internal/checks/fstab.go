package checks

import (
	"strings"

	"github.com/kubev2v/vm-migration-detective/pkg/types"
)

// FstabCheck validates fstab entries for migration compatibility
type FstabCheck struct{}

// NewFstabCheck creates a new FstabCheck instance
func NewFstabCheck() *FstabCheck {
	return &FstabCheck{}
}

// Run executes the fstab validation check using the shared inspector
func (c *FstabCheck) Run(params InspectionParams) CheckResult {
	// Call the inspection using the shared inspector
	inspectionData, err := params.Inspector.InspectWithVirt(
		params.Ctx,
		params.VMMoref,
		params.SnapshotMoref,
		params.DiskInfo,
	)
	if err != nil {
		errMsg := err.Error()
		return CheckResult{
			Passed:   false,
			Concerns: nil,
			Error:    &errMsg,
		}
	}

	// Validate the fstab data
	return ValidateMigrateableFstab(inspectionData)
}

// ValidateMigrateableFstab checks if the VM's fstab is migrateable
// Returns concerns if fstab created by path (/dev/disk/by-path/)
func ValidateMigrateableFstab(inspectionData *types.VirtInspectorXML) CheckResult {
	if inspectionData == nil {
		return CheckResult{
			Passed:   true,
			Concerns: nil,
		}
	}

	var concerns []Concern

	// Check all operating systems in the inspection data
	for _, os := range inspectionData.Operatingsystems {
		// Check all mountpoints for path-based device references
		for _, mountpoint := range os.Mountpoints.Mountpoint {
			if strings.HasPrefix(mountpoint.Device, "/dev/disk/by-path/") {
				concerns = append(concerns, Concern{
					ID:       "fstab-by-path-device",
					Category: ConcernCategoryCritical,
					Label:    "Fstab contains /dev/disk/by-path/ entry",
					Message: "Fstab contains /dev/disk/by-path/ entries which are not migrateable. " +
						"Found device: " + mountpoint.Device + " mounted at: " + mountpoint.MountPoint,
				})
			}
		}
	}

	return CheckResult{
		Passed:   len(concerns) == 0,
		Concerns: concerns,
	}
}
