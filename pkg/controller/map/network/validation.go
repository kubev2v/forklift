package network

import (
	"errors"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/validation"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	SourceNetworkNotValid      = "SourceNetworkNotValid"
	DestinationNetworkNotValid = "DestinationNetworkNotValid"
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

// Network types.
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

// Validate the mp resource.
func (r *Reconciler) validate(mp *api.NetworkMap) error {
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
func (r *Reconciler) validateSource(mp *api.NetworkMap) (err error) {
	provider := mp.Provider.Source
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
		references.List = append(references.List, *ref)
	}
	mp.Status.Refs = references
	if len(notValid) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     SourceNetworkNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Source network not found.",
			Items:    notValid,
		})
	}
	if len(ambiguous) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
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

// Validate destination refs.
func (r *Reconciler) validateDestination(mp *api.NetworkMap) (err error) {
	provider := mp.Referenced.Provider.Destination
	inventory, err := web.NewClient(provider)
	if err != nil {
		return
	}
	list := mp.Spec.Map
	notFound := []string{}
	ambiguous := []string{}
next:
	for _, entry := range list {
		switch entry.Destination.Type {
		case Ignored, Pod:
			continue next
		case Multus:
			if entry.Destination.Namespace == "" {
				ambiguous = append(
					ambiguous,
					path.Join(
						entry.Destination.Namespace,
						entry.Destination.Name))
				continue
			}
			id := path.Join(
				entry.Destination.Namespace,
				entry.Destination.Name)
			_, pErr := inventory.Network(&refapi.Ref{Name: id})
			if pErr != nil {
				if errors.As(pErr, &web.NotFoundError{}) {
					notFound = append(
						notFound,
						path.Join(
							entry.Destination.Namespace,
							entry.Destination.Name))
				} else {
					err = pErr
					return
				}
			}
		}
	}
	if len(notFound) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     DestinationNetworkNotValid,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Destination network (NAD) not found.",
			Items:    notFound,
		})
	}
	if len(ambiguous) > 0 {
		mp.Status.SetCondition(libcnd.Condition{
			Type:     DestinationNetworkNotValid,
			Status:   True,
			Reason:   Ambiguous,
			Category: Critical,
			Message:  "Destination network (NAD) namespace required.",
			Items:    ambiguous,
		})
	}

	return
}
