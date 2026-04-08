package conversion

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const ()

// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

// Reasons
const (
	NotSet = "NotSet"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

func (r *Reconciler) validate(conversion *api.Conversion) (err error) {

	return
}
