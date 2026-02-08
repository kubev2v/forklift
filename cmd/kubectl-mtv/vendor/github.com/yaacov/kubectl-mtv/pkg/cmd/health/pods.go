package health

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// ForkliftPodLabelSelector is the label selector used to identify Forklift pods
const ForkliftPodLabelSelector = "app=forklift"

// CheckPodsHealth checks the health of Forklift operator component pods.
//
// IMPORTANT: This function checks Forklift OPERATOR pods (forklift-controller,
// forklift-api, forklift-validation, etc.) which ALWAYS run in the operator
// namespace. The caller (health.go) should pass the auto-detected operator
// namespace here, NOT a user-specified namespace.
//
// User workload pods (like conversion pods) are NOT checked here - they are
// associated with Plans and Providers which can be in any namespace.
func CheckPodsHealth(ctx context.Context, configFlags *genericclioptions.ConfigFlags, operatorNamespace string) ([]PodHealth, error) {
	clientset, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes clientset: %v", err)
	}

	// Use provided operator namespace or fall back to default
	ns := operatorNamespace
	if ns == "" {
		ns = client.OpenShiftMTVNamespace
	}

	// List pods with forklift labels
	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: ForkliftPodLabelSelector,
	})

	var forkliftPods []corev1.Pod
	if err == nil {
		// Label selector query succeeded, use these pods directly (no filtering needed)
		forkliftPods = pods.Items
	} else {
		// Fallback: try without label selector and filter manually
		pods, err = clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list pods: %v", err)
		}
		// Filter to only forklift-related pods
		for _, pod := range pods.Items {
			if isForkliftPod(&pod) {
				forkliftPods = append(forkliftPods, pod)
			}
		}
	}

	var podHealths []PodHealth
	for _, pod := range forkliftPods {
		podHealth := analyzePod(&pod)
		podHealths = append(podHealths, podHealth)
	}

	return podHealths, nil
}

// isForkliftPod checks if a pod is a Forklift-related pod
func isForkliftPod(pod *corev1.Pod) bool {
	name := pod.Name
	// Check by name prefix
	forkliftPrefixes := []string{
		"forklift-",
	}

	for _, prefix := range forkliftPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}

	// Check by labels
	if labels := pod.Labels; labels != nil {
		if labels["app.kubernetes.io/part-of"] == "forklift" {
			return true
		}
		if labels["app"] == "forklift" {
			return true
		}
	}

	return false
}

// analyzePod analyzes a single pod and returns its health status
func analyzePod(pod *corev1.Pod) PodHealth {
	health := PodHealth{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    string(pod.Status.Phase),
		Ready:     isPodReady(pod),
		Restarts:  getTotalRestarts(pod),
		Age:       formatAge(pod.CreationTimestamp.Time),
		Issues:    []string{},
	}

	// Check for terminated containers
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.LastTerminationState.Terminated != nil {
			health.TerminatedReason = cs.LastTerminationState.Terminated.Reason
			if health.TerminatedReason == "OOMKilled" {
				health.Issues = append(health.Issues, "Container was OOMKilled")
			}
		}
	}

	// Check for high restart count
	if health.Restarts > 5 {
		health.Issues = append(health.Issues, fmt.Sprintf("High restart count: %d", health.Restarts))
	}

	// Check for pending state
	if pod.Status.Phase == corev1.PodPending {
		health.Issues = append(health.Issues, "Pod is in Pending state")
		// Check for scheduling issues
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
				health.Issues = append(health.Issues, "Scheduling failed: "+condition.Reason)
			}
		}
	}

	// Check for failed state
	if pod.Status.Phase == corev1.PodFailed {
		health.Issues = append(health.Issues, "Pod is in Failed state")
	}

	// Check for not ready
	if !health.Ready && pod.Status.Phase == corev1.PodRunning {
		health.Issues = append(health.Issues, "Pod is running but not ready")
	}

	return health
}

// isPodReady checks if all containers in a pod are ready
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// getTotalRestarts returns the total restart count across all containers
func getTotalRestarts(pod *corev1.Pod) int {
	total := 0
	for _, cs := range pod.Status.ContainerStatuses {
		total += int(cs.RestartCount)
	}
	return total
}

// formatAge formats the age of a pod
func formatAge(creationTime time.Time) string {
	duration := time.Since(creationTime)

	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	} else if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

// AnalyzePodsHealth analyzes pod health and adds issues to the report
func AnalyzePodsHealth(pods []PodHealth, report *HealthReport) {
	for _, pod := range pods {
		// Check for critical issues
		if pod.Status == "Failed" {
			report.AddIssue(
				SeverityCritical,
				"Pod",
				pod.Name,
				"Pod is in Failed state",
				"Check pod logs and events: kubectl describe pod "+pod.Name,
			)
		}

		// Check for OOMKilled
		if pod.TerminatedReason == "OOMKilled" {
			report.AddIssue(
				SeverityCritical,
				"Pod",
				pod.Name,
				"Container was OOMKilled - memory limit too low",
				"Increase memory limits for the pod",
			)
		}

		// Check for high restarts
		if pod.Restarts > 5 {
			report.AddIssue(
				SeverityWarning,
				"Pod",
				pod.Name,
				fmt.Sprintf("High restart count (%d) - indicates instability", pod.Restarts),
				"Check pod logs for crash reasons",
			)
		}

		// Check for pending pods
		if pod.Status == "Pending" {
			report.AddIssue(
				SeverityWarning,
				"Pod",
				pod.Name,
				"Pod is stuck in Pending state",
				"Check node resources and scheduling constraints",
			)
		}

		// Check for not ready pods
		if !pod.Ready && pod.Status == "Running" {
			report.AddIssue(
				SeverityWarning,
				"Pod",
				pod.Name,
				"Pod is running but not ready",
				"Check readiness probe failures",
			)
		}
	}
}
