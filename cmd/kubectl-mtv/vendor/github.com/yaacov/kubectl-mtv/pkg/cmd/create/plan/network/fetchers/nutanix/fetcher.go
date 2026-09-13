package nutanix

import (
	"context"
	"fmt"
	"sort"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// NetworkFetcher implements network fetching for Nutanix providers.
type NetworkFetcher struct{}

// NewNetworkFetcher creates a Nutanix network fetcher.
func NewNetworkFetcher() *NetworkFetcher {
	return &NetworkFetcher{}
}

// FetchSourceNetworks extracts network references from Nutanix VMs via NIC subnetUuid.
func (f *NetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %w", err)
	}

	networksInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "networks?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch networks inventory: %w", err)
	}
	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	networkIDToNetwork := make(map[string]map[string]interface{})
	for _, item := range networksArray {
		if network, ok := item.(map[string]interface{}); ok {
			if networkID, ok := network["id"].(string); ok {
				networkIDToNetwork[networkID] = network
			}
		}
	}

	vmsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "vms?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %w", err)
	}
	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	planVMSet := make(map[string]bool, len(planVMNames))
	for _, vmName := range planVMNames {
		planVMSet[vmName] = true
	}

	networkIDSet := make(map[string]bool)
	for _, item := range vmsArray {
		vm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vmName, ok := vm["name"].(string)
		if !ok || !planVMSet[vmName] {
			continue
		}
		nics, err := query.GetValueByPathString(vm, "nics")
		if err != nil || nics == nil {
			continue
		}
		nicsArray, ok := nics.([]interface{})
		if !ok {
			continue
		}
		for _, nicItem := range nicsArray {
			nicMap, ok := nicItem.(map[string]interface{})
			if !ok {
				continue
			}
			if subnetUUID, ok := nicMap["subnetUuid"].(string); ok && subnetUUID != "" {
				networkIDSet[subnetUUID] = true
			}
		}
	}

	var sourceNetworks []ref.Ref
	for networkID := range networkIDSet {
		sourceNetwork := ref.Ref{ID: networkID}
		if networkItem, exists := networkIDToNetwork[networkID]; exists {
			if name, ok := networkItem["name"].(string); ok {
				sourceNetwork.Name = name
			}
		}
		sourceNetworks = append(sourceNetworks, sourceNetwork)
	}

	sort.Slice(sourceNetworks, func(i, j int) bool {
		return sourceNetworks[i].ID < sourceNetworks[j].ID
	})

	klog.V(4).Infof("Nutanix network fetcher - found %d source networks", len(sourceNetworks))
	return sourceNetworks, nil
}

// FetchTargetNetworks is not supported; Nutanix is a source provider only.
func (f *NetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationNetwork, error) {
	return nil, fmt.Errorf("nutanix provider does not support target network fetching - only OpenShift is supported as migration target")
}
