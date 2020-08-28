package migration

import (
	"context"
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	PlanNotValid = "PlanNotValid"
	PlanNotReady = "PlanNotReady"
	Running      = "Running"
	Failed       = "Failed"
)

//
// Categories
const (
	Advisory = cnd.Advisory
	Critical = cnd.Critical
	Error    = cnd.Error
	Warn     = cnd.Warn
)

// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Statuses
const (
	True  = cnd.True
	False = cnd.False
)

// Messages
const (
	ReadyMessage        = "The migration is ready."
	PlanNotValidMessage = "`plan` not valid."
	PlanNotReadyMessage = "`plan` does not have Ready condition."
	RunningMessage      = "The migration is running."
	FailedMessage       = "The migration failed."
)

//
// Validate the plan resource.
func (r *Reconciler) validate(migration *api.Migration) error {
	err := r.validatePlan(migration)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate plan reference.
func (r *Reconciler) validatePlan(migration *api.Migration) error {
	ref := migration.Spec.Plan
	if !libref.RefSet(&ref) {
		migration.Status.SetCondition(
			cnd.Condition{
				Type:     PlanNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  PlanNotValidMessage,
			})
		return nil
	}
	plan := &api.Plan{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := r.Get(context.TODO(), key, plan)
	if errors.IsNotFound(err) {
		err = nil
		migration.Status.SetCondition(
			cnd.Condition{
				Type:     PlanNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  PlanNotValidMessage,
			})
	}
	if err != nil {
		return liberr.Wrap(err)
	}
	if !plan.Status.HasCondition(cnd.Ready) {
		migration.Status.SetCondition(
			cnd.Condition{
				Type:     PlanNotReady,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  PlanNotReadyMessage,
			})
	}

	return nil
}
