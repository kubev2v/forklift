package v1beta1

import "strings"

// HookExecutionConfigValid reports whether the Hook spec is sufficient to run:
// either a complete AAP configuration, or a local hook with a non-empty container image.
// For local hooks, spec.playbook is optional (image default CMD may be used).
func HookExecutionConfigValid(hook *Hook) bool {
	if hook == nil {
		return false
	}
	if hook.Spec.AAP != nil {
		a := hook.Spec.AAP
		return strings.TrimSpace(a.URL) != "" && a.JobTemplateID > 0 && strings.TrimSpace(a.TokenSecret.Name) != ""
	}
	return strings.TrimSpace(hook.Spec.Image) != ""
}
