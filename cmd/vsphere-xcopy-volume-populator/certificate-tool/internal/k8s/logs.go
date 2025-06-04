package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GetPodLogs fetches the logs for a given pod and returns the last 'tailLines' lines.
func GetPodLogs(ctx context.Context, clientset kubernetes.Interface, namespace, podName string, tailLines int64) (string, error) {
	podLogOptions := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("error opening log stream for pod %s/%s: %w", namespace, podName, err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error copying logs for pod %s/%s: %w", namespace, podName, err)
	}

	return buf.String(), nil
}
