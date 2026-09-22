package base

import (
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/templateutil"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

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

// ResolveTargetVmName returns the DNS1123-safe target VM name.
// Resolution order (first non-empty wins):
//  1. spec.vms[].targetName  — explicit user override
//  2. status.migration.vms[].newName — assigned during a previous migration run
//  3. util.ChangeVmName(vmName) — automatic sanitization when vmName is not a valid DNS1123 label
//  4. vmName as-is
func ResolveTargetVmName(p *api.Plan, vmID, vmName string) string {
	if name := planOverrideName(p, vmID); name != "" {
		return name
	}
	if errs := k8svalidation.IsDNS1123Label(vmName); len(errs) > 0 {
		if hasAlphanumeric(vmName) {
			return util.ChangeVmName(vmName)
		}
		return vmIDFallbackName(vmID)
	}
	return vmName
}

// planOverrideName checks the plan for an explicit target name (spec.targetName
// or status.newName) for the given vmID.
func planOverrideName(p *api.Plan, vmID string) string {
	if p == nil {
		return ""
	}
	for i := range p.Spec.VMs {
		if p.Spec.VMs[i].ID == vmID {
			if name := strings.TrimSpace(p.Spec.VMs[i].TargetName); name != "" {
				return name
			}
			break
		}
	}
	for _, vmStatus := range p.Status.Migration.VMs {
		if vmStatus.ID == vmID && vmStatus.NewName != "" {
			return vmStatus.NewName
		}
	}
	return ""
}

// hasAlphanumeric reports whether s contains at least one ASCII letter or digit.
// When false, ChangeVmName would strip all characters and produce a random suffix.
func hasAlphanumeric(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return true
		}
	}
	return false
}

// vmIDFallbackName produces a deterministic DNS1123-safe name from a VM ID.
func vmIDFallbackName(vmID string) string {
	if hasAlphanumeric(vmID) {
		return util.ChangeVmName(vmID)
	}
	return "vm-0000"
}
