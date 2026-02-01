package openshift

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

// OpenShiftStorageFetcher implements storage fetching for OpenShift providers
type OpenShiftStorageFetcher struct{}

// NewOpenShiftStorageFetcher creates a new OpenShift storage fetcher
func NewOpenShiftStorageFetcher() *OpenShiftStorageFetcher {
	return &OpenShiftStorageFetcher{}
}

// FetchSourceStorages extracts storage references from OpenShift VMs
func (f *OpenShiftStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("OpenShift storage fetcher - extracting source storages for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch storage inventory (StorageClasses in OpenShift) first to create ID-to-storage mapping
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storageclasses?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Create ID-to-storage and name-to-ID mappings for StorageClasses
	storageIDToStorage := make(map[string]map[string]interface{})
	storageNameToID := make(map[string]string)
	for _, item := range storageArray {
		if storage, ok := item.(map[string]interface{}); ok {
			if storageID, ok := storage["id"].(string); ok {
				storageIDToStorage[storageID] = storage
				if storageName, ok := storage["name"].(string); ok {
					storageNameToID[storageName] = storageID
				}
			}
		}
	}

	klog.V(4).Infof("Available StorageClass mappings:")
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

		// Extract storage references from VM spec (OpenShift VMs use dataVolumeTemplates and volumes)
		dataVolumeTemplates, err := query.GetValueByPathString(vm, "object.spec.dataVolumeTemplates")
		if err == nil && dataVolumeTemplates != nil {
			if dvtArray, ok := dataVolumeTemplates.([]interface{}); ok {
				klog.V(4).Infof("VM %s has %d dataVolumeTemplates", vmName, len(dvtArray))
				for _, dvtItem := range dvtArray {
					if dvtMap, ok := dvtItem.(map[string]interface{}); ok {
						// Look for storageClassName in spec.storageClassName
						storageClassName, err := query.GetValueByPathString(dvtMap, "spec.storageClassName")
						if err == nil && storageClassName != nil {
							if scName, ok := storageClassName.(string); ok {
								klog.V(4).Infof("Found explicit storageClassName: %s", scName)
								if storageID, exists := storageNameToID[scName]; exists {
									storageIDSet[storageID] = true
								}
							}
						} else {
							// No explicit storageClassName - check if this dataVolumeTemplate has storage requirements
							// This indicates it uses the default storage class
							storage, err := query.GetValueByPathString(dvtMap, "spec.storage")
							if err == nil && storage != nil {
								klog.V(4).Infof("Found dataVolumeTemplate with storage but no explicit storageClassName - using default storage class")
								// Find the default storage class
								for storageID, storageItem := range storageIDToStorage {
									isDefaultValue, err := query.GetValueByPathString(storageItem, "object.metadata.annotations.storageclass.kubernetes.io/is-default-class")
									if err == nil && isDefaultValue != nil {
										if isDefault, ok := isDefaultValue.(string); ok && isDefault == "true" {
											klog.V(4).Infof("Using default StorageClass for VM %s: %s", vmName, storageID)
											storageIDSet[storageID] = true
											break
										}
									}
								}
							}
						}
					}
				}
			}
		}

		volumes, err := query.GetValueByPathString(vm, "object.spec.template.spec.volumes")
		if err == nil && volumes != nil {
			if volumesArray, ok := volumes.([]interface{}); ok {
				klog.V(4).Infof("VM %s has %d volumes", vmName, len(volumesArray))
				for _, volumeItem := range volumesArray {
					if volumeMap, ok := volumeItem.(map[string]interface{}); ok {
						// Check if this volume references a DataVolume (which may have storage class info)
						dataVolume, err := query.GetValueByPathString(volumeMap, "dataVolume")
						if err == nil && dataVolume != nil {
							klog.V(4).Infof("Found volume with dataVolume reference in VM %s", vmName)
							// The actual storage class info is in the dataVolumeTemplates we already processed
						} else {
							klog.V(4).Infof("Found volume in VM %s", vmName)
						}
					}
				}
			}
		}
	}

	klog.V(4).Infof("Final storageIDSet: %v", storageIDSet)

	// If no storages found from VMs, still try to find a default storage class
	// This handles cases where VMs exist but don't have explicit storage references
	if len(storageIDSet) == 0 {
		klog.V(4).Infof("No explicit storage found from VMs, looking for default storage class")
		for storageID, storageItem := range storageIDToStorage {
			isDefaultValue, err := query.GetValueByPathString(storageItem, "object.metadata.annotations.storageclass.kubernetes.io/is-default-class")
			if err == nil && isDefaultValue != nil {
				if isDefault, ok := isDefaultValue.(string); ok && isDefault == "true" {
					klog.V(4).Infof("Found and using default StorageClass: %s", storageID)
					storageIDSet[storageID] = true
					break
				}
			}
		}
	}

	// If still no storages found, return empty list
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

	klog.V(4).Infof("OpenShift storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages extracts available destination storages from target provider
func (f *OpenShiftStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("OpenShift storage fetcher - extracting target storages for provider: %s", providerName)

	// Get the target provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get target provider: %v", err)
	}

	// For OpenShift targets, always fetch StorageClasses
	klog.V(4).Infof("Fetching StorageClasses for OpenShift target")
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storageclasses?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for target storage inventory")
	}

	// Parse all storage classes and find the best one using priority logic
	var virtAnnotationStorage, k8sAnnotationStorage, virtualizationNameStorage, firstStorage map[string]interface{}

	for _, item := range storageArray {
		storageItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Set first storage if not already set
		if firstStorage == nil {
			firstStorage = storageItem
		}

		storageName := ""
		if name, ok := storageItem["name"].(string); ok {
			storageName = name
		}

		// Check for storage class name containing "virtualization"
		if virtualizationNameStorage == nil && strings.Contains(strings.ToLower(storageName), "virtualization") {
			klog.V(4).Infof("Found storage class with 'virtualization' in name: %s", storageName)
			virtualizationNameStorage = storageItem
		}

		// Check for annotations in the object.metadata.annotations
		if object, ok := storageItem["object"].(map[string]interface{}); ok {
			if metadata, ok := object["metadata"].(map[string]interface{}); ok {
				if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
					// Check for virt default annotation
					if virtAnnotationStorage == nil {
						if virtDefault, ok := annotations["storageclass.kubevirt.io/is-default-virt-class"].(string); ok && virtDefault == "true" {
							klog.V(4).Infof("Found storage class with virt default annotation: %s", storageName)
							virtAnnotationStorage = storageItem
						}
					}

					// Check for k8s default annotation
					if k8sAnnotationStorage == nil {
						if k8sDefault, ok := annotations["storageclass.kubernetes.io/is-default-class"].(string); ok && k8sDefault == "true" {
							klog.V(4).Infof("Found storage class with k8s default annotation: %s", storageName)
							k8sAnnotationStorage = storageItem
						}
					}
				}
			}
		}
	}

	// Priority: virt annotation > k8s annotation (only if no virt found) > name with "virtualization" > first available
	var selectedStorage map[string]interface{}
	var selectionReason string

	if virtAnnotationStorage != nil {
		selectedStorage = virtAnnotationStorage
		selectionReason = "virt default annotation"
	} else if k8sAnnotationStorage != nil {
		selectedStorage = k8sAnnotationStorage
		selectionReason = "k8s default annotation"
	} else if virtualizationNameStorage != nil {
		selectedStorage = virtualizationNameStorage
		selectionReason = "name contains 'virtualization'"
	} else if firstStorage != nil {
		selectedStorage = firstStorage
		selectionReason = "first available"
	} else {
		return nil, fmt.Errorf("no storage classes found")
	}

	storageName := ""
	if name, ok := selectedStorage["name"].(string); ok {
		storageName = name
	}

	klog.V(4).Infof("Selected storage class '%s' based on: %s", storageName, selectionReason)

	// Return only the selected storage class
	targetStorages := []forkliftv1beta1.DestinationStorage{
		{
			StorageClass: storageName,
		},
	}

	klog.V(4).Infof("Returning single target storage: %s", storageName)
	return targetStorages, nil
}
