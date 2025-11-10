package plan

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	planutil "github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// DescribeVM describes a specific VM in a migration plan
func DescribeVM(configFlags *genericclioptions.ConfigFlags, name, namespace, vmName string, watchMode bool, useUTC bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return describeVMOnce(configFlags, name, namespace, vmName, useUTC)
		}, 20*time.Second)
	}

	return describeVMOnce(configFlags, name, namespace, vmName, useUTC)
}

// Helper function to truncate strings to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Reserve 3 characters for the ellipsis
	return s[:maxLen-3] + "..."
}

func describeVMOnce(configFlags *genericclioptions.ConfigFlags, name, namespace, vmName string, useUTC bool) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Get the plan
	plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan: %v", err)
	}

	// First check if VM exists in plan spec
	specVMs, exists, err := unstructured.NestedSlice(plan.Object, "spec", "vms")
	if err != nil || !exists {
		fmt.Printf("No VMs found in plan '%s' specification\n", output.Yellow(name))
		return nil
	}

	// Find VM ID from spec
	var vmID string
	for _, v := range specVMs {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		currentVMName, _, _ := unstructured.NestedString(vm, "name")
		if currentVMName == vmName {
			vmID, _, _ = unstructured.NestedString(vm, "id")
			break
		}
	}

	if vmID == "" {
		fmt.Printf("VM '%s' is not part of plan '%s'\n", output.Yellow(vmName), output.Yellow(name))
		return nil
	}

	// Get plan details
	planDetails, _ := status.GetPlanDetails(c, namespace, plan, client.MigrationsGVR)

	// Get migration object to display VM details
	migration := planDetails.RunningMigration
	if migration == nil {
		migration = planDetails.LatestMigration
	}
	if migration == nil {
		fmt.Printf("No migration found for plan '%s'. VM details will be available after the plan starts running.\n", output.Yellow(name))
		return nil
	}

	// Get VMs list from migration status
	vms, exists, err := unstructured.NestedSlice(migration.Object, "status", "vms")
	if err != nil {
		return fmt.Errorf("failed to get VM list: %v", err)
	}
	if !exists {
		fmt.Printf("No VM status information found in migration. Please wait for the migration to start.\n")
		return nil
	}

	// Find the specified VM using vmID
	var targetVM map[string]interface{}
	for _, v := range vms {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		currentVMID, _, _ := unstructured.NestedString(vm, "id")
		if currentVMID == vmID {
			targetVM = vm
			break
		}
	}

	if targetVM == nil {
		fmt.Printf("VM '%s' (vmID=%s) status not yet available in migration\n", output.Yellow(vmName), output.Cyan(vmID))
		return nil
	}

	// Print VM details
	fmt.Print("\n", output.ColorizedSeparator(105, output.YellowColor))
	fmt.Printf("\n%s", output.Bold("MIGRATION PLAN"))
	fmt.Printf("\n%s %s\n", output.Bold("VM Details for:"), output.Yellow(vmName))
	fmt.Printf("%s %s\n", output.Bold("Migration Plan:"), output.Yellow(name))
	fmt.Printf("%s %s\n", output.Bold("Migration:"), output.Yellow(migration.GetName()))

	fmt.Print("\n", output.ColorizedSeparator(105, output.YellowColor), "\n")

	// Print basic VM information
	vmID, _, _ = unstructured.NestedString(targetVM, "id")
	vmPhase, _, _ := unstructured.NestedString(targetVM, "phase")
	vmOS, _, _ := unstructured.NestedString(targetVM, "operatingSystem")
	started, _, _ := unstructured.NestedString(targetVM, "started")
	completed, _, _ := unstructured.NestedString(targetVM, "completed")
	newName, _, _ := unstructured.NestedString(targetVM, "newName")

	fmt.Printf("%s %s\n", output.Bold("ID:"), output.Cyan(vmID))
	fmt.Printf("%s %s\n", output.Bold("Phase:"), output.ColorizeStatus(vmPhase))
	fmt.Printf("%s %s\n", output.Bold("OS:"), output.Blue(vmOS))
	if newName != "" {
		fmt.Printf("%s %s\n", output.Bold("New Name:"), output.Yellow(newName))
	}
	if started != "" {
		fmt.Printf("%s %s\n", output.Bold("Started:"), planutil.FormatTime(started, useUTC))
	}
	if completed != "" {
		fmt.Printf("%s %s\n", output.Bold("Completed:"), planutil.FormatTime(completed, useUTC))
	}

	// Print conditions
	conditions, exists, _ := unstructured.NestedSlice(targetVM, "conditions")
	if exists && len(conditions) > 0 {
		fmt.Print("\n", output.ColorizedSeparator(105, output.YellowColor))

		fmt.Printf("\n%s\n", output.Bold("Conditions:"))
		headers := []string{"TYPE", "STATUS", "CATEGORY", "MESSAGE"}
		colWidths := []int{15, 10, 15, 50}
		rows := make([][]string, 0, len(conditions))

		for _, c := range conditions {
			condition, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condition, "type")
			status, _, _ := unstructured.NestedString(condition, "status")
			category, _, _ := unstructured.NestedString(condition, "category")
			message, _, _ := unstructured.NestedString(condition, "message")

			// Apply color to status
			switch status {
			case "True":
				status = output.Green(status)
			case "False":
				status = output.Red(status)
			}

			rows = append(rows, []string{output.Bold(condType), status, category, message})
		}

		if len(rows) > 0 {
			planutil.PrintTable(headers, rows, colWidths)
		}
	}

	// Print pipeline information
	pipeline, exists, _ := unstructured.NestedSlice(targetVM, "pipeline")
	if exists {
		fmt.Print("\n", output.ColorizedSeparator(105, output.YellowColor))

		fmt.Printf("\n%s\n", output.Bold("Pipeline:"))
		for _, p := range pipeline {
			phase, ok := p.(map[string]interface{})
			if !ok {
				continue
			}

			phaseName, _, _ := unstructured.NestedString(phase, "name")
			phaseDesc, _, _ := unstructured.NestedString(phase, "description")
			phaseStatus, _, _ := unstructured.NestedString(phase, "phase")
			phaseStarted, _, _ := unstructured.NestedString(phase, "started")
			phaseCompleted, _, _ := unstructured.NestedString(phase, "completed")

			fmt.Printf("\n%s\n", output.Yellow(fmt.Sprintf("[%s] %s", output.Bold(phaseName), phaseDesc)))
			fmt.Printf("%s %s\n", output.Bold("Status:"), output.ColorizeStatus(phaseStatus))
			fmt.Printf("%s %s\n", output.Bold("Started:"), planutil.FormatTime(phaseStarted, useUTC))
			if phaseCompleted != "" {
				fmt.Printf("%s %s\n", output.Bold("Completed:"), planutil.FormatTime(phaseCompleted, useUTC))
			}

			// Print progress
			progressMap, exists, _ := unstructured.NestedMap(phase, "progress")
			if exists {
				completed, _, _ := unstructured.NestedInt64(progressMap, "completed")
				total, _, _ := unstructured.NestedInt64(progressMap, "total")
				if total > 0 {
					percentage := float64(completed) / float64(total) * 100
					progressText := fmt.Sprintf("%.1f%% (%d/%d)", percentage, completed, total)

					if percentage >= 100 {
						fmt.Printf("%s %s\n", output.Bold("Progress:"), output.Green(progressText))
					} else if percentage >= 75 {
						fmt.Printf("%s %s\n", output.Bold("Progress:"), output.Blue(progressText))
					} else if percentage >= 25 {
						fmt.Printf("%s %s\n", output.Bold("Progress:"), output.Yellow(progressText))
					} else {
						fmt.Printf("%s %s\n", output.Bold("Progress:"), output.Cyan(progressText))
					}
				}
			}

			// Print tasks if they exist
			tasks, exists, _ := unstructured.NestedSlice(phase, "tasks")
			if exists && len(tasks) > 0 {
				fmt.Printf("\n%s\n", output.Bold("Tasks:"))
				headers := []string{"NAME", "PHASE", "PROGRESS", "STARTED", "COMPLETED"}
				colWidths := []int{40, 10, 15, 20, 20}
				rows := make([][]string, 0, len(tasks))

				for _, t := range tasks {
					task, ok := t.(map[string]interface{})
					if !ok {
						continue
					}

					taskName, _, _ := unstructured.NestedString(task, "name")
					// Truncate task name if longer than column width
					taskName = truncateString(taskName, colWidths[0])

					taskPhase, _, _ := unstructured.NestedString(task, "phase")
					taskStarted, _, _ := unstructured.NestedString(task, "started")
					taskCompleted, _, _ := unstructured.NestedString(task, "completed")

					progress := "-"
					progressMap, exists, _ := unstructured.NestedMap(task, "progress")
					if exists {
						completed, _, _ := unstructured.NestedInt64(progressMap, "completed")
						total, _, _ := unstructured.NestedInt64(progressMap, "total")
						if total > 0 {
							percentage := float64(completed) / float64(total) * 100
							progressText := fmt.Sprintf("%.1f%%", percentage)

							if percentage >= 100 {
								progress = output.Green(progressText)
							} else if percentage >= 75 {
								progress = output.Blue(progressText)
							} else if percentage >= 25 {
								progress = output.Yellow(progressText)
							} else {
								progress = output.Cyan(progressText)
							}
						}
					}

					rows = append(rows, []string{
						taskName,
						output.ColorizeStatus(taskPhase),
						progress,
						planutil.FormatTime(taskStarted, useUTC),
						planutil.FormatTime(taskCompleted, useUTC),
					})
				}

				planutil.PrintTable(headers, rows, colWidths)
			}
		}
	}

	return nil
}
