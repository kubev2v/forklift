package validation

import (
	"context"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/provider"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	ProviderNotValid            = "ProviderNotValid"
	ProviderNotReady            = "ProviderNotReady"
	SourceProviderNotValid      = "SourceProviderNotValid"
	SourceProviderNotReady      = "SourceProviderNotReady"
	DestinationProviderNotValid = "DestinationProviderNotValid"
	DestinationProviderNotReady = "DestinationProviderNotReady"
)

//
// Categories
const (
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

// Reasons
const (
	NotSet       = "NotSet"
	NotFound     = "NotFound"
	TypeNotValid = "TypeNotValid"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Referenced Provider validation.
type Provider struct {
	client.Client
	// Found and populated by Validate().
	Referenced *api.Provider
}

//
// Validate a provider.
func (r *Provider) Validate(ref core.ObjectReference) (result libcnd.Conditions, err error) {
	newCnd := libcnd.Condition{
		Type:     ProviderNotValid,
		Status:   True,
		Category: Critical,
		Message:  "The provider is not valid.",
	}
	if !libref.RefSet(&ref) {
		newCnd.Reason = NotSet
		result.SetCondition(newCnd)
		return
	}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	provider := api.Provider{}
	err = r.Get(context.TODO(), key, &provider)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		result.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Referenced = &provider

	if !provider.Status.HasCondition(libcnd.Ready) {
		result.SetCondition(libcnd.Condition{
			Type:     ProviderNotReady,
			Status:   True,
			Category: Critical,
			Message:  "The provider does not have the Ready condition.",
		})
	}

	return
}

//
// ProviderPair
type ProviderPair struct {
	client.Client
	// Found and populated by Validate().
	Referenced struct {
		Source      *api.Provider
		Destination *api.Provider
	}
}

//
// Validate the pair.
func (r *ProviderPair) Validate(pair provider.Pair) (result libcnd.Conditions, err error) {
	conditions, err := r.validateSource(pair)
	if err != nil {
		return
	}
	result.UpdateConditions(conditions)
	conditions, err = r.validateDestination(pair)
	if err != nil {
		return
	}
	result.UpdateConditions(conditions)

	return
}

//
// Validate the source.
func (r *ProviderPair) validateSource(pair provider.Pair) (result libcnd.Conditions, err error) {
	pv := Provider{Client: r.Client}
	conditions, err := pv.Validate(pair.Source)
	if err != nil {
		return
	}
	// Remap the condition to be source oriented.
	for _, newCnd := range conditions.List {
		switch newCnd.Type {
		case ProviderNotValid:
			newCnd.Type = SourceProviderNotValid
			newCnd.Message = "The source provider is not valid."
		case ProviderNotReady:
			newCnd.Type = SourceProviderNotReady
			newCnd.Message = "The source provider does not have the Ready condition."
		default:
			err = liberr.New("unknown")
			return
		}
		result.SetCondition(newCnd)
	}
	// An openshift source is not supported.
	r.Referenced.Source = pv.Referenced
	if r.Referenced.Source != nil && r.Referenced.Source.Type() == api.OpenShift {
		result.SetCondition(libcnd.Condition{
			Type:     SourceProviderNotValid,
			Status:   True,
			Reason:   TypeNotValid,
			Category: Critical,
			Message:  "The provider is not valid.",
		})
		return
	}

	return
}

//
// Validate the destination.
func (r *ProviderPair) validateDestination(pair provider.Pair) (result libcnd.Conditions, err error) {
	pv := Provider{Client: r.Client}
	conditions, err := pv.Validate(pair.Destination)
	if err != nil {
		return
	}
	// Remap the condition to be destination oriented.
	for _, newCnd := range conditions.List {
		switch newCnd.Type {
		case ProviderNotValid:
			newCnd.Type = DestinationProviderNotValid
			newCnd.Message = "The destination provider is not valid."
		case ProviderNotReady:
			newCnd.Type = DestinationProviderNotReady
			newCnd.Message = "The destination provider does not have the Ready condition."
		default:
			err = liberr.New("unknown")
			return
		}
		result.SetCondition(newCnd)
	}
	// A non-openshift destination is not supported.
	r.Referenced.Destination = pv.Referenced
	if r.Referenced.Destination != nil && r.Referenced.Destination.Type() != api.OpenShift {
		result.SetCondition(libcnd.Condition{
			Type:     DestinationProviderNotValid,
			Status:   True,
			Reason:   TypeNotValid,
			Category: Critical,
			Message:  "The destination provider is not valid.",
		})
		return
	}

	return
}
