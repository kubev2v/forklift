package checks

// ConcernCategory represents the severity level of a concern
type ConcernCategory string

const (
	// ConcernCategoryCritical indicates a critical issue that must be resolved
	ConcernCategoryCritical ConcernCategory = "Critical"
	// ConcernCategoryWarning indicates a warning that should be addressed
	ConcernCategoryWarning ConcernCategory = "Warning"
	// ConcernCategoryInformation indicates informational message
	ConcernCategoryInformation ConcernCategory = "Information"
	// ConcernCategoryAdvisory indicates an advisory recommendation
	ConcernCategoryAdvisory ConcernCategory = "Advisory"
	// ConcernCategoryError indicates an error occurred during the check
	ConcernCategoryError ConcernCategory = "Error"
)

// Concern represents a validation concern found during checks
type Concern struct {
	// ID is the unique identifier for this concern type
	ID string `json:"id"`
	// Category indicates the severity level of the concern
	Category ConcernCategory `json:"category"`
	// Label is a human-readable short description
	Label string `json:"label"`
	// Message provides detailed information about the concern
	Message string `json:"message"`
}

// CheckType represents the type of validation check
type CheckType string

const (
	// CheckTypeFstab validates fstab entries for migration compatibility
	CheckTypeFstab CheckType = "fstab"
	// CheckTypeDiskAccess validates that the disk is accessible (not encrypted)
	CheckTypeDiskAccess CheckType = "disk-access"
)

// AllCheckTypes returns all available check types
func AllCheckTypes() []CheckType {
	return []CheckType{
		CheckTypeFstab,
		CheckTypeDiskAccess,
	}
}

// IsValidCheckType checks if a check type is valid
func IsValidCheckType(checkType CheckType) bool {
	for _, ct := range AllCheckTypes() {
		if ct == checkType {
			return true
		}
	}
	return false
}

// CheckResult represents the result of a single validation check
type CheckResult struct {
	// CheckType indicates which check was performed
	CheckType CheckType `json:"check_type"`
	// Passed indicates whether the check passed (true) or found concerns (false)
	Passed bool `json:"passed"`
	// Concerns contains all issues found by this check (empty if passed)
	Concerns []Concern `json:"concerns,omitempty"`
	// Error contains the error message if an unexpected error occurred
	Error *string `json:"error,omitempty"`
}
