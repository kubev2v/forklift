package migration

import (
	"context"
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	plancnt "github.com/konveyor/forklift-controller/pkg/controller/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	PlanNotValid = "PlanNotValid"
	PlanNotReady = "PlanNotReady"
	VMNotFound   = "VMNotFound"
	VMNotUnique  = "VMNotUnique"
	Running      = "Running"
	Executing    = plancnt.Executing
	Succeeded    = plancnt.Succeeded
	Failed       = plancnt.Failed
	Canceled     = plancnt.Canceled
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

// Reasons
const (
	NotSet    = "NotSet"
	NotFound  = "NotFound"
	Ambiguous = "Ambiguous"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Validate the migration resource.
func (r *Reconciler) validate(migration *api.Migration) (plan *api.Plan, err error) {
	newCnd := libcnd.Condition{
		Type:     PlanNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "The `plan` is not valid.",
	}
	ref := migration.Spec.Plan
	if !libref.RefSet(&ref) {
		migration.Status.SetCondition(newCnd)
		return
	}
	plan = &api.Plan{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err = r.Get(context.TODO(), key, plan)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		migration.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !plan.Status.HasCondition(libcnd.Ready) {
		migration.Status.SetCondition(
			libcnd.Condition{
				Type:     PlanNotReady,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  "The `plan` does not have Ready condition.",
			})
		return
	}

	// Validate the refs in the Cancel array
	notFound := libcnd.Condition{
		Type:     VMNotFound,
		Status:   True,
		Reason:   NotFound,
		Category: Warn,
		Message:  "VM not found.",
		Items:    []string{},
	}
	ambiguous := libcnd.Condition{
		Type:     VMNotUnique,
		Status:   True,
		Reason:   Ambiguous,
		Category: Warn,
		Message:  "VM reference is ambiguous.",
		Items:    []string{},
	}
	source := plan.Spec.Provider.Source
	provider := &api.Provider{}
	key = client.ObjectKey{
		Namespace: source.Namespace,
		Name:      source.Name,
	}
	err = r.Get(context.TODO(), key, provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	inventory, err := web.NewClient(provider)
	if err != nil {
		return
	}
	for _, ref := range migration.Spec.Cancel {
		_, err = inventory.VM(&ref)
		if err != nil {
			if errors.As(err, &web.NotFoundError{}) {
				notFound.Items = append(notFound.Items, ref.String())
				err = nil
				continue
			}
			if errors.As(err, &web.RefNotUniqueError{}) {
				ambiguous.Items = append(ambiguous.Items, ref.String())
				err = nil
				continue
			}
			return
		}
	}

	if len(notFound.Items) > 0 {
		migration.Status.SetCondition(notFound)
	}
	if len(ambiguous.Items) > 0 {
		migration.Status.SetCondition(ambiguous)
	}

	return
}
