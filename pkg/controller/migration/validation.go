package migration

import (
	cnd "github.com/konveyor/controller/pkg/condition"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
)

//
// Types
const ()

//
// Categories
const (
	Advisory = cnd.Advisory
	Critical = cnd.Critical
	Error    = cnd.Error
	Warn     = cnd.Warn
)

// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Statuses
const (
	True  = cnd.True
	False = cnd.False
)

// Messages
const (
	ReadyMessage = "The migration is ready."
)

// Validate the plan resource.
func (r Reconciler) validate(plan *api.Migration) error {

	return nil
}
