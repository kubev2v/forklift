package diagnostics

import (
	"bufio"
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CollectControllerLogs tails the forklift-controller pod logs, filters for
// lines mentioning the plan name or plan UID, and applies error pattern analysis.
func CollectControllerLogs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, clientset *kubernetes.Clientset, planName, planUID string, logLines, showLines int) *ControllerLogAnalysis {
	operatorNS := client.GetMTVOperatorNamespace(ctx, configFlags)

	pod := findControllerPod(ctx, clientset, operatorNS)
	if pod == nil {
		return nil
	}

	containerName := ""
	for _, c := range pod.Spec.Containers {
		switch c.Name {
		case "main", "forklift-controller", "controller":
			containerName = c.Name
		}
	}
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	tailLines := int64(logLines)
	req := clientset.CoreV1().Pods(operatorNS).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return nil
	}
	defer stream.Close()

	var relevantLines []string
	var rootCauseLines []string
	var otherErrorLines []string
	var errorCount, warnCount int

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !matchesPlan(line, planName, planUID) {
			continue
		}

		relevantLines = append(relevantLines, line)

		if isIgnoredLine(line) {
			continue
		}
		if isRootCauseLine(line) {
			errorCount++
			rootCauseLines = append(rootCauseLines, line)
		} else if isErrorLine(line) {
			errorCount++
			otherErrorLines = append(otherErrorLines, line)
		} else if isWarnLine(line) {
			warnCount++
		}
	}
	_ = scanner.Err()

	if len(relevantLines) == 0 {
		return nil
	}

	// Build significant error lines: root-cause first, then other errors (capped)
	var errorLines []string
	errorLines = append(errorLines, rootCauseLines...)
	remaining := showLines - len(errorLines)
	if remaining > 0 && len(otherErrorLines) > 0 {
		if len(otherErrorLines) > remaining {
			otherErrorLines = otherErrorLines[len(otherErrorLines)-remaining:]
		}
		errorLines = append(errorLines, otherErrorLines...)
	}
	if len(errorLines) > showLines {
		errorLines = errorLines[len(errorLines)-showLines:]
	}

	// Keep only last N relevant lines for tail display
	if len(relevantLines) > showLines {
		relevantLines = relevantLines[len(relevantLines)-showLines:]
	}

	return &ControllerLogAnalysis{
		LogTail:    relevantLines,
		ErrorLines: errorLines,
		ErrorCount: errorCount,
		WarnCount:  warnCount,
	}
}

func findControllerPod(ctx context.Context, clientset *kubernetes.Clientset, namespace string) *corev1.Pod {
	selectors := []string{
		"app=forklift,control-plane=controller-manager",
		"app.kubernetes.io/name=forklift-controller",
	}

	for _, sel := range selectors {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: sel,
		})
		if err == nil && len(pods.Items) > 0 {
			for i := range pods.Items {
				if pods.Items[i].Status.Phase == corev1.PodRunning {
					return &pods.Items[i]
				}
			}
			return &pods.Items[0]
		}
	}

	// Fallback: find by name prefix
	allPods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for i := range allPods.Items {
		if strings.HasPrefix(allPods.Items[i].Name, "forklift-controller") &&
			allPods.Items[i].Status.Phase == corev1.PodRunning {
			return &allPods.Items[i]
		}
	}
	return nil
}

func matchesPlan(line, planName, planUID string) bool {
	if planName != "" && strings.Contains(line, planName) {
		return true
	}
	if planUID != "" && strings.Contains(line, planUID) {
		return true
	}
	return false
}
