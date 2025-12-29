package plan

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
)

// Types
const (
	VIBReady    = "VIBReady"
	VIBNotReady = "VIBNotReady"
)

// validateVIBReadiness validates VIB readiness for migration plans using xcopy volume populators
func (r *Reconciler) validateVIBReadiness(plan *api.Plan) error {
	// Check if provider reference exists
	if plan.Referenced.Provider.Source == nil {
		return nil
	}
	sourceProvider := plan.Referenced.Provider.Source

	// Only check vSphere providers with VIB method enabled
	if sourceProvider.Type() != api.VSphere {
		return nil
	}

	// Only validate VIB readiness for plans using VSphere xcopy volume populators and VIB method enabled
	usesXcopy := r.planUsesVSphereXcopyPopulator(plan)
	if !usesXcopy || !sourceProvider.UseVIBMethod() {
		// Remove any existing VIB readiness conditions since xcopy is not used
		plan.Status.DeleteCondition(VIBReady)
		plan.Status.DeleteCondition(VIBNotReady)
		return nil
	}

	// Check for provider VIB ready condition (advisory - hosts that passed validation)
	vibReadyCondition := sourceProvider.Status.FindCondition(VIBReady)
	if vibReadyCondition != nil && vibReadyCondition.Status == libcnd.True && len(vibReadyCondition.Items) > 0 {
		suggestion := fmt.Sprintf("Provider '%s' has ESXi hosts with VIB (vmkfstools-wrapper) validated. ", sourceProvider.Name)
		suggestion += vibReadyCondition.Suggestion

		plan.Status.SetCondition(libcnd.Condition{
			Type:       VIBReady,
			Status:     libcnd.True,
			Reason:     "ProviderVIBReady",
			Category:   libcnd.Advisory,
			Message:    "VIB (vmkfstools-wrapper) validated on ESXi hosts (checked because 'esxiCloneMethod' is not set to 'ssh' on the source provider). See the suggestion field in the Plan's YAML for the list of available ESXi hosts.",
			Suggestion: suggestion,
			Items:      formatVIBHostItems(vibReadyCondition.Items),
		})
	} else {
		plan.Status.DeleteCondition(VIBReady)
	}

	// Check for provider VIB not ready condition (warning - hosts that failed validation)
	vibNotReadyCondition := sourceProvider.Status.FindCondition(VIBNotReady)
	if vibNotReadyCondition != nil && vibNotReadyCondition.Status == libcnd.True && len(vibNotReadyCondition.Items) > 0 {
		suggestion := fmt.Sprintf("Migration plan uses xcopy volume populator with provider '%s' that has VIB (vmkfstools-wrapper) issues. ", sourceProvider.Name)
		suggestion += vibNotReadyCondition.Suggestion

		plan.Status.SetCondition(libcnd.Condition{
			Type:       VIBNotReady,
			Status:     libcnd.True,
			Reason:     "ProviderVIBNotReady",
			Category:   libcnd.Warn,
			Message:    "VIB readiness validation issue (checked because 'esxiCloneMethod' is not set to 'ssh' on the source provider). See the suggestion field in the Plan's YAML for details.",
			Suggestion: suggestion,
			Items:      formatVIBHostItems(vibNotReadyCondition.Items),
		})
	} else {
		plan.Status.DeleteCondition(VIBNotReady)
	}

	return nil
}

// formatVIBHostItems transforms Provider host items from "id|name|ip" format to Plan format "id:host-123 ip:10.0.0.1"
func formatVIBHostItems(providerItems []string) []string {
	result := make([]string, 0, len(providerItems))
	result = append(result, providerItems...)
	return result
}

