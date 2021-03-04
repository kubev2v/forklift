package hook

import (
	"encoding/base64"
	libcnd "github.com/konveyor/controller/pkg/condition"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
)

//
// Types
const (
	InvalidImage    = "InvalidImage"
	InvalidPlaybook = "InvalidPlaybook"
)

//
// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

//
// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
	DataErr  = "DataError"
)

//
// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

//
// Validate the hook.
func (r *Reconciler) validate(hook *api.Hook) (err error) {
	err = r.validateImage(hook)
	if err != nil {
		return
	}
	err = r.validatePlaybook(hook)
	if err != nil {
		return
	}
	return
}

//
// Validate the hook.
func (r *Reconciler) validateImage(hook *api.Hook) (err error) {
	match := ReferenceRegexp.MatchString(hook.Spec.Image)
	if !match {
		hook.Status.SetCondition(libcnd.Condition{
			Type:     InvalidImage,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The image name specified in `Image` is invalid.",
		})
	}

	return
}

func (r Reconciler) validatePlaybook(hook *api.Hook) (err error) {
	if _, dErr := base64.StdEncoding.DecodeString(hook.Spec.Playbook); dErr != nil {
		hook.Status.SetCondition(libcnd.Condition{
			Type:     InvalidPlaybook,
			Status:   True,
			Reason:   DataErr,
			Category: Critical,
			Message:  "`Playbook` should contain a base64 encoded playbook.",
		})
	}

	return
}
