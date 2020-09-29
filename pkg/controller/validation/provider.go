package validation

import (
	"context"
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	ProviderNotValid                  = "ProviderNotValid"
	ProviderSecretNotValid            = "ProviderSecretNotValid"
	SourceProviderNotValid            = "SourceProviderNotValid"
	SourceProviderSecretNotValid      = "SourceProviderSecretNotValid"
	DestinationProviderNotValid       = "DestinationProviderNotValid"
	DestinationProviderSecretNotValid = "DestinationProviderSecretNotValid"
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
	NotSet       = "NotSet"
	NotFound     = "NotFound"
	TypeNotValid = "TypeNotValid"
)

// Statuses
const (
	True  = cnd.True
	False = cnd.False
)

//
// Provider validation.
type Provider struct {
	client.Client
	// Found and populated by Validate().
	Referenced *api.Provider
}

//
// Validate a provider.
func (r *Provider) Validate(ref core.ObjectReference) (result cnd.Conditions, err error) {
	newCnd := cnd.Condition{
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
	newCnd = cnd.Condition{
		Type:     ProviderSecretNotValid,
		Status:   True,
		Category: Critical,
		Message:  "The provider secret is not valid.",
	}
	ref = provider.Spec.Secret
	key = client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	secret := core.Secret{}
	err = r.Get(context.TODO(), key, &secret)
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
func (r *ProviderPair) Validate(pair api.ProviderPair) (result cnd.Conditions, err error) {
	conditions, err := r.validateSource(pair)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	result.SetCondition(conditions.List...)
	conditions, err = r.validateDestination(pair)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	result.SetCondition(conditions.List...)

	return
}

//
// Validate the source.
func (r *ProviderPair) validateSource(pair api.ProviderPair) (result cnd.Conditions, err error) {
	validation := Provider{Client: r.Client}
	conditions, err := validation.Validate(pair.Source)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	// Remap the condition to be source oriented.
	for _, newCnd := range conditions.List {
		switch newCnd.Type {
		case ProviderNotValid:
			newCnd.Type = SourceProviderNotValid
			newCnd.Message = "The source provider is not valid."
		case ProviderSecretNotValid:
			newCnd.Type = SourceProviderSecretNotValid
			newCnd.Message = "The source provider secret is not valid."
		default:
			err = liberr.New("unknown")
			return
		}
		result.SetCondition(newCnd)
	}
	// An openshift source is not supported.
	r.Referenced.Source = validation.Referenced
	if r.Referenced.Source != nil && r.Referenced.Source.Type() == api.OpenShift {
		result.SetCondition(cnd.Condition{
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
func (r *ProviderPair) validateDestination(pair api.ProviderPair) (result cnd.Conditions, err error) {
	validation := Provider{Client: r.Client}
	conditions, err := validation.Validate(pair.Destination)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	// Remap the condition to be destination oriented.
	for _, newCnd := range conditions.List {
		switch newCnd.Type {
		case ProviderNotValid:
			newCnd.Type = DestinationProviderNotValid
			newCnd.Message = "The destination provider is not valid."
		case ProviderSecretNotValid:
			newCnd.Type = DestinationProviderSecretNotValid
			newCnd.Message = "The destination provider secret is not valid."
		default:
			err = liberr.New("unknown")
			return
		}
		result.SetCondition(newCnd)
	}
	// A non-openshift destination is not supported.
	r.Referenced.Destination = validation.Referenced
	if r.Referenced.Destination != nil && r.Referenced.Destination.Type() != api.OpenShift {
		result.SetCondition(cnd.Condition{
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
