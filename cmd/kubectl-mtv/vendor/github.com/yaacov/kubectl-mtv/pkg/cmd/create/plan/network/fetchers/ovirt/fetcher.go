package ovirt

import "context"

import (
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// OvirtNetworkFetcher implements network fetching for oVirt providers
type OvirtNetworkFetcher struct{}

// NewOvirtNetworkFetcher creates a new oVirt network fetcher
func NewOvirtNetworkFetcher() *OvirtNetworkFetcher {
	return &OvirtNetworkFetcher{}
}

// FetchSourceNetworks extracts network references from oVirt VMs
func (f *OvirtNetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error) {
	klog.V(4).Infof("oVirt fetcher - extracting source networks for provider: %s", providerName)

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

	// Fetch NIC profiles to map profile IDs to network IDs
	nicProfilesInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "nicprofiles?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NIC profiles inventory: %v", err)
	}

	nicProfilesArray, ok := nicProfilesInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for NIC profiles inventory")
	}

	// Create profile ID to network ID mapping
	profileIDToNetworkID := make(map[string]string)
	for _, item := range nicProfilesArray {
		if profile, ok := item.(map[string]interface{}); ok {
			if profileID, ok := profile["id"].(string); ok {
				if networkID, ok := profile["network"].(string); ok {
					profileIDToNetworkID[profileID] = networkID
				}
			}
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

	for _, vmItem := range vmsArray {
		if vm, ok := vmItem.(map[string]interface{}); ok {
			if vmName, ok := vm["name"].(string); ok && planVMSet[vmName] {
				klog.V(4).Infof("Processing VM: %s", vmName)

				// Extract profiles from VM nics
				if nics, ok := vm["nics"].([]interface{}); ok {
					for _, nicItem := range nics {
						if nic, ok := nicItem.(map[string]interface{}); ok {
							// Get profile ID from nic
							if profileID, ok := nic["profile"].(string); ok {
								klog.V(4).Infof("Found profile ID: %s", profileID)
								// Map profile ID to network ID
								if networkID, exists := profileIDToNetworkID[profileID]; exists {
									klog.V(4).Infof("Mapped profile %s to network %s", profileID, networkID)
									networkIDSet[networkID] = true
								}
							}
						}
					}
				}
			}
		}
	}

	klog.V(4).Infof("oVirt fetcher - found %d source networks", len(networkIDSet))

	// Create source network references for the networks used by VMs
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

	return sourceNetworks, nil
}

// FetchTargetNetworks is not supported for oVirt as target - only OpenShift is supported as target
func (f *OvirtNetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationNetwork, error) {
	klog.V(4).Infof("oVirt provider does not support target network fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("oVirt provider does not support target network fetching - only OpenShift is supported as migration target")
}
