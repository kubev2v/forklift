package health

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// Log analysis patterns (pre-compiled for performance)
var (
	errorPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\berror\b`),
		regexp.MustCompile(`(?i)\bfailed\b`),
		regexp.MustCompile(`(?i)\bfatal\b`),
		regexp.MustCompile(`(?i)\bpanic\b`),
	}

	warningPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bwarn(ing)?\b`),
	}

	// Patterns to ignore (false positives)
	ignorePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)error.*nil`),
		regexp.MustCompile(`(?i)no error`),
		regexp.MustCompile(`(?i)error count.*0`),
	}
)

// DeploymentLogs maps deployment names for log analysis
var forkliftDeployments = []string{
	"forklift-controller",
	"forklift-api",
	"forklift-validation",
	"forklift-ui-plugin",
	"forklift-volume-populator-controller",
}

// CheckLogsHealth analyzes logs from Forklift operator component deployments.
//
// IMPORTANT: This function checks logs from Forklift OPERATOR pods (forklift-controller,
// forklift-api, forklift-validation, etc.) which ALWAYS run in the operator
// namespace. The caller (health.go) should pass the auto-detected operator
// namespace here, NOT a user-specified namespace.
func CheckLogsHealth(ctx context.Context, configFlags *genericclioptions.ConfigFlags, operatorNamespace string, logLines int) ([]LogAnalysis, error) {
	clientset, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes clientset: %v", err)
	}

	// Use provided operator namespace or fall back to default
	ns := operatorNamespace
	if ns == "" {
		ns = client.OpenShiftMTVNamespace
	}

	var analyses []LogAnalysis

	for _, deployment := range forkliftDeployments {
		// Get pods for this deployment
		pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", deployment),
		})
		if err != nil {
			continue
		}

		// If no pods found with specific label, try by name prefix
		if len(pods.Items) == 0 {
			allPods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, pod := range allPods.Items {
				if strings.HasPrefix(pod.Name, deployment) {
					pods.Items = append(pods.Items, pod)
					break // Just take the first matching pod
				}
			}
		}

		if len(pods.Items) == 0 {
			continue
		}

		// Analyze logs from the first pod
		pod := &pods.Items[0]
		analysis := analyzePodsLogs(ctx, clientset, pod, deployment, logLines)
		analyses = append(analyses, analysis)
	}

	return analyses, nil
}

// analyzePodsLogs analyzes logs from a single pod
func analyzePodsLogs(ctx context.Context, clientset *kubernetes.Clientset, pod *corev1.Pod, name string, logLines int) LogAnalysis {
	analysis := LogAnalysis{
		Name:       name,
		Errors:     0,
		Warnings:   0,
		ErrorLines: []string{},
		WarnLines:  []string{},
	}

	// Get logs for all containers in the pod
	for _, container := range pod.Spec.Containers {
		tailLines := int64(logLines)
		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: container.Name,
			TailLines: &tailLines,
		})

		logStream, err := req.Stream(ctx)
		if err != nil {
			continue
		}

		// Use anonymous function to ensure logStream.Close() is called via defer
		func() {
			defer logStream.Close()
			scanner := bufio.NewScanner(logStream)
			for scanner.Scan() {
				line := scanner.Text()
				analyzeLogLine(line, &analysis)
			}
			// Note: Scanner errors are intentionally ignored as partial
			// log analysis is acceptable for health checks
		}()
	}

	return analysis
}

// analyzeLogLine analyzes a single log line for errors and warnings
func analyzeLogLine(line string, analysis *LogAnalysis) {
	// Check if line should be ignored
	for _, pattern := range ignorePatterns {
		if pattern.MatchString(line) {
			return
		}
	}

	// Check for errors
	for _, pattern := range errorPatterns {
		if pattern.MatchString(line) {
			analysis.Errors++
			if len(analysis.ErrorLines) < 5 { // Keep only first 5 error lines
				analysis.ErrorLines = append(analysis.ErrorLines, truncateLine(line, 200))
			}
			return // Count as error only once
		}
	}

	// Check for warnings
	for _, pattern := range warningPatterns {
		if pattern.MatchString(line) {
			analysis.Warnings++
			if len(analysis.WarnLines) < 5 { // Keep only first 5 warning lines
				analysis.WarnLines = append(analysis.WarnLines, truncateLine(line, 200))
			}
			return // Count as warning only once
		}
	}
}

// truncateLine truncates a line to the specified length
func truncateLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}

// AnalyzeLogsHealth adds log-related issues to the report
func AnalyzeLogsHealth(analyses []LogAnalysis, report *HealthReport) {
	for _, analysis := range analyses {
		if analysis.Errors > 10 {
			report.AddIssue(
				SeverityWarning,
				"Logs",
				analysis.Name,
				fmt.Sprintf("High error count in logs: %d errors", analysis.Errors),
				fmt.Sprintf("Check %s logs for details", analysis.Name),
			)
		} else if analysis.Errors > 0 {
			report.AddIssue(
				SeverityInfo,
				"Logs",
				analysis.Name,
				fmt.Sprintf("%d errors found in recent logs", analysis.Errors),
				"",
			)
		}

		if analysis.Warnings > 20 {
			report.AddIssue(
				SeverityInfo,
				"Logs",
				analysis.Name,
				fmt.Sprintf("High warning count in logs: %d warnings", analysis.Warnings),
				fmt.Sprintf("Review %s logs for potential issues", analysis.Name),
			)
		}
	}
}
