package hyperv

import (
	"context"
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// HyperVStorageFetcher implements storage fetching for HyperV providers
type HyperVStorageFetcher struct{}

// NewHyperVStorageFetcher creates a new HyperV storage fetcher
func NewHyperVStorageFetcher() *HyperVStorageFetcher {
	return &HyperVStorageFetcher{}
}

// FetchSourceStorages extracts storage references from HyperV provider.
// HyperV uses a single SMB share for all VM storage, so the storages endpoint
// typically returns one entry. We return all storages from the API.
func (f *HyperVStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, _ []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("HyperV storage fetcher - extracting source storages for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %w", err)
	}

	// Fetch storage inventory - HyperV typically has a single SMB share
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %w", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	klog.V(4).Infof("HyperV storage fetcher - found %d storage entries", len(storageArray))

	// Build source storages list from all available storages
	var sourceStorages []ref.Ref
	for _, item := range storageArray {
		if storage, ok := item.(map[string]interface{}); ok {
			storageRef := ref.Ref{}

			if storageID, ok := storage["id"].(string); ok {
				storageRef.ID = storageID
			}
			if name, ok := storage["name"].(string); ok {
				storageRef.Name = name
			}

			if storageRef.ID != "" {
				sourceStorages = append(sourceStorages, storageRef)
				klog.V(4).Infof("  Storage: %s (ID: %s)", storageRef.Name, storageRef.ID)
			}
		}
	}

	klog.V(4).Infof("HyperV storage fetcher - returning %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages is not supported for HyperV as target - only OpenShift is supported as target
func (f *HyperVStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("HyperV provider does not support target storage fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("HyperV provider does not support target storage fetching - only OpenShift is supported as migration target")
}
