package diagnostics

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ExtractVMErrors extracts error information from a single VM's migration status.
// The vmStatus map corresponds to one entry in migration.status.vms[].
func ExtractVMErrors(vmStatus map[string]interface{}) (string, []ConditionEntry, []StepError) {
	vmError, _, _ := unstructured.NestedString(vmStatus, "error", "reasons")
	if vmError == "" {
		// Try alternate path: some versions use a flat error string
		reasons, found, _ := unstructured.NestedSlice(vmStatus, "error", "reasons")
		if found && len(reasons) > 0 {
			for _, r := range reasons {
				if s, ok := r.(string); ok {
					if vmError != "" {
						vmError += "; "
					}
					vmError += s
				}
			}
		}
		if vmError == "" {
			phase, _, _ := unstructured.NestedString(vmStatus, "error", "phase")
			if phase != "" {
				vmError = "Failed at phase: " + phase
			}
		}
	}

	conditions := extractConditions(vmStatus)
	stepErrors := extractStepErrors(vmStatus)

	return vmError, conditions, stepErrors
}

func extractConditions(vmStatus map[string]interface{}) []ConditionEntry {
	rawConditions, found, _ := unstructured.NestedSlice(vmStatus, "conditions")
	if !found {
		return nil
	}

	var entries []ConditionEntry
	for _, c := range rawConditions {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := cm["status"].(string)
		if status == "True" {
			condType, _ := cm["type"].(string)
			if condType == "Ready" {
				continue
			}
		}
		entries = append(entries, ConditionEntry{
			Type:    stringVal(cm, "type"),
			Status:  stringVal(cm, "status"),
			Reason:  stringVal(cm, "reason"),
			Message: stringVal(cm, "message"),
		})
	}
	return entries
}

func extractStepErrors(vmStatus map[string]interface{}) []StepError {
	pipeline, found, _ := unstructured.NestedSlice(vmStatus, "pipeline")
	if !found {
		return nil
	}

	var errors []StepError
	for _, p := range pipeline {
		step, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		stepName, _, _ := unstructured.NestedString(step, "name")
		stepPhase, _, _ := unstructured.NestedString(step, "phase")

		// Check step-level error
		if errMap, found, _ := unstructured.NestedMap(step, "error"); found {
			phase := stepPhase
			if phase == "" || phase == "Running" || phase == "Pending" {
				phase = "Failed"
			}
			errors = append(errors, StepError{
				Step:    stepName,
				Phase:   phase,
				Reason:  joinReasons(errMap),
				Message: joinReasons(errMap),
			})
		}

		// Check task-level errors
		tasks, found, _ := unstructured.NestedSlice(step, "tasks")
		if !found {
			continue
		}
		for _, t := range tasks {
			task, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			if errMap, found, _ := unstructured.NestedMap(task, "error"); found {
				taskName, _, _ := unstructured.NestedString(task, "name")
				phase := stepPhase
				if phase == "" || phase == "Running" || phase == "Pending" {
					phase = "Failed"
				}
				errors = append(errors, StepError{
					Step:    stepName + "/" + taskName,
					Phase:   phase,
					Reason:  stringVal(errMap, "phase"),
					Message: joinReasons(errMap),
				})
			}
		}
	}
	return errors
}

func joinReasons(errMap map[string]interface{}) string {
	reasons, found, _ := unstructured.NestedSlice(errMap, "reasons")
	if !found || len(reasons) == 0 {
		return stringVal(errMap, "reasons")
	}
	var result string
	for _, r := range reasons {
		if s, ok := r.(string); ok {
			if result != "" {
				result += "; "
			}
			result += s
		}
	}
	return result
}

func stringVal(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}
