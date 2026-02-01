package health

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CheckPlansHealth checks the health of all migration plans
func CheckPlansHealth(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string, allNamespaces bool) ([]PlanHealth, error) {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, err
	}

	var plans *unstructured.UnstructuredList
	if allNamespaces {
		plans, err = dynamicClient.Resource(client.PlansGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		ns := namespace
		if ns == "" {
			ns = client.OpenShiftMTVNamespace
		}
		plans, err = dynamicClient.Resource(client.PlansGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}

	var planHealths []PlanHealth
	for _, plan := range plans.Items {
		health := analyzePlan(&plan)
		planHealths = append(planHealths, health)
	}

	return planHealths, nil
}

// analyzePlan analyzes a single plan and returns its health status
func analyzePlan(plan *unstructured.Unstructured) PlanHealth {
	health := PlanHealth{
		Name:      plan.GetName(),
		Namespace: plan.GetNamespace(),
	}

	// Get VM count
	vms, exists, _ := unstructured.NestedSlice(plan.Object, "spec", "vms")
	if exists {
		health.VMCount = len(vms)
	}

	// Extract conditions
	conditions, exists, _ := unstructured.NestedSlice(plan.Object, "status", "conditions")
	if exists {
		for _, c := range conditions {
			condition, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condition, "type")
			condStatus, _, _ := unstructured.NestedString(condition, "status")
			message, _, _ := unstructured.NestedString(condition, "message")

			isTrue := condStatus == "True"

			switch condType {
			case "Ready":
				health.Ready = isTrue
				if !isTrue && message != "" {
					health.Message = message
				}
			case "Failed":
				if isTrue {
					health.Status = "Failed"
					if message != "" {
						health.Message = message
					}
				}
			case "Succeeded":
				if isTrue && health.Status != "Failed" {
					health.Status = "Succeeded"
				}
			case "Running":
				if isTrue && health.Status == "" {
					health.Status = "Running"
				}
			case "Executing":
				if isTrue && health.Status == "" {
					health.Status = "Executing"
				}
			case "Canceled":
				if isTrue && health.Status != "Failed" {
					health.Status = "Canceled"
				}
			}
		}
	}

	// Default status if not set
	if health.Status == "" {
		if health.Ready {
			health.Status = "Ready"
		} else {
			health.Status = "NotReady"
		}
	}

	// Get migration statistics if available
	migration, exists, _ := unstructured.NestedMap(plan.Object, "status", "migration")
	if exists {
		if vms, vmExists, _ := unstructured.NestedSlice(migration, "vms"); vmExists {
			for _, v := range vms {
				vm, ok := v.(map[string]interface{})
				if !ok {
					continue
				}

				// Check VM conditions for success/failure
				vmConditions, exists, _ := unstructured.NestedSlice(vm, "conditions")
				if !exists {
					continue
				}

				for _, c := range vmConditions {
					condition, ok := c.(map[string]interface{})
					if !ok {
						continue
					}

					condType, _, _ := unstructured.NestedString(condition, "type")
					condStatus, _, _ := unstructured.NestedString(condition, "status")

					if condStatus == "True" {
						switch condType {
						case "Succeeded":
							health.Succeeded++
						case "Failed":
							health.Failed++
						}
					}
				}
			}
		}
	}

	return health
}

// AnalyzePlansHealth analyzes plan health and adds issues to the report
func AnalyzePlansHealth(plans []PlanHealth, report *HealthReport) {
	for _, plan := range plans {
		// Check for failed plans
		if plan.Status == "Failed" {
			message := "Migration plan failed"
			if plan.Message != "" {
				message += ": " + plan.Message
			}
			report.AddIssue(
				SeverityCritical,
				"Plan",
				plan.Name,
				message,
				"Check plan status and VM logs for details",
			)
		}

		// Check for not ready plans
		if !plan.Ready && plan.Status != "Failed" && plan.Status != "Succeeded" && plan.Status != "Canceled" {
			message := "Migration plan is not ready"
			if plan.Message != "" {
				message += ": " + plan.Message
			}
			report.AddIssue(
				SeverityWarning,
				"Plan",
				plan.Name,
				message,
				"Verify plan configuration, mappings, and provider status",
			)
		}

		// Check for VM failures in running/executing plans
		if plan.Failed > 0 && (plan.Status == "Running" || plan.Status == "Executing") {
			report.AddIssue(
				SeverityWarning,
				"Plan",
				plan.Name,
				"Plan has failed VM migrations",
				"Check individual VM status and logs",
			)
		}

		// Check for plans stuck in executing
		if plan.Status == "Executing" && plan.VMCount > 0 && plan.Succeeded+plan.Failed == 0 {
			report.AddIssue(
				SeverityInfo,
				"Plan",
				plan.Name,
				"Plan is executing but no VMs have completed",
				"Monitor migration progress",
			)
		}
	}
}
