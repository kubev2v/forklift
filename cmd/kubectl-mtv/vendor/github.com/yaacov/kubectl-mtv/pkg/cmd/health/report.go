package health

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// FormatReport formats the health report in the specified output format
func FormatReport(report *HealthReport, outputFormat string) (string, error) {
	switch strings.ToLower(outputFormat) {
	case "json":
		return formatJSON(report)
	case "yaml":
		return formatYAML(report)
	default:
		return formatTable(report), nil
	}
}

// formatJSON formats the report as JSON
func formatJSON(report *HealthReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// formatYAML formats the report as YAML
func formatYAML(report *HealthReport) (string, error) {
	data, err := yaml.Marshal(report)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// formatTable formats the report as a colored table
func formatTable(report *HealthReport) string {
	var sb strings.Builder

	// Header
	sb.WriteString(output.ColorizedSeparator(80, output.YellowColor))
	sb.WriteString("\n")
	sb.WriteString(output.Bold(output.Cyan("MTV HEALTH REPORT")))
	sb.WriteString("\n")
	sb.WriteString(output.ColorizedSeparator(80, output.YellowColor))
	sb.WriteString("\n\n")

	// Operator Status
	sb.WriteString(formatOperatorSection(report))

	// Controller Status
	sb.WriteString(formatControllerSection(report))

	// Pods Status
	sb.WriteString(formatPodsSection(report))

	// Log Analysis
	if len(report.LogAnalysis) > 0 {
		sb.WriteString(formatLogAnalysisSection(report))
	}

	// Providers
	sb.WriteString(formatProvidersSection(report))

	// Plans
	sb.WriteString(formatPlansSection(report))

	// Summary
	sb.WriteString(formatSummarySection(report))

	return sb.String()
}

func formatOperatorSection(report *HealthReport) string {
	var sb strings.Builder
	sb.WriteString(output.Bold(output.Cyan("OPERATOR STATUS")))
	sb.WriteString("\n")

	op := report.Operator
	if op.Installed {
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("MTV Operator:"), output.Green("Installed")))
		if op.Version != "" && op.Version != "unknown" {
			sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("Version:"), output.Yellow(op.Version)))
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("Namespace:"), output.Yellow(op.Namespace)))
	} else {
		// Check for error or unknown status before falling back to generic "Not Installed"
		if op.Error != "" {
			sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("MTV Operator:"), output.Red(op.Error)))
		} else if op.Status == "Unknown" {
			sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("MTV Operator:"), output.Yellow(op.Status)))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("MTV Operator:"), output.Red("Not Installed")))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func formatControllerSection(report *HealthReport) string {
	var sb strings.Builder
	sb.WriteString(output.Bold(output.Cyan("FORKLIFT CONTROLLER")))
	sb.WriteString("\n")

	ctrl := report.Controller
	if !ctrl.Found {
		// Prefer displaying the error if present (e.g., API/auth failures)
		if ctrl.Error != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", output.Red(ctrl.Error)))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", output.Red("ForkliftController not found")))
		}
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("Name:"), output.Yellow(ctrl.Name)))

	// Feature Flags
	sb.WriteString(fmt.Sprintf("  %s\n", output.Bold("Feature Flags:")))
	sb.WriteString(fmt.Sprintf("    UI Plugin:      %s\n", formatBoolPtr(ctrl.FeatureFlags.UIPlugin)))
	sb.WriteString(fmt.Sprintf("    Validation:     %s\n", formatBoolPtr(ctrl.FeatureFlags.Validation)))
	sb.WriteString(fmt.Sprintf("    Volume Pop:     %s\n", formatBoolPtr(ctrl.FeatureFlags.VolumePopulator)))
	sb.WriteString(fmt.Sprintf("    Auth Required:  %s\n", formatBoolPtr(ctrl.FeatureFlags.AuthRequired)))
	// Show OCP Live Migration if there's a remote OpenShift provider or if it's explicitly set
	if ctrl.HasRemoteOpenShiftProvider || ctrl.FeatureFlags.OCPLiveMigration != nil {
		liveMigStatus := formatBoolPtr(ctrl.FeatureFlags.OCPLiveMigration)
		if ctrl.HasRemoteOpenShiftProvider && (ctrl.FeatureFlags.OCPLiveMigration == nil || !*ctrl.FeatureFlags.OCPLiveMigration) {
			liveMigStatus = output.Red("[not set] *** Remote OpenShift provider exists!")
		}
		sb.WriteString(fmt.Sprintf("    OCP Live Mig:   %s\n", liveMigStatus))
	}

	// Custom Images
	if len(ctrl.CustomImages) > 0 {
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("Custom Images:"), output.Yellow(fmt.Sprintf("%d overrides", len(ctrl.CustomImages)))))
		for _, img := range ctrl.CustomImages {
			sb.WriteString(fmt.Sprintf("    - %s: %s\n", img.Field, output.Blue(img.Image)))
		}
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("Custom Images:"), "None"))
	}

	// VDDK Image
	if ctrl.VDDKImage != "" {
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("VDDK Image:"), output.Green(ctrl.VDDKImage)))
	} else if ctrl.HasVSphereProvider {
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("VDDK Image:"), output.Red("[NOT SET] *** WARNING: vSphere providers exist!")))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("VDDK Image:"), "[not set]"))
	}

	// Log Level
	if ctrl.LogLevel > 0 {
		sb.WriteString(fmt.Sprintf("  %s %d\n", output.Bold("Log Level:"), ctrl.LogLevel))
	}

	sb.WriteString("\n")
	return sb.String()
}

func formatPodsSection(report *HealthReport) string {
	var sb strings.Builder

	unhealthy := 0
	for _, pod := range report.Pods {
		if !pod.Ready || pod.Status != "Running" || len(pod.Issues) > 0 {
			unhealthy++
		}
	}

	sb.WriteString(output.Bold(output.Cyan(fmt.Sprintf("FORKLIFT PODS (%d total, %d unhealthy)", len(report.Pods), unhealthy))))
	sb.WriteString("\n")

	if len(report.Pods) == 0 {
		sb.WriteString("  No Forklift pods found\n")
		sb.WriteString("\n")
		return sb.String()
	}

	// Table header
	sb.WriteString(fmt.Sprintf("  %-45s %-10s %-10s %s\n",
		output.Bold("NAME"), output.Bold("STATUS"), output.Bold("RESTARTS"), output.Bold("ISSUES")))

	for _, pod := range report.Pods {
		status := pod.Status
		if pod.Ready && pod.Status == "Running" {
			status = output.Green(status)
		} else if pod.Status == "Failed" {
			status = output.Red(status)
		} else if pod.Status == "Pending" {
			status = output.Yellow(status)
		}

		restarts := fmt.Sprintf("%d", pod.Restarts)
		if pod.Restarts > 5 {
			restarts = output.Red(restarts)
		} else if pod.Restarts > 0 {
			restarts = output.Yellow(restarts)
		}

		issues := "None"
		if len(pod.Issues) > 0 {
			issues = output.Red(strings.Join(pod.Issues, ", "))
		}

		// Truncate name if too long
		name := pod.Name
		if len(name) > 43 {
			name = name[:40] + "..."
		}

		sb.WriteString(fmt.Sprintf("  %-45s %-10s %-10s %s\n", name, status, restarts, issues))
	}

	sb.WriteString("\n")
	return sb.String()
}

func formatLogAnalysisSection(report *HealthReport) string {
	var sb strings.Builder
	sb.WriteString(output.Bold(output.Cyan("POD LOG ANALYSIS")))
	sb.WriteString("\n")

	for _, analysis := range report.LogAnalysis {
		errStr := fmt.Sprintf("%d errors", analysis.Errors)
		warnStr := fmt.Sprintf("%d warnings", analysis.Warnings)

		if analysis.Errors > 0 {
			errStr = output.Red(errStr)
		} else {
			errStr = output.Green(errStr)
		}

		if analysis.Warnings > 5 {
			warnStr = output.Yellow(warnStr)
		}

		sb.WriteString(fmt.Sprintf("  %-25s %s, %s\n", analysis.Name+":", errStr, warnStr))
	}

	sb.WriteString("\n")
	return sb.String()
}

func formatProvidersSection(report *HealthReport) string {
	var sb strings.Builder

	unhealthy := 0
	for _, provider := range report.Providers {
		if !provider.Ready {
			unhealthy++
		}
	}

	sb.WriteString(output.Bold(output.Cyan(fmt.Sprintf("PROVIDERS (%d total, %d unhealthy)", len(report.Providers), unhealthy))))
	sb.WriteString("\n")

	if len(report.Providers) == 0 {
		sb.WriteString("  No providers found\n")
		sb.WriteString("\n")
		return sb.String()
	}

	// Table header
	sb.WriteString(fmt.Sprintf("  %-20s %-15s %-12s %-10s %-10s %-10s\n",
		output.Bold("NAME"), output.Bold("NAMESPACE"), output.Bold("TYPE"),
		output.Bold("CONNECTED"), output.Bold("INVENTORY"), output.Bold("READY")))

	for _, provider := range report.Providers {
		connected := formatBool(provider.Connected)
		inventory := formatBool(provider.InventoryCreated)
		ready := formatBool(provider.Ready)

		name := provider.Name
		if len(name) > 18 {
			name = name[:15] + "..."
		}

		ns := provider.Namespace
		if len(ns) > 13 {
			ns = ns[:10] + "..."
		}

		sb.WriteString(fmt.Sprintf("  %-20s %-15s %-12s %-10s %-10s %-10s\n",
			name, ns, provider.Type, connected, inventory, ready))
	}

	sb.WriteString("\n")
	return sb.String()
}

func formatPlansSection(report *HealthReport) string {
	var sb strings.Builder

	unhealthy := 0
	for _, plan := range report.Plans {
		if !plan.Ready || plan.Status == "Failed" {
			unhealthy++
		}
	}

	sb.WriteString(output.Bold(output.Cyan(fmt.Sprintf("PLANS (%d total, %d with issues)", len(report.Plans), unhealthy))))
	sb.WriteString("\n")

	if len(report.Plans) == 0 {
		sb.WriteString("  No migration plans found\n")
		sb.WriteString("\n")
		return sb.String()
	}

	// Table header
	sb.WriteString(fmt.Sprintf("  %-25s %-15s %-12s %-8s %s\n",
		output.Bold("NAME"), output.Bold("NAMESPACE"), output.Bold("STATUS"),
		output.Bold("READY"), output.Bold("VMS")))

	for _, plan := range report.Plans {
		status := plan.Status
		switch status {
		case "Failed":
			status = output.Red(status)
		case "Succeeded":
			status = output.Green(status)
		case "Running", "Executing":
			status = output.Blue(status)
		}

		ready := formatBool(plan.Ready)

		vmInfo := fmt.Sprintf("%d", plan.VMCount)
		if plan.Failed > 0 {
			vmInfo = fmt.Sprintf("%d (F:%d)", plan.VMCount, plan.Failed)
			vmInfo = output.Red(vmInfo)
		} else if plan.Succeeded > 0 {
			vmInfo = fmt.Sprintf("%d (S:%d)", plan.VMCount, plan.Succeeded)
		}

		name := plan.Name
		if len(name) > 23 {
			name = name[:20] + "..."
		}

		ns := plan.Namespace
		if len(ns) > 13 {
			ns = ns[:10] + "..."
		}

		sb.WriteString(fmt.Sprintf("  %-25s %-15s %-12s %-8s %s\n",
			name, ns, status, ready, vmInfo))
	}

	sb.WriteString("\n")
	return sb.String()
}

func formatSummarySection(report *HealthReport) string {
	var sb strings.Builder
	sb.WriteString(output.Bold(output.Cyan("SUMMARY")))
	sb.WriteString("\n")

	// Overall Health
	healthStr := string(report.OverallStatus)
	switch report.OverallStatus {
	case HealthStatusHealthy:
		healthStr = output.Green(healthStr)
	case HealthStatusWarning:
		healthStr = output.Yellow(healthStr)
	case HealthStatusCritical:
		healthStr = output.Red(healthStr)
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", output.Bold("Overall Health:"), healthStr))

	// Issue counts
	sb.WriteString(fmt.Sprintf("  %s %d\n", output.Bold("Issues Found:"), report.Summary.TotalIssues))
	if report.Summary.CriticalIssues > 0 {
		sb.WriteString(fmt.Sprintf("    Critical: %s\n", output.Red(fmt.Sprintf("%d", report.Summary.CriticalIssues))))
	}
	if report.Summary.WarningIssues > 0 {
		sb.WriteString(fmt.Sprintf("    Warning:  %s\n", output.Yellow(fmt.Sprintf("%d", report.Summary.WarningIssues))))
	}

	// Recommendations
	if len(report.Issues) > 0 {
		sb.WriteString(fmt.Sprintf("  %s\n", output.Bold("Recommendations:")))
		for _, issue := range report.Issues {
			prefix := "  "
			switch issue.Severity {
			case SeverityCritical:
				prefix = output.Red("[CRITICAL]")
			case SeverityWarning:
				prefix = output.Yellow("[WARNING]")
			case SeverityInfo:
				prefix = output.Blue("[INFO]")
			}

			resource := ""
			if issue.Resource != "" {
				resource = issue.Resource + ": "
			}
			sb.WriteString(fmt.Sprintf("    %s %s%s\n", prefix, resource, issue.Message))
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("      %s\n", output.Cyan(issue.Suggestion)))
			}
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// Helper functions

func formatBool(b bool) string {
	if b {
		return output.Green("True")
	}
	return output.Red("False")
}

func formatBoolPtr(b *bool) string {
	if b == nil {
		return "[not set]"
	}
	if *b {
		return output.Green("True")
	}
	return output.Yellow("False")
}
