package v1beta1

import "strings"

// HookExecutionConfigValid reports whether the Hook spec is sufficient to run:
// either an AAP hook with jobTemplateId > 0 (cluster or per-hook connection is validated separately), or a local hook with a non-empty container image.
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
		return a.JobTemplateID > 0
	}
	return strings.TrimSpace(hook.Spec.Image) != ""
}

// HookAAPRunnable returns true when an AAP hook has a job template and either:
//   - per-hook connection: non-empty spec.aap.url and spec.aap.tokenSecret.name, or
//   - cluster connection: non-empty ForkliftController aap_url and aap_token_secret_name.
func HookAAPRunnable(hook *Hook, clusterAAPURL, clusterAAPTokenSecretName string) bool {
	if hook == nil || hook.Spec.AAP == nil {
		return true
	}
	a := hook.Spec.AAP
	if a.JobTemplateID <= 0 {
		return false
	}
	hookConn := strings.TrimSpace(a.URL) != "" && a.TokenSecret != nil && strings.TrimSpace(a.TokenSecret.Name) != ""
	clusterConn := strings.TrimSpace(clusterAAPURL) != "" && strings.TrimSpace(clusterAAPTokenSecretName) != ""
	return hookConn || clusterConn
}
