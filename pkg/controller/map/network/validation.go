package network

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
)

//
// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

//
// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

//
// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Validate the mp resource.
func (r *Reconciler) validate(mp *api.NetworkMap) error {
	provider := validation.ProviderPair{Client: r}
	conditions, err := provider.Validate(mp.Spec.Provider)
	if err != nil {
		return err
	}
	mp.Status.UpdateConditions(conditions)
	if mp.Status.HasCondition(validation.SourceProviderNotReady) {
		return nil
	}
	network := validation.NetworkPair{Client: r, Provider: provider.Referenced}
	conditions, err = network.Validate(mp.Spec.Map)
	if err != nil {
		return err
	}
	mp.Status.UpdateConditions(conditions)

	return nil
}
