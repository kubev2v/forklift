package plan

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	SSHReadiness = "SSHReadiness"
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
		plan.Status.DeleteCondition(SSHReadiness)
		return nil
	}

	// Only check when ESXiCloneMethod is set to "ssh"
	esxiCloneMethod, methodSet := sourceProvider.Spec.Settings[api.ESXiCloneMethod]
	if !methodSet || esxiCloneMethod != "ssh" {
		// Remove any existing SSH readiness conditions since SSH method is not enabled
		plan.Status.DeleteCondition(SSHReadiness)
		return nil
	}

	// Check if the source provider has SSH readiness conditions
	sshReadinessCondition := sourceProvider.Status.FindCondition("SSHReadiness")
	if sshReadinessCondition != nil && sshReadinessCondition.Status == libcnd.False {
		// Provider has SSH readiness issues, propagate to plan
		message := fmt.Sprintf("Migration plan uses xcopy volume populator with provider '%s' that has SSH connectivity issues. ", sourceProvider.Name)
		message += sshReadinessCondition.Message

		plan.Status.SetCondition(libcnd.Condition{
			Type:     SSHReadiness,
			Status:   libcnd.False,
			Reason:   "ProviderSSHReadinessFailed",
			Category: libcnd.Warn,
			Message:  message,
			Items:    sshReadinessCondition.Items,
		})
	} else {
		// Provider SSH is ready or not checked, remove any existing condition
		plan.Status.DeleteCondition(SSHReadiness)
	}

	return nil
}

// planUsesVSphereXcopyPopulator checks if a plan uses VSphere xcopy volume populators
func (r *Reconciler) planUsesVSphereXcopyPopulator(plan *api.Plan) bool {
	// Check storage mappings for VSphereXcopyPluginConfig
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
