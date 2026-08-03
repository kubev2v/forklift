package azure

import (
	"context"
	"fmt"
	"sort"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// AzureStorageFetcher implements storage fetching for Azure providers
type AzureStorageFetcher struct{}

// NewAzureStorageFetcher creates a new Azure storage fetcher
func NewAzureStorageFetcher() fetchers.StorageFetcher {
	return &AzureStorageFetcher{}
}

// FetchSourceStorages fetches disk type SKUs from Azure provider.
// When planVMNames is non-empty, only SKUs actually used by those VMs are returned.
func (f *AzureStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: Azure - Fetching source storage types from provider: %s", providerName)

	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure provider: %v", err)
	}

	// Fetch all available storage SKUs
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Azure storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Build name->ref map from storage inventory
	storageRefByName := make(map[string]ref.Ref)
	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := storage["name"].(string)
		id, _ := storage["id"].(string)
		if name != "" {
			storageRefByName[name] = ref.Ref{ID: id, Name: name}
		}
	}

	// If planVMNames provided, filter to only SKUs used by those VMs
	if len(planVMNames) > 0 {
		return f.fetchSKUsFromVMs(ctx, configFlags, inventoryURL, provider, planVMNames, storageRefByName, insecureSkipTLS)
	}

	// Fallback: return all unique SKUs
	return f.allUniqueSKUs(storageRefByName), nil
}

// fetchSKUsFromVMs fetches VMs inventory, filters by planVMNames, and extracts
// unique disk SKUs used by those VMs.
func (f *AzureStorageFetcher) fetchSKUsFromVMs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, planVMNames []string, storageRefByName map[string]ref.Ref, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: Azure - Filtering source storages by %d plan VMs", len(planVMNames))

	vmsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "vms?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Azure VMs inventory: %v", err)
	}

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	planVMSet := make(map[string]bool, len(planVMNames))
	for _, name := range planVMNames {
		planVMSet[name] = true
	}

	skuSet := make(map[string]struct{})
	for _, item := range vmsArray {
		vm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vmName, _ := vm["name"].(string)
		if !planVMSet[vmName] {
			continue
		}

		disks, _ := vm["disks"].([]interface{})
		for _, d := range disks {
			disk, ok := d.(map[string]interface{})
			if !ok {
				continue
			}
			if sku, ok := disk["sku"].(string); ok && sku != "" {
				skuSet[sku] = struct{}{}
				klog.V(4).Infof("DEBUG: Azure - VM %s uses disk SKU: %s", vmName, sku)
			}
		}
	}

	if len(skuSet) == 0 {
		klog.V(4).Infof("DEBUG: Azure - No disk SKUs found from plan VMs, falling back to all SKUs")
		return f.allUniqueSKUs(storageRefByName), nil
	}

	var sourceStorages []ref.Ref
	for sku := range skuSet {
		if r, exists := storageRefByName[sku]; exists {
			sourceStorages = append(sourceStorages, r)
		} else {
			sourceStorages = append(sourceStorages, ref.Ref{Name: sku})
		}
	}

	sort.Slice(sourceStorages, func(i, j int) bool {
		return sourceStorages[i].Name < sourceStorages[j].Name
	})

	klog.V(4).Infof("DEBUG: Azure - Found %d source storage types from plan VMs", len(sourceStorages))
	return sourceStorages, nil
}

// allUniqueSKUs returns all unique SKUs from the storage inventory.
func (f *AzureStorageFetcher) allUniqueSKUs(storageRefByName map[string]ref.Ref) []ref.Ref {
	var sourceStorages []ref.Ref
	for _, r := range storageRefByName {
		sourceStorages = append(sourceStorages, r)
	}

	sort.Slice(sourceStorages, func(i, j int) bool {
		return sourceStorages[i].Name < sourceStorages[j].Name
	})

	klog.V(4).Infof("DEBUG: Azure - Found %d source storage types (all)", len(sourceStorages))
	return sourceStorages
}

// FetchTargetStorages fetches target storage from Azure provider (not typically used as Azure is usually source)
func (f *AzureStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("DEBUG: Azure - Fetching target storage (Azure is typically not a migration target)")
	return []forkliftv1beta1.DestinationStorage{}, nil
}
