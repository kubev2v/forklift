package plan

import (
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	SSHReady    = "SSHReady"
	SSHNotReady = "SSHNotReady"
)

// validateSSHReadiness validates SSH readiness for migration plans using xcopy volume populators
func (r *Reconciler) validateSSHReadiness(plan *api.Plan) error {
	// Check source provider for SSH readiness issues
	sourceProvider := plan.Referenced.Provider.Source
	if sourceProvider == nil {
		return nil // This would be caught by other validation
	}

	// Only check vSphere providers with SSH method enabled
	if sourceProvider.Type() != api.VSphere {
		return nil
	}

	// Only validate SSH readiness for plans using VSphere xcopy volume populators
	if !r.planUsesVSphereXcopyPopulator(plan) {
		// Remove any existing SSH readiness conditions since xcopy is not used
		plan.Status.DeleteCondition(SSHReady)
		plan.Status.DeleteCondition(SSHNotReady)
		return nil
	}

	// Only check when ESXiCloneMethod is set to "ssh"
	esxiCloneMethod, methodSet := sourceProvider.Spec.Settings[api.ESXiCloneMethod]
	if !methodSet || esxiCloneMethod != api.ESXiCloneMethodSSH {
		// Remove any existing SSH readiness conditions since SSH method is not enabled
		plan.Status.DeleteCondition(SSHReady)
		plan.Status.DeleteCondition(SSHNotReady)
		return nil
	}

	// Check for provider SSH ready condition (advisory - hosts that passed validation)
	sshReadyCondition := sourceProvider.Status.FindCondition(SSHReady)
	if sshReadyCondition != nil && sshReadyCondition.Status == libcnd.True && len(sshReadyCondition.Items) > 0 {
		suggestion := fmt.Sprintf("Provider '%s' has ESXi hosts with SSH connectivity validated. ", sourceProvider.Name)
		suggestion += sshReadyCondition.Suggestion

		plan.Status.SetCondition(libcnd.Condition{
			Type:       SSHReady,
			Status:     libcnd.True,
			Reason:     "ProviderSSHReady",
			Category:   libcnd.Advisory,
			Message:    "SSH connectivity validated (checked because 'esxiCloneMethod' setting is set to 'ssh' on the source provider). See the suggestion field in the Plan's YAML for the list of available ESXi hosts.",
			Suggestion: suggestion,
			Items:      formatHostItems(sshReadyCondition.Items),
		})
	} else {
		plan.Status.DeleteCondition(SSHReady)
	}

	// Check for provider SSH not ready condition (warning - hosts that failed validation)
	sshNotReadyCondition := sourceProvider.Status.FindCondition(SSHNotReady)
	if sshNotReadyCondition != nil && sshNotReadyCondition.Status == libcnd.True && len(sshNotReadyCondition.Items) > 0 {
		suggestion := fmt.Sprintf("Migration plan uses xcopy volume populator with provider '%s' that has SSH connectivity issues. ", sourceProvider.Name)
		suggestion += sshNotReadyCondition.Suggestion

		plan.Status.SetCondition(libcnd.Condition{
			Type:       SSHNotReady,
			Status:     libcnd.True,
			Reason:     "ProviderSSHNotReady",
			Category:   libcnd.Warn,
			Message:    "SSH readiness validation issue (checked because 'esxiCloneMethod' setting is set to 'ssh' on the source provider). See the suggestion field in the Plan's YAML for details.",
			Suggestion: suggestion,
			Items:      formatHostItems(sshNotReadyCondition.Items),
		})
	} else {
		plan.Status.DeleteCondition(SSHNotReady)
	}

	return nil
}

// planUsesVSphereXcopyPopulator checks if a plan uses VSphere xcopy volume populators
func (r *Reconciler) planUsesVSphereXcopyPopulator(plan *api.Plan) bool {
	// Check storage mappings for VSphereXcopyPluginConfig
	if plan.Referenced.Map.Storage == nil {
		return false
	}
	if plan.Referenced.Map.Storage.Spec.Map == nil {
		return false
	}
	dsMapIn := plan.Referenced.Map.Storage.Spec.Map
	for _, mapping := range dsMapIn {
		if mapping.OffloadPlugin != nil && mapping.OffloadPlugin.VSphereXcopyPluginConfig != nil {
			r.Log.V(2).Info("Plan uses VSphere xcopy volume populator", "plan", plan.Name)
			return true
		}
	}

	r.Log.V(2).Info("Plan does not use VSphere xcopy volume populator", "plan", plan.Name)
	return false
}

// formatHostItems transforms Provider host items from "id|name|ip" format to Plan format "id:host-123 ip:10.0.0.1"
func formatHostItems(providerItems []string) []string {
	result := make([]string, 0, len(providerItems))
	for _, item := range providerItems {
		parts := strings.Split(item, "|")
		if len(parts) == 3 {
			// Format: "id:host-123 ip:10.0.0.1"
			result = append(result, fmt.Sprintf("id:%s ip:%s", parts[0], parts[2]))
		} else {
			// Fallback: keep original
			result = append(result, item)
		}
	}
	return result
}
