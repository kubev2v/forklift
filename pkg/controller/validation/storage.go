package validation

import (
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/mapped"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	SourceStorageNotValid      = "SourceStorageNotValid"
	DestinationStorageNotValid = "DestinationStorageNotValid"
)

//
// Storage pair validation.
type StoragePair struct {
	client.Client
	Provider struct {
		Source      *api.Provider
		Destination *api.Provider
	}
}

//
// Validate pairs.
func (r *StoragePair) Validate(list []mapped.StoragePair) (result libcnd.Conditions, err error) {
	conditions, err := r.validateSource(list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	result.UpdateConditions(conditions)
	conditions, err = r.validateDestination(list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	result.UpdateConditions(conditions)

	return
}

//
// Validate source storage.
func (r *StoragePair) validateSource(list []mapped.StoragePair) (result libcnd.Conditions, err error) {
	provider := r.Provider.Source
	if provider == nil {
		return
	}
	inventory, err := web.NewClient(provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	notValid := []string{}
	ambiguous := []string{}
	for i := range list {
		ref := &list[i].Source
		if ref.NotSet() {
			result.SetCondition(libcnd.Condition{
				Type:     DestinationNetworkNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "Destination network: either `ID` or `Name` required.",
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
			err = liberr.Wrap(pErr)
			return
		}
	}
	if len(notValid) > 0 {
		result.SetCondition(libcnd.Condition{
			Type:     SourceStorageNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Source storage not found.",
			Items:    notValid,
		})
	}
	if len(ambiguous) > 0 {
		result.SetCondition(libcnd.Condition{
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

//
// Validate destination storage.
func (r *StoragePair) validateDestination(list []mapped.StoragePair) (result libcnd.Conditions, err error) {
	provider := r.Provider.Destination
	if provider == nil {
		return
	}
	inventory, err := web.NewClient(provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	notValid := []string{}
	var resource interface{}
	switch provider.Type() {
	case api.OpenShift:
		resource = &ocp.StorageClass{}
	case api.VSphere:
		return
	default:
		err = liberr.Wrap(
			web.ProviderNotSupportedError{
				Provider: provider,
			})
		return
	}
	for _, entry := range list {
		name := entry.Destination.StorageClass
		pErr := inventory.Get(resource, name)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				notValid = append(notValid, entry.Destination.StorageClass)
			} else {
				err = liberr.Wrap(pErr)
				return
			}
		}
	}
	if len(notValid) > 0 {
		result.SetCondition(libcnd.Condition{
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
