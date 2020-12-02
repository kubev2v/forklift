package plan

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
	"net/http"
)

//
// Types
const (
	VMNotValid  = "VMNotValid"
	DuplicateVM = "DuplicateVM"
	Executing   = "Executing"
	Succeeded   = "Succeeded"
	Failed      = "Failed"
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
	NotSet    = "NotSet"
	NotFound  = "NotFound"
	NotUnique = "NotUnique"
)

//
// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Validate the plan resource.
func (r *Reconciler) validate(plan *api.Plan) error {
	if plan.Status.HasAnyCondition(Executing) {
		return nil
	}
	// Provider.
	provider := validation.ProviderPair{Client: r}
	conditions, err := provider.Validate(plan.Spec.Provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SetCondition(conditions.List...)
	if plan.Status.HasCondition(validation.SourceProviderNotReady) {
		return nil
	}
	//
	// Map
	network := validation.NetworkPair{Client: r, Provider: provider.Referenced}
	conditions, err = network.Validate(plan.Spec.Map.Networks)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.UpdateConditions(conditions)
	storage := validation.StoragePair{Client: r, Provider: provider.Referenced}
	conditions, err = storage.Validate(plan.Spec.Map.Datastores)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.UpdateConditions(conditions)
	//
	// VM list.
	err = r.validateVM(provider.Referenced.Source, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	plan.Referenced.Provider.Source = provider.Referenced.Source
	plan.Referenced.Provider.Destination = provider.Referenced.Destination

	return nil
}

//
// Validate listed VMs.
func (r *Reconciler) validateVM(provider *api.Provider, plan *api.Plan) error {
	if provider == nil {
		return nil
	}
	inventory, pErr := web.NewClient(provider)
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
		return liberr.Wrap(web.ProviderNotSupportedErr)
	}
	notValid := libcnd.Condition{
		Type:     VMNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "The VMs (list) contains invalid VMs.",
		Items:    []string{},
	}
	notUnique := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   NotUnique,
		Category: Critical,
		Message:  "The VMs (list) contains duplicate VMs.",
		Items:    []string{},
	}
	//
	// Referenced VMs.
	setOf := map[string]bool{}
	for _, vm := range plan.Spec.VMs {
		if _, found := setOf[vm.ID]; found {
			notUnique.Items = append(notUnique.Items, vm.ID)
			setOf[vm.ID] = true
		}
		status, pErr := inventory.Get(resource, vm.ID)
		if pErr != nil {
			return liberr.Wrap(pErr)
		}
		switch status {
		case http.StatusOK:
		case http.StatusNotFound:
			notValid.Items = append(notValid.Items, vm.ID)
		default:
			return liberr.New(http.StatusText(status))
		}
	}
	if len(notValid.Items) > 0 {
		plan.Status.SetCondition(notValid)
	}
	if len(notUnique.Items) > 0 {
		plan.Status.SetCondition(notUnique)
	}

	return nil
}
