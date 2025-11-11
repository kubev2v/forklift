package mapping

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	corev1 "k8s.io/api/core/v1"
)

// parseProviderReference parses a provider reference that might contain namespace/name pattern
// Returns the name and namespace separately. If no namespace is specified, returns the default namespace.
func parseProviderReference(providerRef, defaultNamespace string) (name, namespace string) {
	if strings.Contains(providerRef, "/") {
		parts := strings.SplitN(providerRef, "/", 2)
		namespace = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
	} else {
		name = strings.TrimSpace(providerRef)
		namespace = defaultNamespace
	}
	return name, namespace
}

// validateNetworkPairs validates network mapping pairs for duplicate targets.
// Network mapping constraints:
// - Pod networking ("default") can only be mapped once
// - Specific NADs (Network Attachment Definitions) can only be mapped once
// - "ignored" targets can be used multiple times (valid for unused networks)
func validateNetworkPairs(pairStr, defaultNamespace string) error {
	if pairStr == "" {
		return nil
	}

	// Track target networks to detect duplicates
	targetsSeen := make(map[string]bool)
	pairList := strings.Split(pairStr, ",")

	for _, pairStr := range pairList {
		pairStr = strings.TrimSpace(pairStr)
		if pairStr == "" {
			continue
		}

		parts := strings.SplitN(pairStr, ":", 2)
		if len(parts) != 2 {
			continue // Skip malformed pairs, let parseNetworkPairs handle the error
		}

		targetPart := strings.TrimSpace(parts[1])

		// Skip validation for ignored targets (they can be used multiple times)
		if strings.ToLower(targetPart) == "ignored" {
			continue
		}

		// Normalize target name for comparison
		var normalizedTarget string
		if strings.Contains(targetPart, "/") {
			// namespace/name format - use as-is but normalize to lowercase
			normalizedTarget = strings.ToLower(targetPart)
		} else if targetPart == "default" {
			// Pod networking
			normalizedTarget = "default"
		} else {
			// Name-only format, normalize with default namespace
			normalizedTarget = strings.ToLower(fmt.Sprintf("%s/%s", defaultNamespace, targetPart))
		}

		// Check for duplicate targets
		if targetsSeen[normalizedTarget] {
			// Provide specific error messages for common cases
			if targetPart == "default" {
				return fmt.Errorf("invalid network mapping: Pod network ('default') can only be mapped once. Found duplicate mapping to 'default' in '%s'. Use 'source:ignored' for additional sources that don't need network access", pairStr)
			}
			return fmt.Errorf("invalid network mapping: Target network '%s' can only be mapped once. Found duplicate mapping to '%s' in '%s'. Use 'source:ignored' for sources that don't need this network or map to different targets", targetPart, targetPart, pairStr)
		}

		targetsSeen[normalizedTarget] = true
	}

	return nil
}

// parseNetworkPairs parses network pairs in format "source1:namespace/target1,source2:namespace/target2"
// If namespace is omitted, the provided defaultNamespace will be used
// Special target values: "default" for pod networking, "ignored" to ignore the source network
func parseNetworkPairs(ctx context.Context, pairStr, defaultNamespace string, configFlags *genericclioptions.ConfigFlags, sourceProvider, inventoryURL string) ([]forkliftv1beta1.NetworkPair, error) {
	if pairStr == "" {
		return nil, nil
	}

	// Validate network pairs for duplicate targets before processing
	if err := validateNetworkPairs(pairStr, defaultNamespace); err != nil {
		return nil, err
	}

	var pairs []forkliftv1beta1.NetworkPair
	pairList := strings.Split(pairStr, ",")

	for _, pairStr := range pairList {
		pairStr = strings.TrimSpace(pairStr)
		if pairStr == "" {
			continue
		}

		parts := strings.SplitN(pairStr, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid network pair format '%s': expected 'source:target-namespace/target-network', 'source:target-network', 'source:default', or 'source:ignored'", pairStr)
		}

		sourceName := strings.TrimSpace(parts[0])
		targetPart := strings.TrimSpace(parts[1])

		// Resolve source network name to ID
		sourceNetworkRefs, err := resolveNetworkNameToID(ctx, configFlags, sourceProvider, defaultNamespace, inventoryURL, sourceName)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve source network '%s': %v", sourceName, err)
		}

		// Parse target part which can be just a name or namespace/name
		var targetNamespace, targetName, targetType string
		if strings.Contains(targetPart, "/") {
			targetParts := strings.SplitN(targetPart, "/", 2)
			targetNamespace = strings.TrimSpace(targetParts[0])
			targetName = strings.TrimSpace(targetParts[1])
			targetType = "multus"
		} else {
			// Special handling for 'default' and 'ignored' types
			switch targetPart {
			case "default":
				targetType = "pod"
			case "ignored":
				targetType = "ignored"
			default:
				// Use the target part as network name and default namespace
				targetName = targetPart
				targetNamespace = defaultNamespace
				targetType = "multus"
			}
		}

		destinationNetwork := forkliftv1beta1.DestinationNetwork{
			Type: targetType,
		}
		if targetName != "" {
			destinationNetwork.Name = targetName
		}
		// Always set namespace for multus networks, use plan namespace if empty
		if targetType == "multus" {
			if targetNamespace != "" {
				destinationNetwork.Namespace = targetNamespace
			} else {
				destinationNetwork.Namespace = defaultNamespace
			}
		}

		// Create a pair for each matching source network resource
		for _, sourceNetworkRef := range sourceNetworkRefs {
			pair := forkliftv1beta1.NetworkPair{
				Source:      sourceNetworkRef,
				Destination: destinationNetwork,
			}

			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}

// createNetworkMapping creates a new network mapping
func createNetworkMapping(configFlags *genericclioptions.ConfigFlags, name, namespace, sourceProvider, targetProvider, networkPairs, inventoryURL string) error {
	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Parse provider references to extract names and namespaces
	sourceProviderName, sourceProviderNamespace := parseProviderReference(sourceProvider, namespace)
	targetProviderName, targetProviderNamespace := parseProviderReference(targetProvider, namespace)

	// Parse network pairs if provided
	var mappingPairs []forkliftv1beta1.NetworkPair
	if networkPairs != "" {
		mappingPairs, err = parseNetworkPairs(context.TODO(), networkPairs, namespace, configFlags, sourceProvider, inventoryURL)
		if err != nil {
			return fmt.Errorf("failed to parse network pairs: %v", err)
		}
	}

	// Create a typed NetworkMap
	networkMap := &forkliftv1beta1.NetworkMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: forkliftv1beta1.NetworkMapSpec{
			Provider: provider.Pair{
				Source: corev1.ObjectReference{
					Name:      sourceProviderName,
					Namespace: sourceProviderNamespace,
				},
				Destination: corev1.ObjectReference{
					Name:      targetProviderName,
					Namespace: targetProviderNamespace,
				},
			},
			Map: mappingPairs,
		},
	}

	// Convert to unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(networkMap)
	if err != nil {
		return fmt.Errorf("failed to convert to unstructured: %v", err)
	}

	mapping := &unstructured.Unstructured{Object: unstructuredObj}
	mapping.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   client.Group,
		Version: client.Version,
		Kind:    "NetworkMap",
	})

	_, err = dynamicClient.Resource(client.NetworkMapGVR).Namespace(namespace).Create(context.TODO(), mapping, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network mapping: %v", err)
	}

	fmt.Printf("networkmap/%s created\n", name)
	return nil
}
