package plan

import (
	"context"
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	HostNotReady  = "HostNotReady"
	VMNotValid    = "VMNotValid"
	DuplicateVM   = "DuplicateVM"
	DupTargetName = "DuplicateTargetName"
	Executing     = "Executing"
	Succeeded     = "Succeeded"
	Failed        = "Failed"
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
	Ambiguous = "Ambiguous"
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
	plan.Referenced.Provider.Source = provider.Referenced.Source
	plan.Referenced.Provider.Destination = provider.Referenced.Destination
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
	// Hosts.
	err = r.validateHosts(plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	//
	// VM list.
	err = r.validateVM(plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate hosts in the same ns as the source provider.
func (r *Reconciler) validateHosts(plan *api.Plan) (err error) {
	provider := plan.Referenced.Provider.Source
	if provider == nil {
		return nil
	}
	list := api.HostList{}
	options := &client.ListOptions{
		Namespace: provider.Namespace,
	}
	err = r.List(context.TODO(), &list, options)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	cnd := libcnd.Condition{
		Type:     HostNotReady,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "Host does not have the Ready condition.",
		Items:    []string{},
	}
	for _, host := range list.Items {
		if !host.Status.HasCondition(libcnd.Ready) {
			cnd.Items = append(cnd.Items, host.Name)
		}
	}
	if len(cnd.Items) > 0 {
		plan.Status.SetCondition(cnd)
	}

	return
}

//
// Validate listed VMs.
func (r *Reconciler) validateVM(plan *api.Plan) error {
	provider := plan.Referenced.Provider.Source
	if provider == nil {
		return nil
	}
	inventory, pErr := web.NewClient(provider)
	if pErr != nil {
		return liberr.Wrap(pErr)
	}
	notValid := libcnd.Condition{
		Type:     VMNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "VM not found.",
		Items:    []string{},
	}
	notUnique := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   NotUnique,
		Category: Critical,
		Message:  "Duplicate (source) VM.",
		Items:    []string{},
	}
	ambiguous := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   Ambiguous,
		Category: Critical,
		Message:  "VM reference is ambiguous.",
		Items:    []string{},
	}
	setOf := map[string]bool{}
	//
	// Referenced VMs.
	for _, vm := range plan.Spec.VMs {
		ref := vm.Ref
		if ref.NotSet() {
			plan.Status.SetCondition(libcnd.Condition{
				Type:     VMNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "Either `ID` or `Name` required.",
			})
			continue
		}
		_, pErr := inventory.VM(&ref)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				notValid.Items = append(notValid.Items, ref.String())
				continue
			}
			if errors.As(pErr, &web.RefNotUniqueError{}) {
				ambiguous.Items = append(ambiguous.Items, ref.String())
				continue
			}
			return liberr.Wrap(pErr)
		}
		if _, found := setOf[ref.ID]; found {
			notUnique.Items = append(notUnique.Items, ref.String())
		} else {
			setOf[ref.ID] = true
		}
	}
	if len(notValid.Items) > 0 {
		plan.Status.SetCondition(notValid)
	}
	if len(notUnique.Items) > 0 {
		plan.Status.SetCondition(notUnique)
	}
	if len(ambiguous.Items) > 0 {
		plan.Status.SetCondition(ambiguous)
	}

	return nil
}
