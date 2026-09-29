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
	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
	"github.com/yaacov/kubectl-mtv/pkg/util/watch"
)

// DescribeVM describes a specific VM in a migration plan.
func DescribeVM(configFlags *genericclioptions.ConfigFlags, name, namespace, vmName string, watchMode bool, useUTC bool, outputFormat string) error {
	if watchMode {
		return watch.Watch(func() error {
			return describeVMOnce(configFlags, name, namespace, vmName, useUTC, outputFormat)
		}, 20*time.Second)
	}

	return describeVMOnce(configFlags, name, namespace, vmName, useUTC, outputFormat)
}

func describeVMOnce(configFlags *genericclioptions.ConfigFlags, name, namespace, vmName string, useUTC bool, outputFormat string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	plan, err := c.Resource(client.PlansGVR).Namespace(namespace).Get(context.TODO(), name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get plan: %v", err)
	}

	specVMs, exists, err := unstructured.NestedSlice(plan.Object, "spec", "vms")
	if err != nil || !exists {
		return fmt.Errorf("no VMs found in plan '%s' specification", name)
	}

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
		return fmt.Errorf("VM '%s' is not part of plan '%s'", vmName, name)
	}

	planDetails, _ := status.GetPlanDetails(c, namespace, plan, client.MigrationsGVR)

	migration := planDetails.RunningMigration
	if migration == nil {
		migration = planDetails.LatestMigration
	}
	if migration == nil {
		return fmt.Errorf("no migration found for plan '%s'; VM details will be available after the plan starts running", name)
	}

	vms, exists, err := unstructured.NestedSlice(migration.Object, "status", "vms")
	if err != nil {
		return fmt.Errorf("failed to get VM list: %v", err)
	}
	if !exists {
		return fmt.Errorf("no VM status information found in migration; please wait for the migration to start")
	}

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
		return fmt.Errorf("VM '%s' (vmID=%s) status not yet available in migration", vmName, vmID)
	}

	b := describe.NewBuilder("VM MIGRATION STATUS")

	// Header
	b.Field("VM Name", vmName)
	b.Field("Migration Plan", name)
	b.Field("Migration", migration.GetName())

	// Basic VM information
	vmID, _, _ = unstructured.NestedString(targetVM, "id")
	vmPhase, _, _ := unstructured.NestedString(targetVM, "phase")
	vmOS, _, _ := unstructured.NestedString(targetVM, "operatingSystem")
	started, _, _ := unstructured.NestedString(targetVM, "started")
	completed, _, _ := unstructured.NestedString(targetVM, "completed")
	newName, _, _ := unstructured.NestedString(targetVM, "newName")

	b.Section("VM DETAILS")
	b.FieldC("ID", vmID, output.Cyan)
	b.FieldC("Phase", vmPhase, output.ColorizeStatus)
	b.FieldC("OS", vmOS, output.Blue)
	if newName != "" {
		b.Field("New Name", newName)
	}
	if started != "" {
		b.Field("Started", planutil.FormatTime(started, useUTC))
	}
	if completed != "" {
		b.Field("Completed", planutil.FormatTime(completed, useUTC))
	}

	// Conditions
	conditions, exists, _ := unstructured.NestedSlice(targetVM, "conditions")
	if exists && len(conditions) > 0 {
		b.Section("CONDITIONS")
		addConditionsTable(b, conditions)
	}

	// Pipeline
	pipeline, exists, _ := unstructured.NestedSlice(targetVM, "pipeline")
	if exists && len(pipeline) > 0 {
		b.Section("PIPELINE")
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

			title := phaseName
			if phaseDesc != "" {
				title += " - " + phaseDesc
			}
			b.SubSection(title)
			b.FieldC("Status", phaseStatus, output.ColorizeStatus)
			b.Field("Started", planutil.FormatTime(phaseStarted, useUTC))
			if phaseCompleted != "" {
				b.Field("Completed", planutil.FormatTime(phaseCompleted, useUTC))
			}

			progressMap, exists, _ := unstructured.NestedMap(phase, "progress")
			if exists {
				comp, _, _ := unstructured.NestedInt64(progressMap, "completed")
				total, _, _ := unstructured.NestedInt64(progressMap, "total")
				if total > 0 {
					pct := float64(comp) / float64(total) * 100
					progressText := fmt.Sprintf("%.1f%% (%d/%d)", pct, comp, total)
					b.FieldC("Progress", progressText, output.ColorizeProgress)
				}
			}

			// Tasks table
			tasks, exists, _ := unstructured.NestedSlice(phase, "tasks")
			if exists && len(tasks) > 0 {
				taskHeaders := []describe.TableColumn{
					{Display: "NAME", Key: "name"},
					{Display: "PHASE", Key: "phase", ColorFunc: output.ColorizeStatus},
					{Display: "PROGRESS", Key: "progress", ColorFunc: output.ColorizeProgress},
					{Display: "STARTED", Key: "started"},
					{Display: "COMPLETED", Key: "completed"},
				}

				taskRows := make([]map[string]string, 0, len(tasks))
				for _, t := range tasks {
					task, ok := t.(map[string]interface{})
					if !ok {
						continue
					}

					taskName, _, _ := unstructured.NestedString(task, "name")
					taskPhase, _, _ := unstructured.NestedString(task, "phase")
					taskStarted, _, _ := unstructured.NestedString(task, "started")
					taskCompleted, _, _ := unstructured.NestedString(task, "completed")

					progress := "-"
					if pm, exists, _ := unstructured.NestedMap(task, "progress"); exists {
						comp, _, _ := unstructured.NestedInt64(pm, "completed")
						total, _, _ := unstructured.NestedInt64(pm, "total")
						if total > 0 {
							progress = fmt.Sprintf("%.1f%%", float64(comp)/float64(total)*100)
						}
					}

					taskRows = append(taskRows, map[string]string{
						"name":      taskName,
						"phase":     taskPhase,
						"progress":  progress,
						"started":   planutil.FormatTime(taskStarted, useUTC),
						"completed": planutil.FormatTime(taskCompleted, useUTC),
					})
				}

				b.Table(taskHeaders, taskRows)
			}

			b.EndSubSection()
		}
	}

	return describe.Print(b.Build(), outputFormat)
}

func addConditionsTable(b *describe.Builder, conditions []interface{}) {
	headers := []describe.TableColumn{
		{Display: "TYPE", Key: "type"},
		{Display: "STATUS", Key: "status", ColorFunc: output.ColorizeConditionStatus},
		{Display: "CATEGORY", Key: "category", ColorFunc: output.ColorizeCategory},
		{Display: "MESSAGE", Key: "message"},
	}

	rows := make([]map[string]string, 0, len(conditions))
	for _, c := range conditions {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)
		category, _ := condMap["category"].(string)
		message, _ := condMap["message"].(string)
		rows = append(rows, map[string]string{
			"type":     condType,
			"status":   condStatus,
			"category": category,
			"message":  message,
		})
	}

	b.Table(headers, rows)
}
