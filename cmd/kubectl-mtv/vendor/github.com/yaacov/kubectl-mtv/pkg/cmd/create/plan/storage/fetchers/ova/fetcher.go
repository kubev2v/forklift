package ova

import (
	"context"
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/query"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// OVAStorageFetcher implements storage fetching for OVA providers
type OVAStorageFetcher struct{}

// NewOVAStorageFetcher creates a new OVA storage fetcher
func NewOVAStorageFetcher() *OVAStorageFetcher {
	return &OVAStorageFetcher{}
}

// FetchSourceStorages extracts storage references from OVA VMs
func (f *OVAStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("OVA storage fetcher - extracting source storages for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch storage inventory first to create ID-to-storage mapping
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Create ID-to-storage mapping
	storageIDToStorage := make(map[string]map[string]interface{})
	for _, item := range storageArray {
		if storage, ok := item.(map[string]interface{}); ok {
			if storageID, ok := storage["id"].(string); ok {
				storageIDToStorage[storageID] = storage
			}
		}
	}

	klog.V(4).Infof("Available storage mappings:")
	for id, storageItem := range storageIDToStorage {
		if name, ok := storageItem["name"].(string); ok {
			klog.V(4).Infof("  %s -> %s", id, name)
		}
	}

	// Fetch VMs inventory to get storage references from VMs
	vmsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "vms?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %v", err)
	}

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	// Extract storage IDs used by the plan VMs
	storageIDSet := make(map[string]bool)
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

		// Extract storage IDs from VM disks (OVA VMs have direct disks array with ID field)
		disks, err := query.GetValueByPathString(vm, "disks")
		if err == nil && disks != nil {
			if disksArray, ok := disks.([]interface{}); ok {
				klog.V(4).Infof("VM %s has %d disks", vmName, len(disksArray))
				for _, diskItem := range disksArray {
					if diskMap, ok := diskItem.(map[string]interface{}); ok {
						// OVA uses capital "ID" field
						if storageID, ok := diskMap["ID"].(string); ok {
							klog.V(4).Infof("Found storage ID: %s", storageID)
							storageIDSet[storageID] = true
						}
					}
				}
			}
		} else {
			klog.V(4).Infof("VM %s has no disks or failed to extract: err=%v", vmName, err)
		}
	}

	klog.V(4).Infof("Final storageIDSet: %v", storageIDSet)

	// If no storages found from VMs, return empty list
	if len(storageIDSet) == 0 {
		klog.V(4).Infof("No storages found from VMs")
		return []ref.Ref{}, nil
	}

	// Build source storages list using the collected IDs
	var sourceStorages []ref.Ref
	for storageID := range storageIDSet {
		if storageItem, exists := storageIDToStorage[storageID]; exists {
			sourceStorage := ref.Ref{
				ID: storageID,
			}
			if name, ok := storageItem["name"].(string); ok {
				sourceStorage.Name = name
			}
			sourceStorages = append(sourceStorages, sourceStorage)
		}
	}

	klog.V(4).Infof("OVA storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages is not supported for OVA as target - only OpenShift is supported as target
func (f *OVAStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("OVA provider does not support target storage fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("OVA provider does not support target storage fetching - only OpenShift is supported as migration target")
}
