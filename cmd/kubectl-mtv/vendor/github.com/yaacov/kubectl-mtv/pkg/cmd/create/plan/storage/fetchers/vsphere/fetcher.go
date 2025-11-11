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

// VSphereStorageFetcher implements storage fetching for VSphere providers
type VSphereStorageFetcher struct{}

// NewVSphereStorageFetcher creates a new VSphere storage fetcher
func NewVSphereStorageFetcher() *VSphereStorageFetcher {
	return &VSphereStorageFetcher{}
}

// FetchSourceStorages extracts storage references from VSphere VMs
func (f *VSphereStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error) {
	klog.V(4).Infof("VSphere storage fetcher - extracting source storages for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch datastores inventory first to create ID-to-datastore mapping
	datastoresInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "datastores?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch datastores inventory: %v", err)
	}

	datastoresArray, ok := datastoresInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for datastores inventory")
	}

	// Create ID-to-datastore mapping
	datastoreIDToDatastore := make(map[string]map[string]interface{})
	for _, item := range datastoresArray {
		if datastore, ok := item.(map[string]interface{}); ok {
			if datastoreID, ok := datastore["id"].(string); ok {
				datastoreIDToDatastore[datastoreID] = datastore
			}
		}
	}

	klog.V(4).Infof("Available datastore mappings:")
	for id, datastoreItem := range datastoreIDToDatastore {
		if name, ok := datastoreItem["name"].(string); ok {
			klog.V(4).Infof("  %s -> %s", id, name)
		}
	}

	// Fetch VMs inventory to get datastore references from VMs
	vmsInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "vms?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %v", err)
	}

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	// Extract datastore IDs used by the plan VMs
	datastoreIDSet := make(map[string]bool)
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

		// Extract datastore IDs from VM disks (VSphere VMs have direct disks array)
		disks, err := query.GetValueByPathString(vm, "disks")
		if err == nil && disks != nil {
			if disksArray, ok := disks.([]interface{}); ok {
				klog.V(4).Infof("VM %s has %d disks", vmName, len(disksArray))
				for _, diskItem := range disksArray {
					if diskMap, ok := diskItem.(map[string]interface{}); ok {
						datastoreID, err := query.GetValueByPathString(diskMap, "datastore.id")
						if err == nil && datastoreID != nil {
							if dsID, ok := datastoreID.(string); ok {
								klog.V(4).Infof("Found datastore ID: %s", dsID)
								datastoreIDSet[dsID] = true
							}
						}
					}
				}
			}
		} else {
			klog.V(4).Infof("VM %s has no disks or failed to extract: err=%v", vmName, err)
		}
	}

	klog.V(4).Infof("Final datastoreIDSet: %v", datastoreIDSet)

	// If no datastores found from VMs, return empty list
	if len(datastoreIDSet) == 0 {
		klog.V(4).Infof("No datastores found from VMs")
		return []ref.Ref{}, nil
	}

	// Build source storages list using the collected IDs
	var sourceStorages []ref.Ref
	for datastoreID := range datastoreIDSet {
		if datastoreItem, exists := datastoreIDToDatastore[datastoreID]; exists {
			sourceStorage := ref.Ref{
				ID: datastoreID,
			}
			if name, ok := datastoreItem["name"].(string); ok {
				sourceStorage.Name = name
			}
			sourceStorages = append(sourceStorages, sourceStorage)
		}
	}

	klog.V(4).Infof("VSphere storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages is not supported for VSphere as target - only OpenShift is supported as target
func (f *VSphereStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("VSphere provider does not support target storage fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("VSphere provider does not support target storage fetching - only OpenShift is supported as migration target")
}
