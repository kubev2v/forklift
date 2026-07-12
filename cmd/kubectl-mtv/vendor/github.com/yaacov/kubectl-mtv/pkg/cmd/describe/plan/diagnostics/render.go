package diagnostics

import (
	"fmt"
	"strings"

	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// Render appends the diagnostics report as sections to the describe Builder.
func Render(b *describe.Builder, report *DiagnosticsReport) {
	b.Section("DIAGNOSTICS")

	// Only show VDDK config since provider/type are already in SPECIFICATION
	if report.Config.VDDKImage != "" {
		b.Field("VDDK Image", report.Config.VDDKImage)
	} else {
		b.FieldC("VDDK Image", "Not configured", output.Yellow)
	}

	// Cutover time for warm migrations
	if report.CutoverTime != "" {
		b.Field("Cutover Scheduled", report.CutoverTime)
	}

	// Remote target notice
	if report.RemoteTarget {
		b.Text(output.Yellow("Note"), "Pod logs and events are not available (target is a remote OpenShift cluster)", "")
	}

	// Per-VM diagnostics
	for i := range report.VMs {
		vm := &report.VMs[i]
		renderVM(b, vm, report.RequestedShowLines)
	}

	// Controller logs
	if report.ControllerLogs != nil {
		b.SubSection("Controller Logs")
		renderControllerLogs(b, report.ControllerLogs, report.RequestedShowLines)
		b.EndSubSection()
	}
}

func renderVM(b *describe.Builder, vm *VMDiagnostics, requestedShowLines int) {
	title := fmt.Sprintf("VM: %s (%s)", vm.Name, vm.ID)
	b.SubSection(title)

	// Phase and Error as fields
	if vm.Error != "" {
		b.FieldC("Phase", vm.Phase, output.Red)
		b.FieldC("Error", vm.Error, output.Red)
	} else {
		b.FieldC("Phase", vm.Phase, output.ColorizeStatus)
	}

	// Everything else as Text blocks to preserve ordering
	// (builder renders: Fields → Tables → Texts → SubSections)

	// Conditions
	if len(vm.Conditions) > 0 {
		b.Text(output.Cyan("VM Conditions"), formatConditionsTable(vm.Conditions), "")
	}

	// Step errors
	if len(vm.StepErrors) > 0 {
		b.Text(output.Red("Pipeline Errors"), formatStepErrorsTable(vm.StepErrors), "")
	}

	// Conversion CR
	if vm.Conversion != nil {
		b.Text(output.Blue("Conversion"), formatConversion(vm.Conversion), "")
	}

	// Pods
	if len(vm.Pods) > 0 {
		for i := range vm.Pods {
			renderPod(b, vm.Pods[i], requestedShowLines)
		}
	} else {
		b.Text(output.Yellow("Pods"), "None found (may have been cleaned up)", "")
	}

	// Events
	if len(vm.Events) > 0 {
		renderEvents(b, vm.Events)
	} else {
		b.Text(output.Yellow("Events"), "None found (may have expired)", "")
	}

	b.EndSubSection()
}

func renderPod(b *describe.Builder, pod PodDiagnostics, requestedShowLines int) {
	phaseColor := output.Green
	switch pod.Phase {
	case "Failed", "Evicted":
		phaseColor = output.Red
	case "Running":
		phaseColor = output.Yellow
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Name:   %s", pod.Name))
	lines = append(lines, fmt.Sprintf("Status: %s", phaseColor(pod.Phase)))
	if pod.Reason != "" && pod.Reason != pod.Phase {
		lines = append(lines, fmt.Sprintf("Reason: %s", pod.Reason))
	}

	if pod.V2VStage != "" {
		stageDisplay := formatV2VStage(pod.V2VStage, pod.ProgressPct, pod.Phase)
		lines = append(lines, fmt.Sprintf("V2V Stage: %s", stageDisplay))
	}

	lines = append(lines, formatLogAnalysis(pod.ErrorCount, pod.WarnCount, pod.ErrorLines, pod.LogTail, requestedShowLines)...)

	b.Text(output.Green("Pod"), strings.Join(lines, "\n"), "")
}

func renderControllerLogs(b *describe.Builder, logs *ControllerLogAnalysis, requestedShowLines int) {
	lines := formatLogAnalysis(logs.ErrorCount, logs.WarnCount, logs.ErrorLines, logs.LogTail, requestedShowLines)
	b.Text("", strings.Join(lines, "\n"), "")
}

func formatLogAnalysis(errorCount, warnCount int, errorLines, logTail []string, requestedShowLines int) []string {
	var lines []string

	summary := fmt.Sprintf("%d errors, %d warnings", errorCount, warnCount)
	if errorCount > 0 {
		lines = append(lines, fmt.Sprintf("Log Analysis: %s", output.Red(summary)))
	} else if warnCount > 0 {
		lines = append(lines, fmt.Sprintf("Log Analysis: %s", output.Yellow(summary)))
	} else {
		lines = append(lines, fmt.Sprintf("Log Analysis: %s", summary))
	}

	if errorCount > 0 && len(errorLines) > 0 {
		lines = append(lines, "")
		if len(errorLines) < requestedShowLines {
			lines = append(lines, output.Red(fmt.Sprintf("Last %d error lines (only %d found, %d requested):", len(errorLines), len(errorLines), requestedShowLines)))
		} else {
			lines = append(lines, output.Red(fmt.Sprintf("Last %d error lines:", len(errorLines))))
		}
		for _, l := range errorLines {
			errLine := l
			if len(errLine) > 200 {
				errLine = errLine[:197] + "..."
			}
			lines = append(lines, "  "+errLine)
		}
	}

	if len(logTail) > 0 {
		lines = append(lines, "")
		if len(logTail) < requestedShowLines {
			lines = append(lines, output.Cyan(fmt.Sprintf("Last %d log lines (only %d found, %d requested):", len(logTail), len(logTail), requestedShowLines)))
		} else {
			lines = append(lines, output.Cyan(fmt.Sprintf("Last %d log lines:", len(logTail))))
		}
		for _, l := range logTail {
			lines = append(lines, "  "+l)
		}
	}

	return lines
}

func renderEvents(b *describe.Builder, events []EventEntry) {
	warningCount := 0
	for _, ev := range events {
		if ev.Type == "Warning" {
			warningCount++
		}
	}

	var lines []string
	for _, ev := range events {
		var prefix string
		if ev.Type == "Warning" {
			prefix = output.Yellow("[warning]") + " "
		} else {
			prefix = output.Green("[ok]") + "      "
		}
		line := fmt.Sprintf("%s%s  %s  (%s)", prefix, ev.Reason, ev.Object, ev.Age)
		lines = append(lines, line)
		if ev.Message != "" {
			lines = append(lines, fmt.Sprintf("           %s", ev.Message))
		}
	}

	label := output.Cyan("Events (from migration pods and PVCs, warnings + scheduling/provisioning)")
	if warningCount > 0 {
		label = output.Yellow(fmt.Sprintf("Events (%d warnings, from migration pods and PVCs)", warningCount))
	}
	b.Text(label, strings.Join(lines, "\n"), "")
}

func formatConditionsTable(conditions []ConditionEntry) string {
	// Manual table formatting for consistent rendering
	var lines []string
	lines = append(lines, fmt.Sprintf("%-12s %-8s %-20s %s", "TYPE", "STATUS", "REASON", "MESSAGE"))
	for _, c := range conditions {
		lines = append(lines, fmt.Sprintf("%-12s %-8s %-20s %s", c.Type, c.Status, c.Reason, c.Message))
	}
	return strings.Join(lines, "\n")
}

func formatStepErrorsTable(stepErrors []StepError) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("%-22s %-8s %s", "STEP", "PHASE", "MESSAGE"))
	for _, se := range stepErrors {
		phase := se.Phase
		if phase == "Failed" || phase == "Error" {
			phase = output.Red(phase)
		}
		lines = append(lines, fmt.Sprintf("%-22s %-8s %s", se.Step, phase, se.Message))
	}
	return strings.Join(lines, "\n")
}

func formatConversion(conv *ConversionInfo) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("CR:       %s", conv.Name))
	lines = append(lines, fmt.Sprintf("Phase:    %s", conv.Phase))
	if conv.Message != "" {
		lines = append(lines, fmt.Sprintf("Message:  %s", conv.Message))
	}
	if conv.PodName != "" {
		lines = append(lines, fmt.Sprintf("Pod Name: %s", conv.PodName))
	}
	return strings.Join(lines, "\n")
}

func formatV2VStage(stage, progressPct, podPhase string) string {
	label := stage
	if progressPct != "" {
		label = fmt.Sprintf("%s (%s%%)", stage, progressPct)
	}

	if podPhase == "Failed" || podPhase == "Evicted" {
		return output.Red(label)
	}
	switch stage {
	case "finish":
		return output.Green(label)
	case "disk-copy":
		return output.Yellow(label)
	default:
		return label
	}
}
