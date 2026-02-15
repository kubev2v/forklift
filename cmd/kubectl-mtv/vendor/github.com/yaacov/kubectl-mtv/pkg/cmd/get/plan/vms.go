package plan

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/plan/status"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// formatDuration calculates and formats the duration between two ISO timestamps
func formatDuration(startedStr, completedStr string) string {
	if startedStr == "" || startedStr == "-" || completedStr == "" || completedStr == "-" {
		return "-"
	}

	started, err := time.Parse(time.RFC3339, startedStr)
	if err != nil {
		started, err = time.Parse(time.RFC3339Nano, startedStr)
		if err != nil {
			return "-"
		}
	}

	completed, err := time.Parse(time.RFC3339, completedStr)
	if err != nil {
		completed, err = time.Parse(time.RFC3339Nano, completedStr)
		if err != nil {
			return "-"
		}
	}

	duration := completed.Sub(started)
	if duration < 0 {
		return "-"
	}

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

// formatDiskSize formats size as human-readable based on unit
func formatDiskSize(size int64, unit string) string {
	if size <= 0 {
		return "-"
	}

	switch unit {
	case "MB":
		return fmt.Sprintf("%.1f GB", float64(size)/1024.0)
	case "GB":
		return fmt.Sprintf("%.1f GB", float64(size))
	case "KB":
		return fmt.Sprintf("%.1f GB", float64(size)/(1024.0*1024.0))
	default:
		const gb = 1024 * 1024 * 1024
		return fmt.Sprintf("%.1f GB", float64(size)/float64(gb))
	}
}

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

// printHeader prints the migration plan header
func printHeader(planName, migrationName, title string) {
	fmt.Print("\n", output.ColorizedSeparator(105, output.YellowColor))
	fmt.Printf("\n%s\n", output.Bold(title))
	fmt.Printf("%s %s\n", output.Bold("Plan:"), output.Yellow(planName))
	fmt.Printf("%s %s\n", output.Bold("Migration:"), output.Yellow(migrationName))
}

// printNoMigrationMessage prints message when no migration is found
func printNoMigrationMessage(planName string, plan *unstructured.Unstructured) {
	fmt.Printf("%s %s\n\n", output.Bold("Plan:"), output.Yellow(planName))
	fmt.Println("No migration information found. Details will be available after the plan starts running.")

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
}

// printVMInfo prints the VM basic information header
func printVMInfo(vm map[string]interface{}, showOS bool) string {
	vmName, _, _ := unstructured.NestedString(vm, "name")
	vmID, _, _ := unstructured.NestedString(vm, "id")
	vmPhase, _, _ := unstructured.NestedString(vm, "phase")
	vmOS, _, _ := unstructured.NestedString(vm, "operatingSystem")
	started, _, _ := unstructured.NestedString(vm, "started")
	completed, _, _ := unstructured.NestedString(vm, "completed")

	vmCompletionStatus := getVMCompletionStatus(vm)

	fmt.Print("\n", output.ColorizedSeparator(105, output.CyanColor))
	fmt.Printf("\n%s %s (%s %s)\n", output.Bold("VM:"), output.Yellow(vmName), output.Bold("vmID="), output.Cyan(vmID))
	fmt.Printf("%s %s  %s %s\n", output.Bold("Phase:"), output.ColorizeStatus(vmPhase), output.Bold("Status:"), output.ColorizeStatus(vmCompletionStatus))

	if showOS && vmOS != "" {
		fmt.Printf("%s %s\n", output.Bold("OS:"), output.Blue(vmOS))
	}

	if started != "" {
		fmt.Printf("%s %s", output.Bold("Started:"), output.Blue(started))
		if completed != "" {
			fmt.Printf("  %s %s", output.Bold("Completed:"), output.Green(completed))
		}
		fmt.Println()
	}

	return vmCompletionStatus
}

// printPipelineTable prints the pipeline table for a VM
func printPipelineTable(vm map[string]interface{}, vmCompletionStatus string) {
	pipeline, exists, _ := unstructured.NestedSlice(vm, "pipeline")
	if !exists || len(pipeline) == 0 {
		fmt.Println("No pipeline information available.")
		return
	}

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

		progressMap, progressExists, _ := unstructured.NestedMap(phase, "progress")
		percentage := -1.0
		if phaseStatus == status.StatusCompleted {
			percentage = 100.0
		} else if progressExists {
			progCompleted, _, _ := unstructured.NestedInt64(progressMap, "completed")
			progTotal, _, _ := unstructured.NestedInt64(progressMap, "total")
			if progTotal > 0 {
				percentage = float64(progCompleted) / float64(progTotal) * 100
				if percentage > 100.0 {
					percentage = 100.0
				}
			}
		}
		if percentage >= 0 {
			progressText := fmt.Sprintf("%14.1f%%", percentage)

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

// printDisksTable prints the disk transfer table for a VM
func printDisksTable(vm map[string]interface{}, vmCompletionStatus string) {
	pipeline, exists, _ := unstructured.NestedSlice(vm, "pipeline")
	if !exists || len(pipeline) == 0 {
		return
	}

	for _, p := range pipeline {
		phase, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		phaseName, _, _ := unstructured.NestedString(phase, "name")
		if !strings.HasPrefix(phaseName, "DiskTransfer") {
			continue
		}

		phaseStatus, _, _ := unstructured.NestedString(phase, "phase")
		phaseStarted, _, _ := unstructured.NestedString(phase, "started")
		phaseCompleted, _, _ := unstructured.NestedString(phase, "completed")

		tasks, tasksExist, _ := unstructured.NestedSlice(phase, "tasks")
		if !tasksExist || len(tasks) == 0 {
			continue
		}

		fmt.Printf("\n%s %s\n", output.Bold("Disk Transfers:"), output.Yellow(phaseName))
		headers := []string{"NAME", "PHASE", "PROGRESS", "SIZE", "DURATION", "STARTED", "COMPLETED"}
		colWidths := []int{35, 12, 12, 12, 10, 22, 22}
		rows := make([][]string, 0, len(tasks))

		phaseAnnotations, _, _ := unstructured.NestedStringMap(phase, "annotations")
		phaseUnit := phaseAnnotations["unit"]

		for _, t := range tasks {
			task, ok := t.(map[string]interface{})
			if !ok {
				continue
			}

			taskName, _, _ := unstructured.NestedString(task, "name")
			taskPhase, _, _ := unstructured.NestedString(task, "phase")
			taskStartedStr, _, _ := unstructured.NestedString(task, "started")
			taskCompletedStr, _, _ := unstructured.NestedString(task, "completed")

			if taskPhase == "" {
				taskPhase = phaseStatus
			}

			if len(taskName) > colWidths[0] {
				taskName = taskName[:colWidths[0]-3] + "..."
			}

			taskAnnotations, _, _ := unstructured.NestedStringMap(task, "annotations")
			taskUnit := taskAnnotations["unit"]
			if taskUnit == "" {
				taskUnit = phaseUnit
			}

			progress := "-"
			size := "-"
			taskProgressMap, taskProgressExists, _ := unstructured.NestedMap(task, "progress")
			if taskProgressExists {
				taskProgCompleted, _, _ := unstructured.NestedInt64(taskProgressMap, "completed")
				taskTotal, _, _ := unstructured.NestedInt64(taskProgressMap, "total")
				if taskTotal > 0 {
					percentage := float64(taskProgCompleted) / float64(taskTotal) * 100
					if percentage > 100.0 {
						percentage = 100.0
					}
					progressText := fmt.Sprintf("%.1f%%", percentage)

					switch vmCompletionStatus {
					case status.StatusFailed:
						progress = output.Red(progressText)
					case status.StatusCanceled:
						progress = output.Yellow(progressText)
					case status.StatusSucceeded, status.StatusCompleted:
						progress = output.Green(progressText)
					default:
						if percentage >= 100 {
							progress = output.Green(progressText)
						} else {
							progress = output.Cyan(progressText)
						}
					}
					size = formatDiskSize(taskTotal, taskUnit)
				}
			} else if taskPhase == status.StatusCompleted {
				progress = output.Green("100.0%")
			}

			startedStr := taskStartedStr
			if startedStr == "" {
				startedStr = phaseStarted
			}
			if startedStr == "" {
				startedStr = "-"
			}
			completedStr := taskCompletedStr
			if completedStr == "" && taskPhase == status.StatusCompleted {
				completedStr = phaseCompleted
			}
			if completedStr == "" {
				completedStr = "-"
			}

			duration := formatDuration(startedStr, completedStr)

			rows = append(rows, []string{
				taskName,
				output.ColorizeStatus(taskPhase),
				progress,
				size,
				duration,
				startedStr,
				completedStr,
			})
		}

		PrintTable(headers, rows, colWidths)
	}
}

// getMigrationData retrieves and validates plan and migration data
func getMigrationData(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string) (*unstructured.Unstructured, *unstructured.Unstructured, []interface{}, error) {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get client: %v", err)
	}

	plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get plan: %v", err)
	}

	planDetails, _ := status.GetPlanDetails(c, namespace, plan, client.MigrationsGVR)

	migration := planDetails.RunningMigration
	if migration == nil {
		migration = planDetails.LatestMigration
	}
	if migration == nil {
		return plan, nil, nil, nil
	}

	vms, exists, err := unstructured.NestedSlice(migration.Object, "status", "vms")
	if err != nil {
		return plan, migration, nil, fmt.Errorf("failed to get VM list: %v", err)
	}
	if !exists {
		return plan, migration, nil, fmt.Errorf("no VMs found in migration status")
	}

	return plan, migration, vms, nil
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

func listVMsOnce(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	plan, migration, vms, err := getMigrationData(ctx, configFlags, name, namespace)
	if err != nil {
		return err
	}

	if migration == nil {
		printNoMigrationMessage(name, plan)
		return nil
	}

	printHeader(name, migration.GetName(), "MIGRATION PLAN")

	for _, v := range vms {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		vmCompletionStatus := printVMInfo(vm, true)
		printPipelineTable(vm, vmCompletionStatus)
	}

	return nil
}

// ListDisks lists all disk transfers in a migration plan
func ListDisks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listDisksOnce(ctx, configFlags, name, namespace)
		}, watch.DefaultInterval)
	}
	return listDisksOnce(ctx, configFlags, name, namespace)
}

func listDisksOnce(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	plan, migration, vms, err := getMigrationData(ctx, configFlags, name, namespace)
	if err != nil {
		return err
	}

	if migration == nil {
		printNoMigrationMessage(name, plan)
		return nil
	}

	printHeader(name, migration.GetName(), "MIGRATION PLAN - DISK TRANSFERS")

	for _, v := range vms {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		vmCompletionStatus := printVMInfo(vm, false)
		printDisksTable(vm, vmCompletionStatus)
	}

	return nil
}

// ListVMsWithDisks lists all VMs with disk transfer details
func ListVMsWithDisks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string, watchMode bool) error {
	if watchMode {
		return watch.Watch(func() error {
			return listVMsWithDisksOnce(ctx, configFlags, name, namespace)
		}, watch.DefaultInterval)
	}
	return listVMsWithDisksOnce(ctx, configFlags, name, namespace)
}

func listVMsWithDisksOnce(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string) error {
	plan, migration, vms, err := getMigrationData(ctx, configFlags, name, namespace)
	if err != nil {
		return err
	}

	if migration == nil {
		printNoMigrationMessage(name, plan)
		return nil
	}

	printHeader(name, migration.GetName(), "MIGRATION PLAN - VMS WITH DISK DETAILS")

	for _, v := range vms {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		vmCompletionStatus := printVMInfo(vm, true)
		printPipelineTable(vm, vmCompletionStatus)
		printDisksTable(vm, vmCompletionStatus)
	}

	return nil
}
