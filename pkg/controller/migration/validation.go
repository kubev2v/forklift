package migration

import (
	"context"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	plancnt "github.com/konveyor/forklift-controller/pkg/controller/plan"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	PlanNotValid = "PlanNotValid"
	PlanNotReady = "PlanNotReady"
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
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Validate the plan resource.
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
	if errors.IsNotFound(err) {
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

	return
}
