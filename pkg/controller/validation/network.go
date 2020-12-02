package validation

import (
	"fmt"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/mapped"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"net/http"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	SourceNetworkNotValid      = "SourceNetworkNotValid"
	DestinationNetworkNotValid = "DestinationNetworkNotValid"
	NetworkTypeNotValid        = "NetworkTypeNotValid"
)

const (
	Pod    = "pod"
	Multus = "multus"
)

//
// Network pair validation.
type NetworkPair struct {
	client.Client
	Provider struct {
		Source      *api.Provider
		Destination *api.Provider
	}
}

//
// Validate pairs.
func (r *NetworkPair) Validate(list []mapped.NetworkPair) (result libcnd.Conditions, err error) {
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
// Validate source networks.
func (r *NetworkPair) validateSource(list []mapped.NetworkPair) (result libcnd.Conditions, err error) {
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
	var resource interface{}
	switch provider.Type() {
	case api.OpenShift:
		return
	case api.VSphere:
		resource = &vsphere.Network{}
	default:
		err = liberr.Wrap(web.ProviderNotSupportedErr)
		return
	}
	for _, entry := range list {
		status, pErr := inventory.Get(resource, entry.Source.ID)
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
		result.SetCondition(libcnd.Condition{
			Type:     SourceNetworkNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Source network not valid.",
		})
	}

	return
}

//
// Validate destination networks.
func (r *NetworkPair) validateDestination(list []mapped.NetworkPair) (result libcnd.Conditions, err error) {
	provider := r.Provider.Destination
	if provider == nil {
		return
	}
	inventory, err := web.NewClient(provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	notFound := []string{}
	notValid := []string{}
	var resource interface{}
	switch provider.Type() {
	case api.OpenShift:
		resource = &ocp.NetworkAttachmentDefinition{}
	case api.VSphere:
		return
	default:
		err = liberr.Wrap(web.ProviderNotSupportedErr)
		return
	}
next:
	for _, entry := range list {
		switch entry.Destination.Type {
		case Pod:
			continue next
		case Multus:
			id := path.Join(
				entry.Destination.Namespace,
				entry.Destination.Name)
			status, pErr := inventory.Get(resource, id)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			switch status {
			case http.StatusOK:
			case http.StatusNotFound:
				notFound = append(notFound, entry.Source.ID)
			default:
				err = liberr.New(http.StatusText(status))
				return
			}
		default:
			notValid = append(notValid, entry.Source.ID)
		}
	}
	if len(notFound) > 0 {
		result.SetCondition(libcnd.Condition{
			Type:     DestinationNetworkNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Destination network not found.",
		})
	}
	if len(notValid) > 0 {
		valid := []string{
			Pod,
			Multus,
		}
		result.SetCondition(libcnd.Condition{
			Type:     NetworkTypeNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  fmt.Sprintf("Network `type` must be: %s.", valid),
		})
	}

	return
}
