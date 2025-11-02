package openstack

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

// OpenStackNetworkFetcher implements network fetching for OpenStack providers
type OpenStackNetworkFetcher struct{}

// NewOpenStackNetworkFetcher creates a new OpenStack network fetcher
func NewOpenStackNetworkFetcher() *OpenStackNetworkFetcher {
	return &OpenStackNetworkFetcher{}
}

// FetchSourceNetworks extracts network references from OpenStack VMs
func (f *OpenStackNetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error) {
	klog.V(4).Infof("OpenStack fetcher - extracting source networks for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch networks inventory first to create name-to-ID mapping
	networksInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "networks?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	// Create name-to-ID and ID-to-network mappings
	networkNameToID := make(map[string]string)
	networkIDToNetwork := make(map[string]map[string]interface{})
	for _, item := range networksArray {
		if network, ok := item.(map[string]interface{}); ok {
			if networkID, ok := network["id"].(string); ok {
				networkIDToNetwork[networkID] = network
				if networkName, ok := network["name"].(string); ok {
					networkNameToID[networkName] = networkID
				}
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

	// Extract network IDs used by the plan VMs (convert names to IDs immediately)
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

		addresses, err := query.GetValueByPathString(vm, "addresses")
		if err != nil || addresses == nil {
			klog.V(4).Infof("VM %s has no addresses or failed to extract: err=%v", vmName, err)
			continue
		}

		if addressesMap, ok := addresses.(map[string]interface{}); ok {
			for networkName := range addressesMap {
				klog.V(4).Infof("Found network name: %s", networkName)

				if networkID, exists := networkNameToID[networkName]; exists {
					klog.V(4).Infof("Found exact network match: %s -> %s", networkName, networkID)
					networkIDSet[networkID] = true
				} else {
					for availableName, availableID := range networkNameToID {
						if strings.Contains(networkName, availableName) || strings.Contains(availableName, networkName) {
							klog.V(4).Infof("Found fuzzy network match: %s -> %s (via %s)", networkName, availableID, availableName)
							networkIDSet[availableID] = true
							break
						}
					}

				}
			}
		}
	}

	if len(networkIDSet) == 0 {
		klog.V(4).Infof("No networks found from VMs - VMs have incomplete data (missing addresses field)")
		return []ref.Ref{}, nil
	}

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

	klog.V(4).Infof("OpenStack fetcher - found %d source networks", len(sourceNetworks))
	return sourceNetworks, nil
}

// FetchTargetNetworks is not supported for OpenStack as target - only OpenShift is supported as target
func (f *OpenStackNetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationNetwork, error) {
	klog.V(4).Infof("OpenStack provider does not support target network fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("OpenStack provider does not support target network fetching - only OpenShift is supported as migration target")
}
