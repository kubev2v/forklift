package plan

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
	SourceNotValid      = "SourceProviderNotValid"
	DestinationNotValid = "DestinationProviderNotValid"
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
	ReadyMessage               = "The migration plan is ready."
	SourceNotValidMessage      = "`providers.source` not valid."
	DestinationNotValidMessage = "`providers.destination` not valid."
)

//
// Validate the plan resource.
func (r *Reconciler) validate(plan *api.Plan) error {
	err := r.validateProvider(plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate provider field.
func (r *Reconciler) validateProvider(plan *api.Plan) error {
	//
	// Source
	ref := plan.Spec.Provider.Source
	if !libref.RefSet(&ref) {
		plan.Status.SetCondition(
			cnd.Condition{
				Type:     SourceNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  SourceNotValidMessage,
			})
	} else {
		provider := &api.Provider{}
		key := client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}
		err := r.Get(context.TODO(), key, provider)
		if errors.IsNotFound(err) {
			err = nil
			plan.Status.SetCondition(
				cnd.Condition{
					Type:     SourceNotValid,
					Status:   True,
					Reason:   NotFound,
					Category: Critical,
					Message:  SourceNotValidMessage,
				})
		}
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	//
	// Destination
	ref = plan.Spec.Provider.Destination
	if !libref.RefSet(&ref) {
		plan.Status.SetCondition(
			cnd.Condition{
				Type:     DestinationNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  DestinationNotValidMessage,
			})
		return nil
	} else {
		provider := &api.Provider{}
		key := client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}
		err := r.Get(context.TODO(), key, provider)
		if errors.IsNotFound(err) {
			err = nil
			plan.Status.SetCondition(
				cnd.Condition{
					Type:     DestinationNotValid,
					Status:   True,
					Reason:   NotFound,
					Category: Critical,
					Message:  DestinationNotValidMessage,
				})
		}
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}
