package health

import (
	"context"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// RunHealthCheck performs all health checks and returns a complete health report
func RunHealthCheck(ctx context.Context, configFlags *genericclioptions.ConfigFlags, opts HealthCheckOptions) (*HealthReport, error) {
	report := NewHealthReport()

	// 1. Check operator health
	report.Operator = CheckOperatorHealth(ctx, configFlags)

	// ==========================================================================
	// IMPORTANT: Two different namespace concepts are used in health checks:
	//
	// 1. operatorNamespace - ALWAYS auto-detected, where Forklift operator runs.
	//    Used for: ForkliftController, Forklift component pods, Forklift logs.
	//    These components ONLY exist in the operator namespace.
	//
	// 2. userNamespace (opts.Namespace) + opts.AllNamespaces - user-specified.
	//    Used for: Providers, Plans (and their related conversion pods).
	//    These user resources can exist in any namespace.
	// ==========================================================================

	// Auto-detect operator namespace from the operator health check
	// If detection failed (error or not found), fall back to default and warn
	operatorNamespace := report.Operator.Namespace
	if operatorNamespace == "" {
		operatorNamespace = "openshift-mtv" // Default fallback
		if report.Operator.Error != "" {
			report.AddIssue(
				SeverityInfo,
				"Operator",
				"",
				fmt.Sprintf("Could not auto-detect operator namespace (%s), using default 'openshift-mtv'", report.Operator.Error),
				"Ensure you have permissions to read CRDs or specify namespace with -n",
			)
		}
	}

	// User-specified namespace for providers/plans (empty means use operatorNamespace as default)
	userNamespace := opts.Namespace

	// If operator is not installed and no error (genuinely not found), we can't do much more
	if !report.Operator.Installed && report.Operator.Error == "" {
		report.AddIssue(
			SeverityCritical,
			"Operator",
			"MTV Operator",
			"MTV Operator is not installed",
			"Install the MTV/Forklift operator first",
		)
		report.CalculateOverallStatus()
		report.CalculateSummary()
		return report, nil
	}

	// If there was an error checking operator, continue with best effort using default namespace
	if report.Operator.Error != "" && !report.Operator.Installed {
		report.AddIssue(
			SeverityWarning,
			"Operator",
			"",
			fmt.Sprintf("Could not verify operator installation: %s", report.Operator.Error),
			"Check cluster connectivity and RBAC permissions",
		)
		// Continue with best effort - operator might be installed but we couldn't verify
	}

	// 2. Check providers (user resources - can be in any namespace)
	// Uses userNamespace if specified, or operatorNamespace as default, or all namespaces if requested
	//
	// NOTE: hasVSphereProvider and hasRemoteOpenShiftProvider are derived from the
	// providers in the checked namespace(s). Warnings about VDDK or live migration
	// configuration will only appear if such providers exist in the scoped namespace(s).
	// Use -A to check all namespaces if you want cluster-wide provider detection.
	var hasVSphereProvider, hasRemoteOpenShiftProvider bool
	providerNS := userNamespace
	if providerNS == "" {
		providerNS = operatorNamespace
	}
	providerResult, err := CheckProvidersHealth(ctx, configFlags, providerNS, opts.AllNamespaces)
	if err != nil {
		// If all-namespaces query failed, try falling back to operator namespace only
		if opts.AllNamespaces {
			report.AddIssue(
				SeverityInfo,
				"Providers",
				"",
				"Cannot list providers across all namespaces (RBAC?), falling back to operator namespace",
				"Request cluster-wide read permissions for providers.forklift.konveyor.io",
			)
			// Fallback: try operator namespace only
			providerResult, err = CheckProvidersHealth(ctx, configFlags, operatorNamespace, false)
		}
		if err != nil {
			report.AddIssue(
				SeverityWarning,
				"Providers",
				"",
				fmt.Sprintf("Failed to check providers: %v", err),
				"",
			)
			// Use safe defaults when provider check fails
			hasVSphereProvider = false
			hasRemoteOpenShiftProvider = false
		} else {
			report.Providers = providerResult.Providers
			AnalyzeProvidersHealth(providerResult.Providers, report)
			hasVSphereProvider = providerResult.HasVSphereProvider
			hasRemoteOpenShiftProvider = providerResult.HasRemoteOpenShiftProvider
		}
	} else {
		report.Providers = providerResult.Providers
		AnalyzeProvidersHealth(providerResult.Providers, report)
		hasVSphereProvider = providerResult.HasVSphereProvider
		hasRemoteOpenShiftProvider = providerResult.HasRemoteOpenShiftProvider
	}

	// 3. Check controller health (operator component - ALWAYS in operatorNamespace)
	controller, err := CheckControllerHealth(ctx, configFlags, operatorNamespace, hasVSphereProvider, hasRemoteOpenShiftProvider)
	if err != nil {
		report.AddIssue(
			SeverityWarning,
			"Controller",
			"",
			fmt.Sprintf("Failed to check controller: %v", err),
			"Check cluster connectivity and RBAC permissions",
		)
	}
	report.Controller = controller
	AnalyzeControllerHealth(&report.Controller, report)

	// 4. Check pods health (operator components - ALWAYS in operatorNamespace)
	pods, err := CheckPodsHealth(ctx, configFlags, operatorNamespace)
	if err != nil {
		report.AddIssue(
			SeverityWarning,
			"Pods",
			"",
			fmt.Sprintf("Failed to check pods: %v", err),
			"",
		)
	} else {
		report.Pods = pods
		AnalyzePodsHealth(pods, report)
	}

	// 5. Check logs (operator components - ALWAYS in operatorNamespace)
	if opts.CheckLogs {
		logLines := opts.LogLines
		if logLines <= 0 {
			logLines = 100
		}
		analyses, err := CheckLogsHealth(ctx, configFlags, operatorNamespace, logLines)
		if err != nil {
			if opts.Verbose {
				report.AddIssue(
					SeverityInfo,
					"Logs",
					"",
					fmt.Sprintf("Failed to analyze logs: %v", err),
					"",
				)
			}
		} else {
			report.LogAnalysis = analyses
			AnalyzeLogsHealth(analyses, report)
		}
	}

	// 6. Check plans (user resources - can be in any namespace)
	// Uses userNamespace if specified, or operatorNamespace as default, or all namespaces if requested
	planNS := userNamespace
	if planNS == "" {
		planNS = operatorNamespace
	}
	plans, err := CheckPlansHealth(ctx, configFlags, planNS, opts.AllNamespaces)
	if err != nil {
		// If all-namespaces query failed, try falling back to operator namespace only
		if opts.AllNamespaces {
			report.AddIssue(
				SeverityInfo,
				"Plans",
				"",
				"Cannot list plans across all namespaces (RBAC?), falling back to operator namespace",
				"Request cluster-wide read permissions for plans.forklift.konveyor.io",
			)
			// Fallback: try operator namespace only
			plans, err = CheckPlansHealth(ctx, configFlags, operatorNamespace, false)
		}
		if err != nil {
			report.AddIssue(
				SeverityWarning,
				"Plans",
				"",
				fmt.Sprintf("Failed to check plans: %v", err),
				"",
			)
		} else {
			report.Plans = plans
			AnalyzePlansHealth(plans, report)
		}
	} else {
		report.Plans = plans
		AnalyzePlansHealth(plans, report)
	}

	// Calculate overall status and summary
	report.CalculateOverallStatus()
	report.CalculateSummary()
	report.GenerateRecommendations()

	return report, nil
}

// PrintHealthReport formats and prints the health report
func PrintHealthReport(report *HealthReport, outputFormat string) error {
	output, err := FormatReport(report, outputFormat)
	if err != nil {
		return fmt.Errorf("failed to format report: %v", err)
	}
	fmt.Print(output)
	return nil
}
