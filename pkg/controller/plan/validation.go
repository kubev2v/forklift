package plan

import (
	"context"
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	NetRefNotValid  = "NetworkMapRefNotValid"
	NetMapNotReady  = "NetworkMapNotReady"
	DsMapNotReady   = "StorageMapNotReady"
	DsRefNotValid   = "StorageRefNotValid"
	VMRefNotValid   = "VMRefNotValid"
	VMNotFound      = "VMNotFound"
	VMAlreadyExists = "VMAlreadyExists"
	DuplicateVM     = "DuplicateVM"
	NameNotValid    = "TargetNameNotValid"
	Executing       = "Executing"
	Succeeded       = "Succeeded"
	Failed          = "Failed"
	Canceled        = "Canceled"
	Deleted         = "Deleted"
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
	NotSet        = "NotSet"
	NotFound      = "NotFound"
	NotUnique     = "NotUnique"
	Ambiguous     = "Ambiguous"
	NotValid      = "NotValid"
	Modified      = "Modified"
	UserRequested = "UserRequested"
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
	// Provider.
	pv := validation.ProviderPair{Client: r}
	conditions, err := pv.Validate(plan.Spec.Provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SetCondition(conditions.List...)
	if plan.Status.HasCondition(validation.SourceProviderNotReady) {
		return nil
	}
	plan.Referenced.Provider.Source = pv.Referenced.Source
	plan.Referenced.Provider.Destination = pv.Referenced.Destination
	//
	// Mapping
	err = r.validateNetworkMap(plan)
	if err != nil {
		return err
	}
	err = r.validateStorageMap(plan)
	if err != nil {
		return err
	}
	//
	// VM list.
	err = r.validateVM(plan)
	if err != nil {
		return err
	}

	return nil
}

//
// Validate network mapping ref.
func (r *Reconciler) validateNetworkMap(plan *api.Plan) (err error) {
	ref := plan.Spec.Map.Network
	newCnd := libcnd.Condition{
		Type:     NetRefNotValid,
		Status:   True,
		Category: Critical,
		Message:  "Map.Network is not valid.",
	}
	if !libref.RefSet(&ref) {
		newCnd.Reason = NotSet
		plan.Status.SetCondition(newCnd)
		return
	}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	mp := &api.NetworkMap{}
	err = r.Get(context.TODO(), key, mp)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		plan.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !mp.Status.HasCondition(libcnd.Ready) {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     NetMapNotReady,
			Status:   True,
			Category: Critical,
			Message:  "Map.Network does not have Ready condition.",
		})
	}

	plan.Referenced.Map.Network = mp

	return
}

//
// Validate storage mapping ref.
func (r *Reconciler) validateStorageMap(plan *api.Plan) (err error) {
	ref := plan.Spec.Map.Storage
	newCnd := libcnd.Condition{
		Type:     DsRefNotValid,
		Status:   True,
		Category: Critical,
		Message:  "Map.Storage is not valid.",
	}
	if !libref.RefSet(&ref) {
		newCnd.Reason = NotSet
		plan.Status.SetCondition(newCnd)
		return
	}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	mp := &api.StorageMap{}
	err = r.Get(context.TODO(), key, mp)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		plan.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !mp.Status.HasCondition(libcnd.Ready) {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     DsMapNotReady,
			Status:   True,
			Category: Critical,
			Message:  "Map.Storage does not have Ready condition.",
		})
	}

	plan.Referenced.Map.Storage = mp

	return
}

//
// Validate listed VMs.
func (r *Reconciler) validateVM(plan *api.Plan) error {
	if plan.Status.HasCondition(Executing) {
		return nil
	}
	notFound := libcnd.Condition{
		Type:     VMNotFound,
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
	nameNotValid := libcnd.Condition{
		Type:     NameNotValid,
		Status:   True,
		Reason:   NotValid,
		Category: Critical,
		Message:  "Target VM name not valid.",
		Items:    []string{},
	}
	alreadyExists := libcnd.Condition{
		Type:     VMAlreadyExists,
		Status:   True,
		Reason:   NotUnique,
		Category: Critical,
		Message:  "Target VM already exists.",
		Items:    []string{},
	}
	setOf := map[string]bool{}
	//
	// Referenced VMs.
	for _, vm := range plan.Spec.VMs {
		ref := &vm.Ref
		if ref.NotSet() {
			plan.Status.SetCondition(libcnd.Condition{
				Type:     VMRefNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "Either `ID` or `Name` required.",
			})
			continue
		}
		// Source.
		provider := plan.Referenced.Provider.Source
		if provider == nil {
			return nil
		}
		inventory, pErr := web.NewClient(provider)
		if pErr != nil {
			return liberr.Wrap(pErr)
		}
		_, pErr = inventory.VM(ref)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				notFound.Items = append(notFound.Items, ref.String())
				continue
			}
			if errors.As(pErr, &web.RefNotUniqueError{}) {
				ambiguous.Items = append(ambiguous.Items, ref.String())
				continue
			}
			return liberr.Wrap(pErr)
		}
		if len(k8svalidation.IsQualifiedName(ref.Name)) > 0 {
			nameNotValid.Items = append(nameNotValid.Items, ref.String())
		}
		if _, found := setOf[ref.ID]; found {
			notUnique.Items = append(notUnique.Items, ref.String())
		} else {
			setOf[ref.ID] = true
		}
		// Destination.
		provider = plan.Referenced.Provider.Destination
		if provider == nil {
			return nil
		}
		inventory, pErr = web.NewClient(provider)
		if pErr != nil {
			return liberr.Wrap(pErr)
		}
		id := path.Join(
			plan.TargetNamespace(),
			ref.Name)
		pErr = inventory.Get(&ocp.VM{}, id)
		if pErr == nil {
			if vm, found := plan.Status.Migration.FindVM(*ref); found {
				if vm.Completed != nil && vm.Error == nil {
					continue // migrated.
				}
			}
			alreadyExists.Items = append(
				alreadyExists.Items,
				ref.String())
		} else {
			if !errors.As(pErr, &web.NotFoundError{}) {
				return liberr.Wrap(pErr)
			}
		}
	}
	if len(notFound.Items) > 0 {
		plan.Status.SetCondition(notFound)
	}
	if len(notUnique.Items) > 0 {
		plan.Status.SetCondition(notUnique)
	}
	if len(alreadyExists.Items) > 0 {
		plan.Status.SetCondition(alreadyExists)
	}
	if len(nameNotValid.Items) > 0 {
		plan.Status.SetCondition(nameNotValid)
	}
	if len(ambiguous.Items) > 0 {
		plan.Status.SetCondition(ambiguous)
	}

	return nil
}
