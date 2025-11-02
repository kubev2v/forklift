package openstack

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

// OpenStackStorageFetcher implements storage fetching for OpenStack providers
type OpenStackStorageFetcher struct{}

// NewOpenStackStorageFetcher creates a new OpenStack storage fetcher
func NewOpenStackStorageFetcher() *OpenStackStorageFetcher {
	return &OpenStackStorageFetcher{}
}

// FetchSourceStorages extracts storage references from OpenStack VMs
func (f *OpenStackStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error) {
	klog.V(4).Infof("OpenStack storage fetcher - extracting source storages for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch volume types inventory first to create ID-to-volumeType mapping
	volumeTypesInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "volumetypes?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch volume types inventory: %v", err)
	}

	volumeTypesArray, ok := volumeTypesInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for volume types inventory")
	}

	volumeTypeIDToVolumeType := make(map[string]map[string]interface{})
	volumeTypeNameToID := make(map[string]string)
	for _, item := range volumeTypesArray {
		if volumeType, ok := item.(map[string]interface{}); ok {
			if volumeTypeID, ok := volumeType["id"].(string); ok {
				volumeTypeIDToVolumeType[volumeTypeID] = volumeType
				// Also create name-to-ID mapping for converting volume type names to IDs
				if volumeTypeName, ok := volumeType["name"].(string); ok {
					volumeTypeNameToID[volumeTypeName] = volumeTypeID
				}
			}
		}
	}

	klog.V(4).Infof("DEBUG: Available volume type mappings:")
	for id, volumeType := range volumeTypeIDToVolumeType {
		if name, ok := volumeType["name"].(string); ok {
			klog.V(4).Infof("  %s -> %s", id, name)
		}
	}

	// Fetch VMs inventory to get volume IDs from VMs
	vmsInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "vms?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %v", err)
	}

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	volumeIDSet := make(map[string]bool)
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

		volumeIDs, err := query.GetValueByPathString(vm, "attachedVolumes[*].ID")
		if err != nil || volumeIDs == nil {
			klog.V(4).Infof("VM %s has no attached volumes or failed to extract: err=%v", vmName, err)
			continue
		}

		if ids, ok := volumeIDs.([]interface{}); ok {
			for _, idItem := range ids {
				if volumeID, ok := idItem.(string); ok {
					klog.V(4).Infof("Found volume ID: %s", volumeID)
					volumeIDSet[volumeID] = true
				}
			}
		}
	}

	volumesInventory, err := client.FetchProviderInventory(configFlags, inventoryURL, provider, "volumes?detail=4")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch volumes inventory: %v", err)
	}

	volumesArray, ok := volumesInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for volumes inventory")
	}

	volumeTypeIDSet := make(map[string]bool)
	for _, item := range volumesArray {
		volumeItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		volumeID, ok := volumeItem["id"].(string)
		if !ok {
			continue
		}

		if !volumeIDSet[volumeID] {
			continue
		}

		klog.V(4).Infof("Processing volume: %s", volumeID)

		volumeType, err := query.GetValueByPathString(volumeItem, "volumeType")
		if err == nil && volumeType != nil {
			if vtNameOrID, ok := volumeType.(string); ok {
				klog.V(4).Infof("Volume %s has volume type: %s", volumeID, vtNameOrID)

				if _, exists := volumeTypeIDToVolumeType[vtNameOrID]; exists {
					volumeTypeIDSet[vtNameOrID] = true
					klog.V(4).Infof("Volume type is already an ID: %s", vtNameOrID)
				} else {
					if volumeTypeID, exists := volumeTypeNameToID[vtNameOrID]; exists {
						volumeTypeIDSet[volumeTypeID] = true
						klog.V(4).Infof("Converted volume type name %s to ID: %s", vtNameOrID, volumeTypeID)
					} else {
						klog.V(4).Infof("No volume type ID found for name: %s", vtNameOrID)
					}
				}
			}
		} else {
			klog.V(4).Infof("Volume %s has no volume type or failed to extract: err=%v", volumeID, err)
		}
	}

	klog.V(4).Infof("DEBUG: Final volumeTypeIDSet: %v", volumeTypeIDSet)

	// If no volume types found from VMs, return empty list
	if len(volumeTypeIDSet) == 0 {
		klog.V(4).Infof("No volume types found from VMs - VMs have incomplete data")
		return []ref.Ref{}, nil
	}

	var sourceStorages []ref.Ref
	for volumeTypeID := range volumeTypeIDSet {
		if volumeTypeItem, exists := volumeTypeIDToVolumeType[volumeTypeID]; exists {
			sourceStorage := ref.Ref{
				ID: volumeTypeID,
			}
			if name, ok := volumeTypeItem["name"].(string); ok {
				sourceStorage.Name = name
			}
			sourceStorages = append(sourceStorages, sourceStorage)
		}
	}

	klog.V(4).Infof("OpenStack storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages is not supported for OpenStack as target - only OpenShift is supported as target
func (f *OpenStackStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("OpenStack provider does not support target storage fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("OpenStack provider does not support target storage fetching - only OpenShift is supported as migration target")
}
