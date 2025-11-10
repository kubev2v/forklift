package inventory

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// GetProviderByName fetches a provider by name from the specified namespace
func GetProviderByName(ctx context.Context, configFlags *genericclioptions.ConfigFlags, name, namespace string) (*unstructured.Unstructured, error) {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	provider, err := c.Resource(client.ProvidersGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get provider '%s': %v", name, err)
	}

	return provider, nil
}

// humanizeBytes converts bytes to a human-readable string with appropriate unit suffix
func humanizeBytes(bytes float64) string {
	const unit = 1024.0
	if bytes < unit {
		return fmt.Sprintf("%.1f B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := "KMGTPE"[exp : exp+1]
	return fmt.Sprintf("%.1f %sB", bytes/div, suffix)
}
