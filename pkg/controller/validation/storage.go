package validation

import (
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	"net/http"
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
func (r *StoragePair) Validate(list []api.StoragePair) (result cnd.Conditions, err error) {
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
func (r *StoragePair) validateSource(list []api.StoragePair) (result cnd.Conditions, err error) {
	provider := r.Provider.Source
	if provider == nil {
		return
	}
	pClient, err := web.NewClient(provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	notValid := []string{}
	var resource interface{}
	switch provider.Type() {
	case api.OpenShift:
		return
	case api.VSphere:
		resource = &vsphere.Datastore{}
	default:
		err = liberr.Wrap(web.ProviderNotSupportedErr)
		return
	}
	for _, entry := range list {
		status, pErr := pClient.Get(resource, entry.Source.ID)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		switch status {
		case http.StatusOK:
		case http.StatusNotFound:
			notValid = append(notValid, entry.Source.ID)
		default:
			err = liberr.New(http.StatusText(status))
			return
		}
	}
	if len(notValid) > 0 {
		result.SetCondition(cnd.Condition{
			Type:     SourceStorageNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Source storage not valid.",
			Items:    notValid,
		})
	}

	return
}

//
// Validate destination storage.
func (r *StoragePair) validateDestination(list []api.StoragePair) (result cnd.Conditions, err error) {
	provider := r.Provider.Destination
	if provider == nil {
		return
	}
	pClient, err := web.NewClient(provider)
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
		err = liberr.Wrap(web.ProviderNotSupportedErr)
		return
	}
	for _, entry := range list {
		name := entry.Destination.StorageClass
		status, pErr := pClient.Get(resource, name)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		switch status {
		case http.StatusOK:
		case http.StatusNotFound:
			notValid = append(notValid, entry.Destination.StorageClass)
		default:
			err = liberr.New(http.StatusText(status))
			return
		}
	}
	if len(notValid) > 0 {
		result.SetCondition(cnd.Condition{
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
