package diagnostics

import (
	"bufio"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const defaultLogTailLines = 500
const defaultShowLines = 10

// MaxLogTailLines is the maximum number of log lines that can be scanned per container.
const MaxLogTailLines = 10000

// MaxShowLines is the maximum number of log lines that can be displayed per container.
const MaxShowLines = 500

// CollectPodDiagnostics lists pods matching the plan/migration labels and collects logs.
func CollectPodDiagnostics(ctx context.Context, clientset *kubernetes.Clientset, namespace, planUID, migrationUID, vmID string, logLines, showLines int) []PodDiagnostics {
	selector := fmt.Sprintf("plan=%s,migration=%s", planUID, migrationUID)
	if vmID != "" {
		selector += fmt.Sprintf(",vmID=%s", vmID)
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(pods.Items) == 0 {
		return nil
	}

	var results []PodDiagnostics
	for i := range pods.Items {
		pod := &pods.Items[i]
		diag := buildPodDiagnostics(ctx, clientset, pod, logLines, showLines)
		results = append(results, diag)
	}
	return results
}

// CollectPodDiagnosticsByName fetches diagnostics for a specific pod by name.
func CollectPodDiagnosticsByName(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, logLines, showLines int) *PodDiagnostics {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	diag := buildPodDiagnostics(ctx, clientset, pod, logLines, showLines)
	return &diag
}

func buildPodDiagnostics(ctx context.Context, clientset *kubernetes.Clientset, pod *corev1.Pod, logLines, showLines int) PodDiagnostics {
	phase := string(pod.Status.Phase)
	reason := pod.Status.Reason
	if reason == "Evicted" || (pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted") {
		phase = "Evicted"
	}

	containerName := mainContainerName(pod)

	diag := PodDiagnostics{
		Name:      pod.Name,
		Phase:     phase,
		Reason:    reason,
		Container: containerName,
	}

	var allLines []string
	diag.LogTail, diag.ErrorLines, diag.ErrorCount, diag.WarnCount, allLines = collectLogs(ctx, clientset, pod.Namespace, pod.Name, containerName, logLines, showLines)

	if isV2VContainer(containerName) {
		diag.V2VStage, diag.ProgressPct = detectV2VStage(allLines)
		if diag.V2VStage == "" && len(allLines) > 0 {
			diag.V2VStage = "init"
		}
	}

	return diag
}

func isV2VContainer(name string) bool {
	switch name {
	case "virt-v2v", "convertor":
		return true
	}
	return false
}

func mainContainerName(pod *corev1.Pod) string {
	if len(pod.Spec.Containers) == 0 {
		return ""
	}
	// Prefer known container names for migration workloads
	for _, c := range pod.Spec.Containers {
		switch c.Name {
		case "importer", "virt-v2v", "convertor":
			return c.Name
		}
	}
	return pod.Spec.Containers[0].Name
}

func collectLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, container string, logLines, showLines int) ([]string, []string, int, int, []string) {
	if container == "" {
		return nil, nil, 0, 0, nil
	}

	tailLines := int64(logLines)
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		req = clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Container: container,
			TailLines: &tailLines,
			Previous:  true,
		})
		stream, err = req.Stream(ctx)
		if err != nil {
			return nil, nil, 0, 0, nil
		}
	}
	defer stream.Close()

	var lines []string
	var rootCauseLines []string
	var otherErrorLines []string
	var errorCount, warnCount int

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

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
	if err := scanner.Err(); err != nil {
		lines = append(lines, fmt.Sprintf("[log scan incomplete: %v]", err))
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

	// Keep all lines for stage detection
	allLines := lines

	// Keep only last N lines for tail display
	if len(lines) > showLines {
		lines = lines[len(lines)-showLines:]
	}

	return lines, errorLines, errorCount, warnCount, allLines
}

func isErrorLine(line string) bool {
	for _, p := range errorPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

func isRootCauseLine(line string) bool {
	for _, p := range rootCausePatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

func isIgnoredLine(line string) bool {
	for _, p := range ignorePatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

func isWarnLine(line string) bool {
	for _, p := range warningPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}
