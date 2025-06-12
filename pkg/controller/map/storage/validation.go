package storage

import (
	"errors"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/validation"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	SourceStorageNotValid      = "SourceStorageNotValid"
	DestinationStorageNotValid = "DestinationStorageNotValid"
)

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

// Validate the mp resource.
func (r *Reconciler) validate(mp *api.StorageMap) error {
	pv := validation.ProviderPair{Client: r}
	conditions, err := pv.Validate(mp.Spec.Provider)
	if err != nil {
		return err
	}
	mp.Status.UpdateConditions(conditions)
	if mp.Status.HasAnyCondition(
		validation.SourceProviderNotValid,
		validation.SourceProviderNotReady,
		validation.DestinationProviderNotValid,
		validation.DestinationProviderNotReady) {
		return nil
	}
	mp.Referenced.Provider.Source = pv.Referenced.Source
	mp.Referenced.Provider.Destination = pv.Referenced.Destination
	err = r.validateSource(mp)
	if err != nil {
		return err
	}
	err = r.validateDestination(mp)
	if err != nil {
		return err
	}

	return nil
}

// Validate source refs.
func (r *Reconciler) validateSource(mp *api.StorageMap) (err error) {
	provider := mp.Referenced.Provider.Source
	inventory, err := web.NewClient(provider)
	if err != nil {
		return
	}
	notValid := []string{}
	ambiguous := []string{}
	references := refapi.Refs{}
	list := mp.Spec.Map
	for i := range list {
		ref := &list[i].Source
		if ref.NotSet() {
			mp.Status.SetCondition(libcnd.Condition{
				Type:     SourceStorageNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "Source storage: either `ID` or `Name` required.",
			})
			continue
		}
		_, pErr := inventory.Storage(ref)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				notValid = append(notValid, ref.String())
				continue
			}
			if errors.As(pErr, &web.RefNotUniqueError{}) {
				ambiguous = append(ambiguous, ref.String())
				continue
			}
			err = pErr
			return
		}
		references.List = append(references.List, *ref)
	}
	mp.Status.Refs = references
	if len(notValid) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     SourceStorageNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Source storage not found.",
			Items:    notValid,
		})
	}
	if len(ambiguous) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     SourceStorageNotValid,
			Status:   True,
			Reason:   Ambiguous,
			Category: Critical,
			Message:  "Source storage has ambiguous ref.",
			Items:    ambiguous,
		})
	}

	return
}

// Validate destination refs.
func (r *Reconciler) validateDestination(mp *api.StorageMap) (err error) {
	provider := mp.Referenced.Provider.Destination
	inventory, err := web.NewClient(provider)
	if err != nil {
		return
	}
	notValid := []string{}
	list := mp.Spec.Map
	for _, entry := range list {
		name := entry.Destination.StorageClass
		_, pErr := inventory.Storage(&refapi.Ref{Name: name})
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				notValid = append(notValid, entry.Destination.StorageClass)
			} else {
				err = pErr
				return
			}
		}
	}
	if len(notValid) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     DestinationStorageNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Destination storage not valid.",
			Items:    notValid,
		})
	}

	return
}
