package nutanix

import (
	"context"
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
	"github.com/yaacov/kubectl-mtv/pkg/util/query"
)

// StorageFetcher implements storage fetching for Nutanix providers.
type StorageFetcher struct{}

// NewStorageFetcher creates a Nutanix storage fetcher.
func NewStorageFetcher() *StorageFetcher {
	return &StorageFetcher{}
}

// FetchSourceStorages extracts storage container refs from Nutanix VM disks.
func (f *StorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %w", err)
	}

	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storagecontainers?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage containers inventory: %w", err)
	}
	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	idToStorage := make(map[string]map[string]interface{})
	for _, item := range storageArray {
		if storage, ok := item.(map[string]interface{}); ok {
			if id, ok := storage["id"].(string); ok {
				idToStorage[id] = storage
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

	storageIDSet := make(map[string]bool)
	for _, item := range vmsArray {
		vm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vmName, ok := vm["name"].(string)
		if !ok || !planVMSet[vmName] {
			continue
		}
		disks, err := query.GetValueByPathString(vm, "disks")
		if err != nil || disks == nil {
			continue
		}
		disksArray, ok := disks.([]interface{})
		if !ok {
			continue
		}
		for _, diskItem := range disksArray {
			diskMap, ok := diskItem.(map[string]interface{})
			if !ok {
				continue
			}
			if isCdrom, _ := diskMap["isCdrom"].(bool); isCdrom {
				continue
			}
			if containerUUID, ok := diskMap["storageContainerUuid"].(string); ok && containerUUID != "" {
				storageIDSet[containerUUID] = true
			}
		}
	}

	var sourceStorages []ref.Ref
	for storageID := range storageIDSet {
		storageRef := ref.Ref{ID: storageID}
		if storageItem, exists := idToStorage[storageID]; exists {
			if name, ok := storageItem["name"].(string); ok {
				storageRef.Name = name
			}
		}
		sourceStorages = append(sourceStorages, storageRef)
	}

	klog.V(4).Infof("Nutanix storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages is not supported; Nutanix is a source provider only.
func (f *StorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	return nil, fmt.Errorf("nutanix provider does not support target storage fetching - only OpenShift is supported as migration target")
}
