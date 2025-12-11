package plan

import (
	"fmt"
	"strings"

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
	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION ENTRY",
		"plan", plan.Name,
		"namespace", plan.Namespace)

	// Check source provider for VIB readiness issues
	sourceProvider := plan.Referenced.Provider.Source
	if sourceProvider == nil {
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION SKIPPED - No source provider",
			"plan", plan.Name)
		return nil // This would be caught by other validation
	}

	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - Source provider found",
		"plan", plan.Name,
		"sourceProvider", sourceProvider.Name,
		"providerType", sourceProvider.Type())

	// Only check vSphere providers with VIB method enabled
	if sourceProvider.Type() != api.VSphere {
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION SKIPPED - Not vSphere provider",
			"plan", plan.Name,
			"providerType", sourceProvider.Type())
		return nil
	}

	// Only validate VIB readiness for plans using VSphere xcopy volume populators
	usesXcopy := r.planUsesVSphereXcopyPopulator(plan)
	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - Xcopy check",
		"plan", plan.Name,
		"usesXcopy", usesXcopy)

	if !usesXcopy {
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION SKIPPED - Not using xcopy populator",
			"plan", plan.Name)
		// Remove any existing VIB readiness conditions since xcopy is not used
		plan.Status.DeleteCondition(VIBReady)
		plan.Status.DeleteCondition(VIBNotReady)
		return nil
	}

	// Check ESXiCloneMethod setting - VIB is the default when not set or explicitly set to "vib"
	esxiCloneMethod, methodSet := sourceProvider.Spec.Settings[api.ESXiCloneMethod]
	useVIBMethod := !methodSet || esxiCloneMethod != api.ESXiCloneMethodSSH

	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - Clone method check",
		"plan", plan.Name,
		"esxiCloneMethod", esxiCloneMethod,
		"methodSet", methodSet,
		"useVIBMethod", useVIBMethod)

	if !useVIBMethod {
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION SKIPPED - SSH method in use",
			"plan", plan.Name,
			"esxiCloneMethod", esxiCloneMethod)
		// Remove any existing VIB readiness conditions since VIB method is not enabled
		plan.Status.DeleteCondition(VIBReady)
		plan.Status.DeleteCondition(VIBNotReady)
		return nil
	}

	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION STARTING - Checking provider conditions",
		"plan", plan.Name,
		"sourceProvider", sourceProvider.Name)

	// Check for provider VIB ready condition (advisory - hosts that passed validation)
	vibReadyCondition := sourceProvider.Status.FindCondition(VIBReady)
	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - VIBReady condition check",
		"plan", plan.Name,
		"hasVIBReadyCondition", vibReadyCondition != nil,
		"conditionStatus", func() string {
			if vibReadyCondition != nil {
				return string(vibReadyCondition.Status)
			}
			return "N/A"
		}(),
		"itemCount", func() int {
			if vibReadyCondition != nil {
				return len(vibReadyCondition.Items)
			}
			return 0
		}())

	if vibReadyCondition != nil && vibReadyCondition.Status == libcnd.True && len(vibReadyCondition.Items) > 0 {
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - Setting VIBReady condition (propagating from provider)",
			"plan", plan.Name,
			"sourceProvider", sourceProvider.Name,
			"itemCount", len(vibReadyCondition.Items),
			"items", vibReadyCondition.Items)

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
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - No VIBReady condition to propagate",
			"plan", plan.Name,
			"sourceProvider", sourceProvider.Name)
		plan.Status.DeleteCondition(VIBReady)
	}

	// Check for provider VIB not ready condition (warning - hosts that failed validation)
	vibNotReadyCondition := sourceProvider.Status.FindCondition(VIBNotReady)
	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - VIBNotReady condition check",
		"plan", plan.Name,
		"hasVIBNotReadyCondition", vibNotReadyCondition != nil,
		"conditionStatus", func() string {
			if vibNotReadyCondition != nil {
				return string(vibNotReadyCondition.Status)
			}
			return "N/A"
		}(),
		"itemCount", func() int {
			if vibNotReadyCondition != nil {
				return len(vibNotReadyCondition.Items)
			}
			return 0
		}())

	if vibNotReadyCondition != nil && vibNotReadyCondition.Status == libcnd.True && len(vibNotReadyCondition.Items) > 0 {
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - Setting VIBNotReady condition (propagating from provider)",
			"plan", plan.Name,
			"sourceProvider", sourceProvider.Name,
			"itemCount", len(vibNotReadyCondition.Items),
			"items", vibNotReadyCondition.Items)

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
		r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION - No VIBNotReady condition to propagate",
			"plan", plan.Name,
			"sourceProvider", sourceProvider.Name)
		plan.Status.DeleteCondition(VIBNotReady)
	}

	r.Log.Info(">>>>>>>>>>>>>>>>>>>>>> DEBUG: PLAN VIB VALIDATION COMPLETE - Conditions propagated",
		"plan", plan.Name,
		"sourceProvider", sourceProvider.Name)

	return nil
}

// formatVIBHostItems transforms Provider host items from "id|name|ip" format to Plan format "id:host-123 ip:10.0.0.1"
func formatVIBHostItems(providerItems []string) []string {
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
