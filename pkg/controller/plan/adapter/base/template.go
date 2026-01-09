package base

import (
	"fmt"
	"strings"

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
