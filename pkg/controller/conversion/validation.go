package conversion

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	TypeNotValid      = "TypeNotValid"
	ProviderNotSet    = "ProviderNotSet"
	VMNotSet          = "VMNotSet"
	DisksNotSet       = "DisksNotSet"
	ConnectionNotSet  = "ConnectionNotSet"
	VDDKImageNotSet   = "VDDKImageNotSet"
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
	NotSet   = "NotSet"
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
	err = r.validateDisks(conversion)
	if err != nil {
		return
	}
	err = r.validateConnection(conversion)
	if err != nil {
		return
	}
	err = r.validateVDDKImage(conversion)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) validateVDDKImage(conversion *api.Conversion) (err error) {
	if conversion.Spec.Type != api.DeepInspection {
		return
	}
	if conversion.Spec.VDDKImage == "" {
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     VDDKImageNotSet,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `vddkImage` field is required when `type` is DeepInspection.",
		})
	}
	return
}

func (r *Reconciler) validateType(conversion *api.Conversion) (err error) {
	switch conversion.Spec.Type {
	case api.DeepInspection, api.Inspection, api.InPlace, api.Remote:
	default:
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     TypeNotValid,
			Status:   True,
			Reason:   NotValid,
			Category: Critical,
			Message:  "The `Type` must be one of: DeepInspection, Inspection, InPlace, Remote.",
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

func (r *Reconciler) validateDisks(conversion *api.Conversion) (err error) {
	switch conversion.Spec.Type {
	case api.InPlace, api.Remote:
		if len(conversion.Spec.Disks) == 0 {
			conversion.Status.SetCondition(libcnd.Condition{
				Type:     DisksNotSet,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "The `Disks` field is required for this conversion type.",
			})
		}
	}
	return
}

func (r *Reconciler) validateConnection(conversion *api.Conversion) (err error) {
	if conversion.Spec.Connection.Secret.Name == "" {
		conversion.Status.SetCondition(libcnd.Condition{
			Type:     ConnectionNotSet,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `Connection.Secret` is required.",
		})
	}
	return
}
