package v1beta1

import "strings"

// HookExecutionConfigValid reports whether the Hook spec is sufficient to run:
// either a complete AAP configuration, or a local hook with a non-empty container image.
// For local hooks, spec.playbook is optional (image default CMD may be used).
// spec.aap and local image/playbook are mutually exclusive.
func HookExecutionConfigValid(hook *Hook) bool {
	if hook == nil {
		return false
	}
	hasLocalFields := strings.TrimSpace(hook.Spec.Image) != "" ||
		strings.TrimSpace(hook.Spec.Playbook) != ""
	if hook.Spec.AAP != nil {
		if hasLocalFields {
			return false
		}
		a := hook.Spec.AAP
		return strings.TrimSpace(a.URL) != "" && a.JobTemplateID > 0 && strings.TrimSpace(a.TokenSecret.Name) != ""
	}
	return strings.TrimSpace(hook.Spec.Image) != ""
}
