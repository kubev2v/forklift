package network

import (
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/validation"
)

//
// Categories
const (
	Required = cnd.Required
	Advisory = cnd.Advisory
	Critical = cnd.Critical
	Error    = cnd.Error
	Warn     = cnd.Warn
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
	True  = cnd.True
	False = cnd.False
)

//
// Errors
var (
	ProviderInvNotReady = validation.ProviderInvNotReady
)

//
// Validate the mp resource.
func (r *Reconciler) validate(mp *api.NetworkMap) error {
	provider := validation.ProviderPair{Client: r}
	conditions, err := provider.Validate(mp.Spec.Provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	mp.Status.SetCondition(conditions.List...)
	network := validation.NetworkPair{Client: r, Provider: provider.Referenced}
	conditions, err = network.Validate(mp.Spec.Map)
	if err != nil {
		return liberr.Wrap(err)
	}
	mp.Status.SetCondition(conditions.List...)

	return nil
}
