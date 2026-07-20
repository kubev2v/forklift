package base

import (
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/templateutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// DefaultPVCNameTemplate is the hardcoded default PVC name template for non-OCP providers.
// Uses trunc 15 for plan and VM names to keep derived resource names under the 63-char DNS1123 limit.
const DefaultPVCNameTemplate = "{{trunc 15 .PlanName}}-{{trunc 15 .TargetVmName}}-disk-{{.DiskIndex}}"

// DefaultOCPPVCNameTemplate is the hardcoded default PVC name template for OpenShift sources.
// With the OCP UseGenerateName default of false, this preserves the source PVC name as an exact Name.
const DefaultOCPPVCNameTemplate = "{{.SourcePVCName}}"

// ValidateAndExecuteTemplate executes a template with the provided data and validates
// that the output is non-empty. Returns the trimmed result string or an error.
// This is a shared utility for PVC name template validation across all providers.
func ValidateAndExecuteTemplate(templateStr string, testData interface{}) (string, error) {
	// Execute the template with test data
	result, err := templateutil.ExecuteTemplate(templateStr, testData)
	if err != nil {
		return "", liberr.Wrap(err, "template", templateStr)
	}

	// Trim whitespace from the result
	result = strings.TrimSpace(result)

	// Empty output is not valid
	if result == "" {
		return "", liberr.New("Template output is empty", "template", templateStr)
	}

	return result, nil
}

// ValidatePVCNameTemplateOutput validates that a template output string is a valid
// Kubernetes DNS1123 label (required for PVC names).
// Returns an error if the output is not valid.
func ValidatePVCNameTemplateOutput(result string) error {
	errs := k8svalidation.IsDNS1123Label(result)
	if len(errs) > 0 {
		errMsg := fmt.Sprintf("Template output is invalid k8s label [%s]", result)
		return liberr.New(errMsg, errs)
	}
	return nil
}

// ValidatePVCNameTemplate is a convenience function that combines template execution
// and k8s label validation. It executes the template with the provided data and
// validates that the output is a valid DNS1123 label.
// Returns the validated result string or an error.
func ValidatePVCNameTemplate(templateStr string, testData interface{}) (string, error) {
	result, err := ValidateAndExecuteTemplate(templateStr, testData)
	if err != nil {
		return "", err
	}

	if err := ValidatePVCNameTemplateOutput(result); err != nil {
		return "", err
	}

	return result, nil
}

// SetPVCNameOnObject executes the PVC name template, validates the output, and sets
// either Name or GenerateName on the ObjectMeta depending on useGenerateName.
// This is the single entry point for applying PVC name templates to any Kubernetes object.
func SetPVCNameOnObject(objectMeta *metav1.ObjectMeta, templateStr string, useGenerateName bool, templateData interface{}) error {
	result, err := ValidatePVCNameTemplate(templateStr, templateData)
	if err != nil {
		return err
	}

	if useGenerateName {
		if !strings.HasSuffix(result, "-") {
			result = result + "-"
		}
		objectMeta.GenerateName = result
		objectMeta.Name = ""
	} else {
		objectMeta.Name = strings.TrimSuffix(result, "-")
		objectMeta.GenerateName = ""
	}
	return nil
}

// ResolveTargetVmName returns the DNS1123-safe target VM name. If the plan has
// assigned a NewName (because the original was not DNS1123-compliant), that is
// returned; otherwise vmName is returned as-is.
func ResolveTargetVmName(p *api.Plan, vmID, vmName string) string {
	if p != nil && p.Status.Migration.VMs != nil {
		for _, vmStatus := range p.Status.Migration.VMs {
			if vmStatus.ID == vmID && vmStatus.NewName != "" {
				return vmStatus.NewName
			}
		}
	}
	return vmName
}

// isOCPSource reports whether the plan's resolved source provider is OpenShift.
func isOCPSource(p *api.Plan) bool {
	if p == nil || p.Provider.Source == nil {
		return false
	}
	return p.Provider.Source.Type() == api.OpenShift
}

// GetPVCNameTemplate returns the PVC name template for the given VM ID.
// Resolution: VM → Plan → provider-specific ForkliftController setting → hardcoded default.
func GetPVCNameTemplate(p *api.Plan, vmID string) string {
	if p != nil {
		for i := range p.Spec.VMs {
			vm := &p.Spec.VMs[i]
			if vm.ID == vmID && vm.PVCNameTemplate != "" {
				return vm.PVCNameTemplate
			}
		}
		if p.Spec.PVCNameTemplate != "" {
			return p.Spec.PVCNameTemplate
		}
	}
	if isOCPSource(p) {
		if settings.Settings.OCPPVCNameTemplate != "" {
			return settings.Settings.OCPPVCNameTemplate
		}
		return DefaultOCPPVCNameTemplate
	}
	if settings.Settings.PVCNameTemplate != "" {
		return settings.Settings.PVCNameTemplate
	}
	return DefaultPVCNameTemplate
}

// GetPVCNameTemplateUseGenerateName returns whether generateName should be used.
// Explicit Plan values always win. When unset: false for OCP, true for other providers.
func GetPVCNameTemplateUseGenerateName(p *api.Plan) bool {
	if p != nil && p.Spec.PVCNameTemplateUseGenerateName != nil {
		return *p.Spec.PVCNameTemplateUseGenerateName
	}
	return !isOCPSource(p)
}
