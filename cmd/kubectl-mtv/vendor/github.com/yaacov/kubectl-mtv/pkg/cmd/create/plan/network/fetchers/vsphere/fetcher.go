package vsphere

import "context"

import (
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// VSphereNetworkFetcher implements network fetching for VSphere providers
type VSphereNetworkFetcher struct{}

// NewVSphereNetworkFetcher creates a new VSphere network fetcher
func NewVSphereNetworkFetcher() *VSphereNetworkFetcher {
	return &VSphereNetworkFetcher{}
}

// FetchSourceNetworks extracts network references from VSphere VMs
func (f *VSphereNetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error) {
	klog.V(4).Infof("VSphere fetcher - extracting source networks for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch networks inventory first to create ID-to-network mapping
	networksInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "networks?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	// Create ID-to-network mapping
	networkIDToNetwork := make(map[string]map[string]interface{})
	for _, item := range networksArray {
		if network, ok := item.(map[string]interface{}); ok {
			if networkID, ok := network["id"].(string); ok {
				networkIDToNetwork[networkID] = network
			}
		}
	}

	klog.V(4).Infof("Available network mappings:")
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

		// Extract network IDs from VM networks (VSphere VMs have direct networks array)
		networks, err := query.GetValueByPathString(vm, "networks")
		if err == nil && networks != nil {
			if networksArray, ok := networks.([]interface{}); ok {
				klog.V(4).Infof("VM %s has %d networks", vmName, len(networksArray))
				for _, networkItem := range networksArray {
					if networkMap, ok := networkItem.(map[string]interface{}); ok {
						if networkID, ok := networkMap["id"].(string); ok {
							klog.V(4).Infof("Found network ID: %s", networkID)
							networkIDSet[networkID] = true
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
			sourceNetworks = append(sourceNetworks, sourceNetwork)
		}
	}

	klog.V(4).Infof("VSphere fetcher - found %d source networks", len(sourceNetworks))
	return sourceNetworks, nil
}

// FetchTargetNetworks is not supported for VSphere as target - only OpenShift is supported as target
func (f *VSphereNetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationNetwork, error) {
	klog.V(4).Infof("VSphere provider does not support target network fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("VSphere provider does not support target network fetching - only OpenShift is supported as migration target")
}
