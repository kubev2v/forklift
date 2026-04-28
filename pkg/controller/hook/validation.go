package hook

import (
	"encoding/base64"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libaap "github.com/kubev2v/forklift/pkg/lib/aap"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	InvalidImage       = "InvalidImage"
	InvalidPlaybook    = "InvalidPlaybook"
	InvalidHookExecute = "InvalidHookExecute"
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
	NotFound = "NotFound"
	DataErr  = "DataError"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

// Validate the hook.
func (r *Reconciler) validate(hook *api.Hook) (err error) {
	if hook.Spec.AAP != nil {
		if strings.TrimSpace(hook.Spec.Image) != "" || strings.TrimSpace(hook.Spec.Playbook) != "" {
			hook.Status.SetCondition(libcnd.Condition{
				Type:     InvalidHookExecute,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "AAP hooks cannot be combined with local `spec.image` or `spec.playbook`; clear those fields or use a local hook instead.",
			})
			return nil
		}
		r.validateAAP(hook)
		return nil
	}

	if !api.HookExecutionConfigValid(hook) {
		hook.Status.SetCondition(libcnd.Condition{
			Type:     InvalidHookExecute,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "Local hooks require `image`; `playbook` is optional. Use `spec.aap` for AAP job templates.",
		})
		return nil
	}

	r.validateImage(hook)
	if hook.Spec.Playbook != "" {
		r.validatePlaybook(hook)
	}
	return nil
}

// validateAAP checks AAP hook configuration beyond CRD admission (Secret content is validated at runtime).
func (r *Reconciler) validateAAP(hook *api.Hook) {
	a := hook.Spec.AAP
	if a == nil {
		return
	}
	if a.JobTemplateID <= 0 {
		hook.Status.SetCondition(libcnd.Condition{
			Type:     InvalidHookExecute,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "AAP hooks require jobTemplateId > 0.",
		})
		return
	}
	if !libaap.HookAAPRunnableFromMigrationSettings(hook) {
		hook.Status.SetCondition(libcnd.Condition{
			Type:     InvalidHookExecute,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "AAP hooks require ForkliftController aap_url and aap_token_secret_name, or spec.aap.url and spec.aap.tokenSecret.name for per-hook connection.",
		})
	}
}

func (r *Reconciler) validateImage(hook *api.Hook) {
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
}

func (r *Reconciler) validatePlaybook(hook *api.Hook) {
	if hook.Spec.Playbook == "" {
		return
	}
	if _, dErr := base64.StdEncoding.DecodeString(hook.Spec.Playbook); dErr != nil {
		hook.Status.SetCondition(libcnd.Condition{
			Type:     InvalidPlaybook,
			Status:   True,
			Reason:   DataErr,
			Category: Critical,
			Message:  "`Playbook` should contain a base64 encoded playbook.",
		})
	}
}
