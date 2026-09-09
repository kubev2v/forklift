package azure

import (
	"context"
	"fmt"
	"sort"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// AzureNetworkFetcher implements network fetching for Azure providers
type AzureNetworkFetcher struct{}

// NewAzureNetworkFetcher creates a new Azure network fetcher
func NewAzureNetworkFetcher() fetchers.NetworkFetcher {
	return &AzureNetworkFetcher{}
}

// FetchSourceNetworks fetches networks (subnets) from Azure provider
func (f *AzureNetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, _ []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: Azure - Fetching source networks from provider: %s", providerName)

	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure provider: %v", err)
	}

	networksInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "networks?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Azure networks inventory: %v", err)
	}

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	var sourceNetworks []ref.Ref
	for _, item := range networksArray {
		network, ok := item.(map[string]interface{})
		if !ok {
			klog.V(4).Infof("DEBUG: Azure - Skipping network item with unexpected type: %T", item)
			continue
		}

		id, _ := network["id"].(string)
		name, _ := network["name"].(string)

		if id != "" {
			sourceNetworks = append(sourceNetworks, ref.Ref{
				ID:   id,
				Name: name,
			})
		}
	}

	sort.Slice(sourceNetworks, func(i, j int) bool {
		return sourceNetworks[i].Name < sourceNetworks[j].Name
	})

	klog.V(4).Infof("DEBUG: Azure - Found %d source networks (subnets)", len(sourceNetworks))
	return sourceNetworks, nil
}

// FetchTargetNetworks fetches target networks from Azure provider (not typically used as Azure is usually source)
func (f *AzureNetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationNetwork, error) {
	klog.V(4).Infof("DEBUG: Azure - Fetching target networks (Azure is typically not a migration target)")
	return []forkliftv1beta1.DestinationNetwork{}, nil
}
