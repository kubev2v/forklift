package mapping

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/output"
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

// validateNetworkPairsTargets validates network mapping pairs for duplicate pod network targets.
// This is a pre-validation that can be done before resolution.
// Network mapping constraints for targets:
// - Pod networking ("default") can only be mapped once (only one source can use pod networking)
// - NAD targets can be used multiple times (multiple sources can map to the same NAD)
// - "ignored" targets can be used multiple times
func validateNetworkPairsTargets(pairStr string) error {
	if pairStr == "" {
		return nil
	}

	// Track if pod networking (default) has already been used
	podNetworkUsed := false

	pairList := strings.Split(pairStr, ",")

	for _, pair := range pairList {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue // Skip malformed pairs, let parseNetworkPairs handle the error
		}

		targetPart := strings.TrimSpace(parts[1])

		// Check for duplicate pod networking target
		if targetPart == "default" {
			if podNetworkUsed {
				return fmt.Errorf("invalid network mapping: Pod network ('default') can only be mapped once. Found duplicate mapping to 'default' in '%s'. Use 'source:ignored' for additional sources that don't need network access", pair)
			}
			podNetworkUsed = true
		}
	}

	return nil
}

// parseNetworkPairs parses network pairs in format "source1:namespace/target1,source2:namespace/target2"
// If namespace is omitted, the provided defaultNamespace will be used
// Special target values: "default" for pod networking, "ignored" to ignore the source network
func parseNetworkPairs(ctx context.Context, pairStr, defaultNamespace string, configFlags *genericclioptions.ConfigFlags, sourceProvider, inventoryURL string) ([]forkliftv1beta1.NetworkPair, error) {
	return parseNetworkPairsWithInsecure(ctx, pairStr, defaultNamespace, configFlags, sourceProvider, inventoryURL, false)
}

// parseNetworkPairsWithInsecure parses network pairs with optional insecure TLS skip verification
func parseNetworkPairsWithInsecure(ctx context.Context, pairStr, defaultNamespace string, configFlags *genericclioptions.ConfigFlags, sourceProvider, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.NetworkPair, error) {
	if pairStr == "" {
		return nil, nil
	}

	// Validate target constraints before processing (pod network can only be mapped once)
	if err := validateNetworkPairsTargets(pairStr); err != nil {
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

		sourcePart := strings.TrimSpace(parts[0])
		targetPart := strings.TrimSpace(parts[1])

		// Parse optional VLAN qualifier from source (syntax: sourceName@vlan)
		sourceName, vlan, err := parseSourceVLAN(sourcePart)
		if err != nil {
			return nil, fmt.Errorf("invalid source in network pair '%s': %v", pairStr, err)
		}

		// Resolve source network name to ID
		sourceNetworkRefs, err := resolveNetworkNameToIDWithInsecure(ctx, configFlags, sourceProvider, defaultNamespace, inventoryURL, sourceName, insecureSkipTLS)
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
				Source:      forkliftv1beta1.NetworkSourceRef{Ref: sourceNetworkRef, Vlan: vlan},
				Destination: destinationNetwork,
			}

			pairs = append(pairs, pair)
		}
	}

	return pairs, nil
}

// createNetworkMappingWithInsecure creates a new network mapping with optional insecure TLS skip verification
func createNetworkMappingWithInsecure(configFlags *genericclioptions.ConfigFlags, name, namespace, sourceProvider, targetProvider, networkPairs, inventoryURL string, insecureSkipTLS bool, dryRun bool, outputFormat string) error {
	// Parse provider references to extract names and namespaces
	sourceProviderName, sourceProviderNamespace := parseProviderReference(sourceProvider, namespace)
	targetProviderName, targetProviderNamespace := parseProviderReference(targetProvider, namespace)

	// Parse network pairs if provided
	var mappingPairs []forkliftv1beta1.NetworkPair
	var err error
	if networkPairs != "" {
		mappingPairs, err = parseNetworkPairsWithInsecure(context.TODO(), networkPairs, namespace, configFlags, sourceProvider, inventoryURL, insecureSkipTLS)
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
	networkMap.Kind = "NetworkMap"
	networkMap.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	if dryRun {
		return output.OutputResource(networkMap, outputFormat)
	}

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
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

// parseSourceVLAN parses optional VLAN qualifier from a source network specifier.
// Format: "sourceName" or "sourceName@vlan" where vlan is 1-4094.
// Returns the source name, VLAN string (empty if not specified), and any error.
func parseSourceVLAN(sourcePart string) (name, vlan string, err error) {
	if idx := strings.LastIndex(sourcePart, "@"); idx >= 0 {
		name = sourcePart[:idx]
		vlan = sourcePart[idx+1:]
		if name == "" {
			return "", "", fmt.Errorf("source network name cannot be empty before @vlan")
		}
		if vlan == "" {
			return "", "", fmt.Errorf("VLAN value cannot be empty after @")
		}
		vlanNum, err := strconv.Atoi(vlan)
		if err != nil {
			return "", "", fmt.Errorf("VLAN must be a number (1-4094), got '%s'", vlan)
		}
		if vlanNum < 1 || vlanNum > 4094 {
			return "", "", fmt.Errorf("VLAN must be in range 1-4094, got %d", vlanNum)
		}
		return name, vlan, nil
	}
	return sourcePart, "", nil
}
