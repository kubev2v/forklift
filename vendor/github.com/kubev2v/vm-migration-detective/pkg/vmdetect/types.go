package vmdetect

import (
	"github.com/kubev2v/vm-migration-detective/pkg/checks"
	"github.com/kubev2v/vm-migration-detective/pkg/types"
)

// Re-export types from pkg/types
type Credentials = types.Credentials
type CacheKey = types.CacheKey
type DB = types.DB

// Re-export check types from pkg/checks
type Concern = checks.Concern
type ConcernCategory = checks.ConcernCategory

// Re-export concern severity categories
const (
	ConcernCategoryCritical    = checks.ConcernCategoryCritical
	ConcernCategoryWarning     = checks.ConcernCategoryWarning
	ConcernCategoryInformation = checks.ConcernCategoryInformation
	ConcernCategoryAdvisory    = checks.ConcernCategoryAdvisory
	ConcernCategoryError       = checks.ConcernCategoryError
)
