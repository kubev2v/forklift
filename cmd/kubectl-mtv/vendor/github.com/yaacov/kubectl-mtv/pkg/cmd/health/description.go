package health

import (
	"fmt"
	"strings"

	"github.com/yaacov/kubectl-mtv/pkg/util/describe"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
)

// ToDescription converts a HealthReport into a describe.Description
// that can be rendered in any supported format (table, json, yaml, markdown).
func (r *HealthReport) ToDescription() *describe.Description {
	b := describe.NewBuilder("MTV HEALTH REPORT")

	r.buildOperatorSection(b)
	r.buildControllerSection(b)
	r.buildPodsSection(b)
	r.buildLogAnalysisSection(b)
	r.buildProvidersSection(b)
	r.buildPlansSection(b)
	r.buildSummarySection(b)

	return b.Build()
}

func (r *HealthReport) buildOperatorSection(b *describe.Builder) {
	b.Section("OPERATOR STATUS")

	op := r.Operator
	if op.Installed {
		b.FieldC("MTV Operator", "Installed", output.Green)
		if op.Version != "" && op.Version != "unknown" {
			b.Field("Version", op.Version)
		}
		b.Field("Namespace", op.Namespace)
	} else if op.Error != "" {
		b.FieldC("MTV Operator", op.Error, output.Red)
	} else if op.Status == "Unknown" {
		b.FieldC("MTV Operator", op.Status, output.Yellow)
	} else {
		b.FieldC("MTV Operator", "Not Installed", output.Red)
	}
}

func (r *HealthReport) buildControllerSection(b *describe.Builder) {
	b.Section("FORKLIFT CONTROLLER")

	ctrl := r.Controller
	if !ctrl.Found {
		msg := "ForkliftController not found"
		if ctrl.Error != "" {
			msg = ctrl.Error
		}
		b.FieldC("Status", msg, output.Red)
		return
	}

	b.Field("Name", ctrl.Name)

	// Feature flags as a sub-section
	b.SubSection("Feature Flags")
	b.Field("UI Plugin", boolPtrStr(ctrl.FeatureFlags.UIPlugin))
	b.Field("Validation", boolPtrStr(ctrl.FeatureFlags.Validation))
	b.Field("Volume Pop", boolPtrStr(ctrl.FeatureFlags.VolumePopulator))
	b.Field("Auth Required", boolPtrStr(ctrl.FeatureFlags.AuthRequired))

	if ctrl.HasRemoteOpenShiftProvider || ctrl.FeatureFlags.OCPLiveMigration != nil {
		val := boolPtrStr(ctrl.FeatureFlags.OCPLiveMigration)
		if ctrl.HasRemoteOpenShiftProvider && (ctrl.FeatureFlags.OCPLiveMigration == nil || !*ctrl.FeatureFlags.OCPLiveMigration) {
			val = "[not set] *** Remote OpenShift provider exists!"
		}
		b.FieldC("OCP Live Mig", val, colorBoolPtrWarning(ctrl.FeatureFlags.OCPLiveMigration, ctrl.HasRemoteOpenShiftProvider))
	}
	b.EndSubSection()

	// Custom images
	if len(ctrl.CustomImages) > 0 {
		b.Field("Custom Images", fmt.Sprintf("%d overrides", len(ctrl.CustomImages)))
		for _, img := range ctrl.CustomImages {
			b.FieldC(img.Field, img.Image, output.Blue)
		}
	} else {
		b.Field("Custom Images", "None")
	}

	// VDDK
	if ctrl.VDDKImage != "" {
		b.FieldC("VDDK Image", ctrl.VDDKImage, output.Green)
	} else if ctrl.HasVSphereProvider {
		b.FieldC("VDDK Image", "[NOT SET] *** WARNING: vSphere providers exist!", output.Red)
	} else {
		b.Field("VDDK Image", "[not set]")
	}

	if ctrl.LogLevel > 0 {
		b.Field("Log Level", fmt.Sprintf("%d", ctrl.LogLevel))
	}
}

func (r *HealthReport) buildPodsSection(b *describe.Builder) {
	unhealthy := 0
	for _, pod := range r.Pods {
		if !pod.Ready || pod.Status != "Running" || len(pod.Issues) > 0 {
			unhealthy++
		}
	}

	b.Section(fmt.Sprintf("FORKLIFT PODS (%d total, %d unhealthy)", len(r.Pods), unhealthy))

	if len(r.Pods) == 0 {
		b.Field("Status", "No Forklift pods found")
		return
	}

	headers := []describe.TableColumn{
		{Display: "NAME", Key: "name"},
		{Display: "STATUS", Key: "status", ColorFunc: output.ColorizeStatus},
		{Display: "RESTARTS", Key: "restarts"},
		{Display: "ISSUES", Key: "issues"},
	}

	rows := make([]map[string]string, 0, len(r.Pods))
	for _, pod := range r.Pods {
		issues := "None"
		if len(pod.Issues) > 0 {
			issues = strings.Join(pod.Issues, ", ")
		}
		rows = append(rows, map[string]string{
			"name":     pod.Name,
			"status":   pod.Status,
			"restarts": fmt.Sprintf("%d", pod.Restarts),
			"issues":   issues,
		})
	}

	b.Table(headers, rows)
}

func (r *HealthReport) buildLogAnalysisSection(b *describe.Builder) {
	if len(r.LogAnalysis) == 0 {
		return
	}

	b.Section("POD LOG ANALYSIS")

	headers := []describe.TableColumn{
		{Display: "NAME", Key: "name"},
		{Display: "ERRORS", Key: "errors"},
		{Display: "WARNINGS", Key: "warnings"},
	}

	rows := make([]map[string]string, 0, len(r.LogAnalysis))
	for _, a := range r.LogAnalysis {
		rows = append(rows, map[string]string{
			"name":     a.Name,
			"errors":   fmt.Sprintf("%d", a.Errors),
			"warnings": fmt.Sprintf("%d", a.Warnings),
		})
	}

	b.Table(headers, rows)
}

func (r *HealthReport) buildProvidersSection(b *describe.Builder) {
	unhealthy := 0
	for _, p := range r.Providers {
		if !p.Ready {
			unhealthy++
		}
	}

	b.Section(fmt.Sprintf("PROVIDERS (%d total, %d unhealthy)", len(r.Providers), unhealthy))

	if len(r.Providers) == 0 {
		b.Field("Status", "No providers found")
		return
	}

	headers := []describe.TableColumn{
		{Display: "NAME", Key: "name"},
		{Display: "NAMESPACE", Key: "namespace"},
		{Display: "TYPE", Key: "type"},
		{Display: "CONNECTED", Key: "connected", ColorFunc: output.ColorizeBooleanString},
		{Display: "INVENTORY", Key: "inventory", ColorFunc: output.ColorizeBooleanString},
		{Display: "READY", Key: "ready", ColorFunc: output.ColorizeBooleanString},
	}

	rows := make([]map[string]string, 0, len(r.Providers))
	for _, p := range r.Providers {
		rows = append(rows, map[string]string{
			"name":      p.Name,
			"namespace": p.Namespace,
			"type":      p.Type,
			"connected": fmt.Sprintf("%t", p.Connected),
			"inventory": fmt.Sprintf("%t", p.InventoryCreated),
			"ready":     fmt.Sprintf("%t", p.Ready),
		})
	}

	b.Table(headers, rows)
}

func (r *HealthReport) buildPlansSection(b *describe.Builder) {
	unhealthy := 0
	for _, p := range r.Plans {
		if !p.Ready || p.Status == "Failed" {
			unhealthy++
		}
	}

	b.Section(fmt.Sprintf("PLANS (%d total, %d with issues)", len(r.Plans), unhealthy))

	if len(r.Plans) == 0 {
		b.Field("Status", "No migration plans found")
		return
	}

	headers := []describe.TableColumn{
		{Display: "NAME", Key: "name"},
		{Display: "NAMESPACE", Key: "namespace"},
		{Display: "STATUS", Key: "status", ColorFunc: output.ColorizeStatus},
		{Display: "READY", Key: "ready", ColorFunc: output.ColorizeBooleanString},
		{Display: "VMS", Key: "vms"},
	}

	rows := make([]map[string]string, 0, len(r.Plans))
	for _, p := range r.Plans {
		vmInfo := fmt.Sprintf("%d", p.VMCount)
		if p.Failed > 0 {
			vmInfo = fmt.Sprintf("%d (F:%d)", p.VMCount, p.Failed)
		} else if p.Succeeded > 0 {
			vmInfo = fmt.Sprintf("%d (S:%d)", p.VMCount, p.Succeeded)
		}
		rows = append(rows, map[string]string{
			"name":      p.Name,
			"namespace": p.Namespace,
			"status":    p.Status,
			"ready":     fmt.Sprintf("%t", p.Ready),
			"vms":       vmInfo,
		})
	}

	b.Table(headers, rows)
}

func (r *HealthReport) buildSummarySection(b *describe.Builder) {
	b.Section("SUMMARY")

	healthStr := string(r.OverallStatus)
	var colorFunc func(string) string
	switch r.OverallStatus {
	case HealthStatusHealthy:
		colorFunc = output.Green
	case HealthStatusWarning:
		colorFunc = output.Yellow
	case HealthStatusCritical:
		colorFunc = output.Red
	}
	b.FieldC("Overall Health", healthStr, colorFunc)
	b.Field("Issues Found", fmt.Sprintf("%d", r.Summary.TotalIssues))

	if r.Summary.CriticalIssues > 0 {
		b.FieldC("Critical", fmt.Sprintf("%d", r.Summary.CriticalIssues), output.Red)
	}
	if r.Summary.WarningIssues > 0 {
		b.FieldC("Warning", fmt.Sprintf("%d", r.Summary.WarningIssues), output.Yellow)
	}

	if len(r.Issues) > 0 {
		b.SubSection("Recommendations")
		for _, issue := range r.Issues {
			prefix := "[INFO]"
			var cfn func(string) string
			switch issue.Severity {
			case SeverityCritical:
				prefix = "[CRITICAL]"
				cfn = output.Red
			case SeverityWarning:
				prefix = "[WARNING]"
				cfn = output.Yellow
			case SeverityInfo:
				cfn = output.Blue
			}

			msg := prefix + " "
			if issue.Resource != "" {
				msg += issue.Resource + ": "
			}
			msg += issue.Message
			if issue.Suggestion != "" {
				msg += " -- " + issue.Suggestion
			}
			b.FieldC("", msg, cfn)
		}
		b.EndSubSection()
	}
}

// boolPtrStr formats a *bool for display.
func boolPtrStr(b *bool) string {
	if b == nil {
		return "[not set]"
	}
	if *b {
		return "True"
	}
	return "False"
}

// colorBoolPtrWarning returns a color function appropriate for the OCP Live Migration field.
func colorBoolPtrWarning(val *bool, hasRemote bool) func(string) string {
	if hasRemote && (val == nil || !*val) {
		return output.Red
	}
	if val != nil && *val {
		return output.Green
	}
	return nil
}
