package plan

import (
	"context"
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/validation"
	"k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	VMNotValid   = "VMNotValid"
	DuplicateVM  = "DuplicateVM"
	HostNotValid = "HostNotValid"
	HookNotValid = "HookNotValid"
	Executing    = "Executing"
	Succeeded    = "Succeeded"
	Failed       = "Failed"
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
	NotSet    = "NotSet"
	NotFound  = "NotFound"
	NotUnique = "NotUnique"
)

//
// Statuses
const (
	True  = cnd.True
	False = cnd.False
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
	plan.Status.Migration.SetSource(provider.Referenced.Source)
	plan.Status.Migration.SetDestination(provider.Referenced.Destination)
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

	//
	// Map status.
	plan.Status.Migration.Map = plan.Spec.Map

	return nil
}

//
// Validate listed VMs.
func (r *Reconciler) validateVM(provider *api.Provider, plan *api.Plan) error {
	if provider == nil {
		return nil
	}
	pClient, pErr := web.NewClient(provider)
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
	notValid := cnd.Condition{
		Type:     VMNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "The VMs (list) contains invalid VMs.",
		Items:    []string{},
	}
	notUnique := cnd.Condition{
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
		status, pErr := pClient.Get(resource, vm.ID)
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
	//
	// Hosts referenced by VMs.
	notValid = cnd.Condition{
		Type:     HostNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "Host referenced by VM not valid.",
		Items:    []string{},
	}
	for _, vm := range plan.Spec.VMs {
		if !ref.RefSet(vm.Host) {
			continue
		}
		host := &api.Host{}
		key := client.ObjectKey{
			Namespace: vm.Host.Namespace,
			Name:      vm.Host.Name,
		}
		err := r.Get(context.TODO(), key, host)
		if err != nil {
			if errors.IsNotFound(err) {
				notValid.Items = append(
					notValid.Items, path.Join(key.Namespace, key.Name))
				continue
			} else {
				return liberr.Wrap(err)
			}
		}
	}
	if len(notValid.Items) > 0 {
		plan.Status.SetCondition(notValid)
	}

	return nil
}
