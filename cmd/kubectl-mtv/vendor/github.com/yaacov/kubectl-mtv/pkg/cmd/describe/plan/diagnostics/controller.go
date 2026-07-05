package diagnostics

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

const controllerLogTailLines = 500
const maxControllerLogEntries = 10

// ControllerLogEntry holds a single relevant line from the forklift-controller logs.
type ControllerLogEntry struct {
	Line string
}

// CollectControllerLogs tails the forklift-controller pod logs and filters for
// lines mentioning the plan name or plan UID.
func CollectControllerLogs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, clientset *kubernetes.Clientset, planName, planUID string) []ControllerLogEntry {
	operatorNS := client.GetMTVOperatorNamespace(ctx, configFlags)

	pod := findControllerPod(ctx, clientset, operatorNS)
	if pod == nil {
		return nil
	}

	containerName := ""
	// Prefer known controller container names
	for _, c := range pod.Spec.Containers {
		switch c.Name {
		case "main", "forklift-controller", "controller":
			containerName = c.Name
		}
	}
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	tailLines := int64(controllerLogTailLines)
	req := clientset.CoreV1().Pods(operatorNS).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return nil
	}
	defer stream.Close()

	var entries []ControllerLogEntry
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		if matchesPlan(line, planName, planUID) {
			entries = append(entries, ControllerLogEntry{Line: line})
			if len(entries) >= maxControllerLogEntries {
				break
			}
		}
	}

	return entries
}

func findControllerPod(ctx context.Context, clientset *kubernetes.Clientset, namespace string) *corev1.Pod {
	// Try the common label selectors for the forklift controller
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

// FormatControllerLogLines formats the controller log entries for display.
func FormatControllerLogLines(entries []ControllerLogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		line := e.Line
		if len(line) > 200 {
			line = line[:197] + "..."
		}
		lines = append(lines, line)
	}
	return fmt.Sprintf("%d relevant lines from forklift-controller:\n%s", len(entries), strings.Join(lines, "\n"))
}
