package plan

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// getVMCompletionStatus determines if a completed VM succeeded, failed, or was canceled
func getVMCompletionStatus(vm map[string]interface{}) string {
	conditions, exists, _ := unstructured.NestedSlice(vm, "conditions")
	if !exists {
		return status.StatusUnknown
	}

	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condition, "type")
		condStatus, _, _ := unstructured.NestedString(condition, "status")

		if condStatus == "True" {
			switch condType {
			case status.StatusSucceeded:
				return status.StatusSucceeded
			case status.StatusFailed:
				return status.StatusFailed
			case status.StatusCanceled:
				return status.StatusCanceled
			case status.StatusCompleted:
				return status.StatusCompleted
			}
		}
	}

	return status.StatusUnknown
}

// ListVMs lists all VMs in a migration plan
func ListVMs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listVMsOnce(ctx, configFlags, name, namespace)
		}, watch.DefaultInterval)
	}

	return listVMsOnce(ctx, configFlags, name, namespace)
}

// listVMsOnce lists VMs in a migration plan once (helper function for ListVMs)
func listVMsOnce(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan: %v", err)
	}

	// Get plan details
	planDetails, _ := status.GetPlanDetails(c, namespace, plan, client.MigrationsGVR)

	// Get migration object to display VM details
	migration := planDetails.RunningMigration
	if migration == nil {
		migration = planDetails.LatestMigration
	}
	if migration == nil {
		fmt.Printf("%s %s\n\n", output.Bold("VMs in migration plan:"), output.Yellow(name))
		fmt.Println("No migration information found. VM details will be available after the plan starts running.")

		// Print VMs from plan spec
		specVMs, exists, err := unstructured.NestedSlice(plan.Object, "spec", "vms")
		if err == nil && exists && len(specVMs) > 0 {
			fmt.Printf("\n%s\n", output.Bold("Plan VM Specifications:"))
			headers := []string{"NAME", "ID"}
			colWidths := []int{40, 20}
			rows := make([][]string, 0, len(specVMs))

			for _, v := range specVMs {
				vm, ok := v.(map[string]interface{})
				if !ok {
					continue
				}

				vmName, _, _ := unstructured.NestedString(vm, "name")
				vmID, _, _ := unstructured.NestedString(vm, "id")
				rows = append(rows, []string{output.Yellow(vmName), output.Cyan(vmID)})
			}

			PrintTable(headers, rows, colWidths)
		}

		return nil
	}

	// Get VMs list from migration status
	vms, exists, err := unstructured.NestedSlice(migration.Object, "status", "vms")
	if err != nil {
		return fmt.Errorf("failed to get VM list: %v", err)
	}
	if !exists {
		return fmt.Errorf("no VMs found in migration status")
	}

	fmt.Print("\n", output.ColorizedSeparator(105, output.YellowColor))
	fmt.Printf("\n%s\n", output.Bold("MIGRATION PLAN"))

	fmt.Printf("%s %s\n", output.Bold("VMs in migration plan:"), output.Yellow(name))
	fmt.Printf("%s %s\n", output.Bold("Migration:"), output.Yellow(migration.GetName()))

	// Print VM information
	for _, v := range vms {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, _, _ := unstructured.NestedString(vm, "name")
		vmID, _, _ := unstructured.NestedString(vm, "id")
		vmPhase, _, _ := unstructured.NestedString(vm, "phase")
		vmOS, _, _ := unstructured.NestedString(vm, "operatingSystem")
		started, _, _ := unstructured.NestedString(vm, "started")
		completed, _, _ := unstructured.NestedString(vm, "completed")

		vmCompletionStatus := getVMCompletionStatus(vm)

		fmt.Printf("%s %s (%s %s)\n", output.Bold("VM:"), output.Yellow(vmName), output.Bold("vmID="), output.Cyan(vmID))
		fmt.Printf("%s %s\n", output.Bold("Phase:"), output.ColorizeStatus(vmPhase))
		fmt.Printf("%s %s\n", output.Bold("Status:"), output.ColorizeStatus(vmCompletionStatus))
		fmt.Printf("%s %s\n", output.Bold("OS:"), output.Blue(vmOS))
		if started != "" {
			fmt.Printf("%s %s\n", output.Bold("Started:"), output.Blue(started))
		}
		if completed != "" {
			fmt.Printf("%s %s\n", output.Bold("Completed:"), output.Green(completed))
		}

		// Print pipeline information
		pipeline, exists, _ := unstructured.NestedSlice(vm, "pipeline")
		if exists && len(pipeline) > 0 {
			fmt.Printf("\n%s\n", output.Bold("Pipeline:"))
			headers := []string{"PHASE", "NAME", "STARTED", "COMPLETED", "PROGRESS"}
			colWidths := []int{15, 25, 25, 25, 15}
			rows := make([][]string, 0, len(pipeline))

			for _, p := range pipeline {
				phase, ok := p.(map[string]interface{})
				if !ok {
					continue
				}

				phaseName, _, _ := unstructured.NestedString(phase, "name")
				phaseStatus, _, _ := unstructured.NestedString(phase, "phase")
				phaseStarted, _, _ := unstructured.NestedString(phase, "started")
				phaseCompleted, _, _ := unstructured.NestedString(phase, "completed")
				progress := "-"

				var progCompleted int64
				var progTotal int64

				progressMap, progressExists, _ := unstructured.NestedMap(phase, "progress")
				percentage := -1.0
				if phaseStatus == status.StatusCompleted {
					// Always show 100% for completed phases, even when totals are missing
					percentage = 100.0
				} else if progressExists {
					progCompleted, _, _ = unstructured.NestedInt64(progressMap, "completed")
					progTotal, _, _ = unstructured.NestedInt64(progressMap, "total")
					if progTotal > 0 {
						percentage = float64(progCompleted) / float64(progTotal) * 100
						if percentage > 100.0 {
							percentage = 100.0
						}
					}
				}
				if percentage >= 0 {
					progressText := fmt.Sprintf("%14.1f%%", percentage)

					// Handle VM completion status
					switch vmCompletionStatus {
					case status.StatusFailed:
						progress = output.Red(progressText)
					case status.StatusCanceled:
						progress = output.Yellow(progressText)
					case status.StatusSucceeded, status.StatusCompleted:
						progress = output.Green(progressText)
					default:
						progress = output.Cyan(progressText)
					}
				}

				rows = append(rows, []string{
					output.ColorizeStatus(phaseStatus),
					output.Bold(phaseName),
					phaseStarted,
					phaseCompleted,
					progress,
				})
			}

			PrintTable(headers, rows, colWidths)
		}
	}

	return nil
}
