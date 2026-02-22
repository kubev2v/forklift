package health

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

const forkliftControllerDeployment = "forklift-controller"

// FetchControllerLogs retrieves raw log text from the forklift-controller deployment.
// The container parameter accepts the aliases "controller" (main container) or
// "inventory", as well as any exact container name; see resolveContainerName.
// It auto-detects the operator namespace if operatorNamespace is empty.
func FetchControllerLogs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, operatorNamespace string, container string, tailLines int, since string) (string, error) {
	clientset, err := client.GetKubernetesClientset(configFlags)
	if err != nil {
		return "", fmt.Errorf("failed to get kubernetes clientset: %v", err)
	}

	ns := operatorNamespace
	if ns == "" {
		ns = client.GetMTVOperatorNamespace(ctx, configFlags)
	}

	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", forkliftControllerDeployment),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %v", err)
	}

	if len(pods.Items) == 0 {
		allPods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to list pods: %v", err)
		}
		for i := range allPods.Items {
			if strings.HasPrefix(allPods.Items[i].Name, forkliftControllerDeployment) {
				pods.Items = append(pods.Items, allPods.Items[i])
				break
			}
		}
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no forklift-controller pods found in namespace %s", ns)
	}

	pod := &pods.Items[0]

	containerName := resolveContainerName(pod, container)
	if containerName == "" {
		available := availableContainerNames(pod)
		return "", fmt.Errorf("container %q not found in pod %s (available: %s)", container, pod.Name, strings.Join(available, ", "))
	}

	tail := int64(tailLines)
	if tail <= 0 {
		tail = 200
	}

	logOpts := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  &tail,
		Timestamps: true,
	}

	if since != "" {
		duration, err := time.ParseDuration(since)
		if err != nil {
			return "", fmt.Errorf("invalid --since value %q: %v", since, err)
		}
		sinceSeconds := int64(duration.Seconds())
		logOpts.SinceSeconds = &sinceSeconds
	}

	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts)
	logStream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream logs from %s/%s: %v", pod.Name, containerName, err)
	}
	defer logStream.Close()

	data, err := io.ReadAll(logStream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %v", err)
	}

	return string(data), nil
}

// resolveContainerName maps the user-friendly container alias to the actual
// container name in the pod spec. For "controller", it picks the first
// non-"inventory" container (typically named "main" or the deployment name).
func resolveContainerName(pod *corev1.Pod, alias string) string {
	switch alias {
	case "inventory":
		for _, c := range pod.Spec.Containers {
			if c.Name == "inventory" {
				return c.Name
			}
		}
		return ""
	case "controller":
		for _, c := range pod.Spec.Containers {
			if c.Name != "inventory" {
				return c.Name
			}
		}
		return ""
	default:
		for _, c := range pod.Spec.Containers {
			if c.Name == alias {
				return c.Name
			}
		}
		return ""
	}
}

func availableContainerNames(pod *corev1.Pod) []string {
	names := make([]string, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	return names
}
