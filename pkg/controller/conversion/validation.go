package conversion

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	TypeNotValid    = "TypeNotValid"
	ProviderNotSet  = "ProviderNotSet"
	VMNotSet        = "VMNotSet"
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
	NotSet  = "NotSet"
	NotValid = "NotValid"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

func (r *Reconciler) validate(conversion *api.Conversion) (err error) {
	err = r.validateType(conversion)
	if err != nil {
		return
	}
	err = r.validateProvider(conversion)
	if err != nil {
		return
	}
	err = r.validateVM(conversion)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) validateType(conversion *api.Conversion) (err error) {
	switch conversion.Spec.Type {
	case api.Inspection, api.InPlace, api.Cold:
	default:
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     TypeNotValid,
			Status:   True,
			Reason:   NotValid,
			Category: Critical,
			Message:  "The `Type` must be one of: Inspection, InPlace, Cold.",
		})
	}
	return
}

func (r *Reconciler) validateProvider(conversion *api.Conversion) (err error) {
	if conversion.Spec.Provider.Name == "" {
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     ProviderNotSet,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `Provider` is not set.",
		})
	}
	return
}

func (r *Reconciler) validateVM(conversion *api.Conversion) (err error) {
	if conversion.Spec.VM.NotSet() {
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     VMNotSet,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `VM` reference is not set.",
		})
	}
	return
}
