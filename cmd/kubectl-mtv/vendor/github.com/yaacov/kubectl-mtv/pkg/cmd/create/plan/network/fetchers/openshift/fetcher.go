package openshift

import "context"

import (
	"fmt"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// OpenShiftNetworkFetcher implements network fetching for OpenShift providers
type OpenShiftNetworkFetcher struct{}

// NewOpenShiftNetworkFetcher creates a new OpenShift network fetcher
func NewOpenShiftNetworkFetcher() *OpenShiftNetworkFetcher {
	return &OpenShiftNetworkFetcher{}
}

// FetchSourceNetworks extracts network references from OpenShift VMs
func (f *OpenShiftNetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error) {
	klog.V(4).Infof("OpenShift fetcher - extracting source networks for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch networks inventory (NADs in OpenShift) first to create name-to-ID mapping
	networksInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "networkattachmentdefinitions?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	// Create name-to-ID and ID-to-network mappings for NADs
	networkNameToID := make(map[string]string)
	networkIDToNetwork := make(map[string]map[string]interface{})
	for _, item := range networksArray {
		if network, ok := item.(map[string]interface{}); ok {
			// Use the actual UUID as the ID
			if networkID, ok := network["id"].(string); ok {
				if networkName, ok := network["name"].(string); ok {
					// Map network name to the actual UUID
					networkNameToID[networkName] = networkID
					networkIDToNetwork[networkID] = network
				}
			}
		}
	}

	klog.V(4).Infof("Available NAD mappings:")
	for id, networkItem := range networkIDToNetwork {
		if name, ok := networkItem["name"].(string); ok {
			klog.V(4).Infof("  %s -> %s", id, name)
		}
	}

	// Fetch VMs inventory to get network references from VMs
	vmsInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "vms?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %v", err)
	}

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	// Extract network IDs used by the plan VMs
	networkIDSet := make(map[string]bool)
	planVMSet := make(map[string]bool)
	for _, vmName := range planVMNames {
		planVMSet[vmName] = true
	}

	for _, item := range vmsArray {
		vm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, ok := vm["name"].(string)
		if !ok || !planVMSet[vmName] {
			continue
		}

		klog.V(4).Infof("Processing VM: %s", vmName)

		// Extract network names from VM spec.template.spec.networks (OpenShift VMs)
		networks, err := query.GetValueByPathString(vm, "object.spec.template.spec.networks")
		if err == nil && networks != nil {
			if networksArray, ok := networks.([]interface{}); ok {
				klog.V(4).Infof("VM %s has %d networks", vmName, len(networksArray))
				for _, networkItem := range networksArray {
					if networkMap, ok := networkItem.(map[string]interface{}); ok {
						// For OpenShift VMs, networks are typically referenced by NAD name
						if networkName, ok := networkMap["name"].(string); ok {
							klog.V(4).Infof("Found network name: %s", networkName)
							if networkID, exists := networkNameToID[networkName]; exists {
								klog.V(4).Infof("Found exact NAD match: %s -> %s", networkName, networkID)
								networkIDSet[networkID] = true
							} else {
								// Try fuzzy matching if exact match fails
								for availableName, availableID := range networkNameToID {
									if strings.Contains(networkName, availableName) || strings.Contains(availableName, networkName) {
										klog.V(4).Infof("Found fuzzy NAD match: %s -> %s (via %s)", networkName, availableID, availableName)
										networkIDSet[availableID] = true
										break
									}
								}
							}
						}
					}
				}
			}
		} else {
			klog.V(4).Infof("VM %s has no networks or failed to extract: err=%v", vmName, err)
		}
	}

	klog.V(4).Infof("Final networkIDSet: %v", networkIDSet)

	// If no networks found from VMs, return empty list
	if len(networkIDSet) == 0 {
		klog.V(4).Infof("No networks found from VMs")
		return []ref.Ref{}, nil
	}

	// Build source networks list using the collected IDs
	var sourceNetworks []ref.Ref
	for networkID := range networkIDSet {
		if networkItem, exists := networkIDToNetwork[networkID]; exists {
			sourceNetwork := ref.Ref{
				ID: networkID,
			}
			if name, ok := networkItem["name"].(string); ok {
				sourceNetwork.Name = name
			}
			if namespace, ok := networkItem["namespace"].(string); ok {
				sourceNetwork.Namespace = namespace
			}
			sourceNetworks = append(sourceNetworks, sourceNetwork)
		}
	}

	klog.V(4).Infof("OpenShift fetcher - found %d source networks", len(sourceNetworks))
	return sourceNetworks, nil
}

// FetchTargetNetworks extracts available destination networks from target provider
func (f *OpenShiftNetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationNetwork, error) {
	klog.V(4).Infof("OpenShift fetcher - extracting target networks for provider: %s", providerName)

	// Get the target provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get target provider: %v", err)
	}

	// Get provider type for target provider to determine network format
	providerClient := inventory.NewProviderClient(configFlags, provider, inventoryURL)
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	klog.V(4).Infof("Target provider name: %s", providerName)
	klog.V(4).Infof("Target provider type detected: %s", providerType)

	// For OpenShift targets, always fetch NADs
	klog.V(4).Infof("Fetching NetworkAttachmentDefinitions for OpenShift target")
	networksInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "networkattachmentdefinitions?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for target networks inventory")
	}

	// Build target networks list
	var targetNetworks []forkliftv1beta1.DestinationNetwork
	for _, item := range networksArray {
		networkItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		networkName := ""
		if name, ok := networkItem["name"].(string); ok {
			networkName = name
		}

		networkNamespace := ""
		if ns, ok := networkItem["namespace"].(string); ok {
			networkNamespace = ns
		}

		// For OpenShift targets, create Multus network reference
		// Always set namespace, use plan namespace if empty
		klog.V(4).Infof("Creating multus network reference for: %s/%s", networkNamespace, networkName)
		destNetwork := forkliftv1beta1.DestinationNetwork{
			Type: "multus",
			Name: networkName,
		}
		// Always set namespace, use plan namespace if empty
		if networkNamespace != "" {
			destNetwork.Namespace = networkNamespace
		} else {
			destNetwork.Namespace = namespace
		}
		targetNetworks = append(targetNetworks, destNetwork)
	}

	klog.V(4).Infof("Available target networks count: %d", len(targetNetworks))
	return targetNetworks, nil
}
