package plan

import (
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/validation"
	"net/http"
)

//
// Types
const (
	VMNotValid = "VMNotValid"
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
// Validate the plan resource.
func (r *Reconciler) validate(plan *api.Plan) error {
	// Provider.
	provider := validation.ProviderPair{Client: r}
	conditions, err := provider.Validate(plan.Spec.Provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SetCondition(conditions.List...)
	// Map
	network := validation.NetworkPair{Client: r, Provider: provider.Referenced}
	conditions, err = network.Validate(plan.Spec.Map.Networks)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SetCondition(conditions.List...)
	storage := validation.StoragePair{Client: r, Provider: provider.Referenced}
	conditions, err = storage.Validate(plan.Spec.Map.Datastores)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SetCondition(conditions.List...)
	// VM list.
	err = r.validateVM(provider.Referenced.Source, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate listed VMs.
func (r *Reconciler) validateVM(provider *api.Provider, plan *api.Plan) error {
	if provider == nil {
		return nil
	}
	notValid := []string{}
	pClient, pErr := web.NewClient(*provider)
	if pErr != nil {
		return liberr.Wrap(pErr)
	}
	var resource interface{}
	switch provider.Type() {
	case api.OpenShift:
		return nil
	case api.VSphere:
		resource = &vsphere.VM{}
	default:
		return web.ProviderNotSupported
	}
	for _, vm := range plan.Spec.VMs {
		status, pErr := pClient.Get(resource, vm.ID)
		if pErr != nil {
			return liberr.Wrap(pErr)
		}
		switch status {
		case http.StatusOK:
		case http.StatusPartialContent:
			return ProviderInvNotReady
		case http.StatusNotFound:
			notValid = append(notValid, vm.ID)
		default:
			return liberr.New(http.StatusText(status))
		}
	}
	if len(notValid) > 0 {
		plan.Status.SetCondition(cnd.Condition{
			Type:     VMNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "The VMs (list) contains invalid VMs.",
		})
	}

	return nil
}
