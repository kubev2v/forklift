package defaultprovider

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// GetDefaultOpenShiftProvider returns the most suitable OpenShift provider in the specified namespace.
// It prioritizes providers with empty url (local cluster), then falls back to the first OpenShift provider.
// Returns an error if no OpenShift provider is found.
func GetDefaultOpenShiftProvider(configFlags *genericclioptions.ConfigFlags, namespace string) (string, error) {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return "", fmt.Errorf("failed to get client: %v", err)
	}

	providers, err := c.Resource(client.ProvidersGVR).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list providers: %v", err)
	}

	var firstOpenShiftProvider string
	var emptyUrlOpenShiftProvider string

	for _, provider := range providers.Items {
		// Get provider type from spec
		providerType, found, err := unstructured.NestedString(provider.Object, "spec", "type")
		if err != nil || !found {
			continue
		}

		// Check if provider type is OpenShift
		if providerType == "openshift" {
			// If this is the first OpenShift provider we've found, record it
			if firstOpenShiftProvider == "" {
				firstOpenShiftProvider = provider.GetName()
			}

			// Check if provider has empty URL
			url, found, err := unstructured.NestedString(provider.Object, "spec", "url")
			if err == nil && (!found || url == "") {
				emptyUrlOpenShiftProvider = provider.GetName()
				break // Found the preferred provider, no need to continue
			}
		}
	}

	// Prefer the empty URL provider, otherwise use the first OpenShift provider
	if emptyUrlOpenShiftProvider != "" {
		return emptyUrlOpenShiftProvider, nil
	}

	if firstOpenShiftProvider != "" {
		return firstOpenShiftProvider, nil
	}

	return "", fmt.Errorf("no OpenShift provider found in namespace '%s'", namespace)
}
