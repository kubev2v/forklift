package validation

import (
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/mapped"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	SourceNetworkNotValid      = "SourceNetworkNotValid"
	DestinationNetworkNotValid = "DestinationNetworkNotValid"
)

//
// Reasons
const (
	Ambiguous = "Ambiguous"
)

//
// Network types.
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
		return
	}
	result.UpdateConditions(conditions)
	conditions, err = r.validateDestination(list)
	if err != nil {
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
		return
	}
	notValid := []string{}
	ambiguous := []string{}
	for i := range list {
		ref := &list[i].Source
		if ref.NotSet() {
			result.SetCondition(libcnd.Condition{
				Type:     SourceNetworkNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "Source network: either `ID` or `Name` required.",
			})
			continue
		}
		_, pErr := inventory.Network(ref)
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
	}
	if len(notValid) > 0 {
		result.SetCondition(libcnd.Condition{
			Type:     SourceNetworkNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Source network not found.",
			Items:    notValid,
		})
	}
	if len(ambiguous) > 0 {
		result.SetCondition(libcnd.Condition{
			Type:     SourceNetworkNotValid,
			Status:   True,
			Reason:   Ambiguous,
			Category: Critical,
			Message:  "Source network has ambiguous ref.",
			Items:    ambiguous,
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
		return
	}
	notFound := []string{}
	var resource interface{}
	switch provider.Type() {
	case api.OpenShift:
		resource = &ocp.NetworkAttachmentDefinition{}
	case api.VSphere:
		return
	default:
		err = liberr.Wrap(
			web.ProviderNotSupportedError{
				Provider: provider,
			})
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
			pErr := inventory.Get(resource, id)
			if pErr != nil {
				if errors.As(pErr, &web.NotFoundError{}) {
					notFound = append(notFound, entry.Source.ID)
				} else {
					err = pErr
					return
				}
			}
		}
	}
	if len(notFound) > 0 {
		result.SetCondition(libcnd.Condition{
			Type:     DestinationNetworkNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Destination network not found.",
			Items:    notFound,
		})
	}

	return
}
