package mapping

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// resolveOpenShiftNetworkNameToID resolves network name for OpenShift provider
func resolveOpenShiftNetworkNameToID(configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, networkName string) ([]ref.Ref, error) {
	// If networkName is empty, return an empty ref
	if networkName == "" {
		return nil, fmt.Errorf("network name cannot be empty")
	}

	// If networkName is default, return special pod reference
	if networkName == "default" {
		return []ref.Ref{{
			Type: "pod",
		}}, nil
	}

	// Parse namespace/name format
	var targetNamespace, targetName string
	if strings.Contains(networkName, "/") {
		parts := strings.SplitN(networkName, "/", 2)
		targetNamespace = strings.TrimSpace(parts[0])
		targetName = strings.TrimSpace(parts[1])
	} else {
		// If no namespace specified, assume "default"
		targetNamespace = "default"
		targetName = strings.TrimSpace(networkName)
	}

	// Fetch NetworkAttachmentDefinitions from OpenShift
	networksInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "networkattachmentdefinitions?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	// Search for all networks matching the name and namespace
	var matchingRefs []ref.Ref
	for _, item := range networksArray {
		network, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// For OpenShift NetworkAttachmentDefinitions
		if obj, exists := network["object"]; exists {
			if objMap, ok := obj.(map[string]interface{}); ok {
				if metadata, exists := objMap["metadata"]; exists {
					if metadataMap, ok := metadata.(map[string]interface{}); ok {
						name, _ := metadataMap["name"].(string)
						ns, _ := metadataMap["namespace"].(string)
						id, _ := metadataMap["uid"].(string)

						// Match both name and namespace
						if name == targetName && ns == targetNamespace {
							matchingRefs = append(matchingRefs, ref.Ref{
								ID:        id,
								Name:      name,
								Namespace: ns,
								Type:      "multus",
							})
						}
					}
				}
			}
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("network '%s' in namespace '%s' not found in OpenShift provider inventory", targetName, targetNamespace)
	}

	return matchingRefs, nil
}

// resolveVirtualizationNetworkNameToID resolves network name for virtualization providers (VMware, oVirt, OpenStack)
func resolveVirtualizationNetworkNameToID(configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, networkName string) ([]ref.Ref, error) {
	// Fetch networks from virtualization providers
	networksInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "networks?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	// Search for all networks matching the name
	var matchingRefs []ref.Ref
	for _, item := range networksArray {
		network, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// For virtualization providers (VMware, oVirt, etc.)
		name, _ := network["name"].(string)
		id, _ := network["id"].(string)

		if name == networkName {
			matchingRefs = append(matchingRefs, ref.Ref{
				ID: id,
			})
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("network '%s' not found in virtualization provider inventory", networkName)
	}

	return matchingRefs, nil
}

// resolveNetworkNameToID resolves a network name to its ref.Ref by querying the provider inventory
func resolveNetworkNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, networkName string) ([]ref.Ref, error) {
	// Get source provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider '%s': %v", providerName, err)
	}

	// Check provider type to determine which helper to use
	providerType, _, err := unstructured.NestedString(provider.Object, "spec", "type")
	if err != nil {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	switch providerType {
	case "openshift":
		return resolveOpenShiftNetworkNameToID(configFlags, inventoryURL, provider, networkName)
	case "vsphere", "ovirt", "openstack", "ova":
		return resolveVirtualizationNetworkNameToID(configFlags, inventoryURL, provider, networkName)
	default:
		return resolveVirtualizationNetworkNameToID(configFlags, inventoryURL, provider, networkName)
	}
}
