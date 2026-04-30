package openshift

import "context"

import (
	"fmt"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// FetchSourceStorages extracts storage references from OpenShift VMs by looking
// up the actual PVCs that back each VM volume. The PVC's spec.storageClassName
// is the authoritative source -- it reflects the resolved SC even when the
// dataVolumeTemplate didn't specify one explicitly.
func (f *OpenShiftStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("OpenShift storage fetcher - extracting source storages for provider: %s", providerName)

	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Build StorageClass lookup maps (name -> ID, ID -> inventory item).
	storageIDToStorage, storageNameToID, err := fetchStorageClassMaps(ctx, configFlags, inventoryURL, provider, insecureSkipTLS)
	if err != nil {
		return nil, err
	}

	// Build PVC lookup map (namespace/name -> storageClassName).
	pvcSCMap, err := fetchPVCStorageClassMap(ctx, configFlags, inventoryURL, provider, insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PVC inventory: %v", err)
	}

	// Fetch VMs inventory.
	vmsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "vms?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %v", err)
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
		vmName, _ := vm["name"].(string)
		if !planVMSet[vmName] {
			continue
		}

		vmNamespace, _ := query.GetValueByPathString(vm, "object.metadata.namespace")
		vmNS, _ := vmNamespace.(string)
		klog.V(4).Infof("Processing VM: %s (namespace: %s)", vmName, vmNS)

		collectStorageFromVM(vm, vmNS, storageIDSet, storageNameToID, pvcSCMap)
	}

	klog.V(4).Infof("Final storageIDSet: %v", storageIDSet)

	if len(storageIDSet) == 0 {
		klog.V(4).Infof("No storages found from VMs")
		return []ref.Ref{}, nil
	}

	var sourceStorages []ref.Ref
	for storageID := range storageIDSet {
		if storageItem, exists := storageIDToStorage[storageID]; exists {
			sourceStorage := ref.Ref{ID: storageID}
			if name, ok := storageItem["name"].(string); ok {
				sourceStorage.Name = name
			}
			sourceStorages = append(sourceStorages, sourceStorage)
		}
	}

	klog.V(4).Infof("OpenShift storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// fetchStorageClassMaps returns ID-to-item and name-to-ID maps for StorageClasses.
func fetchStorageClassMaps(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, insecureSkipTLS bool) (map[string]map[string]interface{}, map[string]string, error) {
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storageclasses?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}
	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	idToStorage := make(map[string]map[string]interface{}, len(storageArray))
	nameToID := make(map[string]string, len(storageArray))
	for _, item := range storageArray {
		if sc, ok := item.(map[string]interface{}); ok {
			if id, ok := sc["id"].(string); ok {
				idToStorage[id] = sc
				if name, ok := sc["name"].(string); ok {
					nameToID[name] = id
				}
			}
		}
	}

	klog.V(4).Infof("Available StorageClass mappings (%d):", len(idToStorage))
	for id, sc := range idToStorage {
		if name, ok := sc["name"].(string); ok {
			klog.V(4).Infof("  %s -> %s", id, name)
		}
	}
	return idToStorage, nameToID, nil
}

// fetchPVCStorageClassMap builds a "namespace/name" -> storageClassName map from
// the PVC inventory. This is the reliable way to discover the resolved SC for
// volumes that didn't specify one explicitly in the dataVolumeTemplate.
func fetchPVCStorageClassMap(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, insecureSkipTLS bool) (map[string]string, error) {
	pvcInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "persistentvolumeclaims?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, err
	}
	pvcArray, ok := pvcInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for PVC inventory")
	}

	pvcMap := make(map[string]string, len(pvcArray))
	for _, item := range pvcArray {
		pvc, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := pvc["name"].(string)
		ns, _ := pvc["namespace"].(string)
		if name == "" {
			continue
		}
		scName, _ := query.GetValueByPathString(pvc, "object.spec.storageClassName")
		if sc, ok := scName.(string); ok && sc != "" {
			key := ns + "/" + name
			pvcMap[key] = sc
			klog.V(4).Infof("PVC %s -> storageClassName: %s", key, sc)
		}
	}
	klog.V(4).Infof("Built PVC storage class map with %d entries", len(pvcMap))
	return pvcMap, nil
}

// collectStorageFromVM extracts storage class IDs from a single VM.
//
// Strategy per dataVolumeTemplate:
//  1. Explicit spec.storageClassName in the template -> use it directly.
//  2. Look up the PVC by template name in the inventory -> use its resolved storageClassName.
//  3. Explicit spec.storage.storageClassName in the template -> use it.
func collectStorageFromVM(vm map[string]interface{}, vmNamespace string, storageIDSet map[string]bool, storageNameToID map[string]string, pvcSCMap map[string]string) {
	dvtTemplates, err := query.GetValueByPathString(vm, "object.spec.dataVolumeTemplates")
	if err != nil || dvtTemplates == nil {
		return
	}
	dvtArray, ok := dvtTemplates.([]interface{})
	if !ok {
		return
	}

	vmName, _ := vm["name"].(string)
	klog.V(4).Infof("VM %s has %d dataVolumeTemplates", vmName, len(dvtArray))

	for _, dvtItem := range dvtArray {
		dvtMap, ok := dvtItem.(map[string]interface{})
		if !ok {
			continue
		}

		dvtName, _ := query.GetValueByPathString(dvtMap, "metadata.name")
		dvtNameStr, _ := dvtName.(string)

		// 1. Explicit storageClassName on the template spec.
		if scVal, err := query.GetValueByPathString(dvtMap, "spec.storageClassName"); err == nil && scVal != nil {
			if scName, ok := scVal.(string); ok && scName != "" {
				klog.V(4).Infof("Found explicit storageClassName in template: %s", scName)
				if id, exists := storageNameToID[scName]; exists {
					storageIDSet[id] = true
				}
				continue
			}
		}

		// 2. Look up the actual PVC in the inventory to get the resolved SC.
		if pvcSCMap != nil && dvtNameStr != "" && vmNamespace != "" {
			pvcKey := vmNamespace + "/" + dvtNameStr
			if scName, exists := pvcSCMap[pvcKey]; exists {
				klog.V(4).Infof("Resolved storageClassName from PVC %s: %s", pvcKey, scName)
				if id, exists := storageNameToID[scName]; exists {
					storageIDSet[id] = true
				}
				continue
			}
		}

		// 3. Explicit storageClassName inside spec.storage.
		if scVal, err := query.GetValueByPathString(dvtMap, "spec.storage.storageClassName"); err == nil && scVal != nil {
			if scName, ok := scVal.(string); ok && scName != "" {
				klog.V(4).Infof("Found storageClassName in spec.storage: %s", scName)
				if id, exists := storageNameToID[scName]; exists {
					storageIDSet[id] = true
				}
				continue
			}
		}

		klog.V(2).Infof("WARNING: could not determine storageClassName for dataVolumeTemplate %q on VM %s", dvtNameStr, vmName)
	}
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

	return buildTargetStorageList(storageArray)
}

// buildTargetStorageList selects the best default SC and returns all SCs with
// the default at index 0.
//
// Priority: virt annotation > k8s annotation > name contains "virtualization" > first available.
func buildTargetStorageList(storageArray []interface{}) ([]forkliftv1beta1.DestinationStorage, error) {
	var virtAnnotationStorage, k8sAnnotationStorage, virtualizationNameStorage, firstStorage map[string]interface{}

	for _, item := range storageArray {
		storageItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if firstStorage == nil {
			firstStorage = storageItem
		}

		storageName := ""
		if name, ok := storageItem["name"].(string); ok {
			storageName = name
		}

		if virtualizationNameStorage == nil && strings.Contains(strings.ToLower(storageName), "virtualization") {
			klog.V(4).Infof("Found storage class with 'virtualization' in name: %s", storageName)
			virtualizationNameStorage = storageItem
		}

		if object, ok := storageItem["object"].(map[string]interface{}); ok {
			if metadata, ok := object["metadata"].(map[string]interface{}); ok {
				if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
					if virtAnnotationStorage == nil {
						if virtDefault, ok := annotations["storageclass.kubevirt.io/is-default-virt-class"].(string); ok && virtDefault == "true" {
							klog.V(4).Infof("Found storage class with virt default annotation: %s", storageName)
							virtAnnotationStorage = storageItem
						}
					}

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

	selectedName := ""
	if name, ok := selectedStorage["name"].(string); ok {
		selectedName = name
	}

	klog.V(4).Infof("Selected default storage class '%s' based on: %s", selectedName, selectionReason)

	// Return all storage classes with the best (default) one first.
	targetStorages := make([]forkliftv1beta1.DestinationStorage, 0, len(storageArray))
	if selectedName != "" {
		targetStorages = append(targetStorages, forkliftv1beta1.DestinationStorage{
			StorageClass: selectedName,
		})
	}

	for _, item := range storageArray {
		storageItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := storageItem["name"].(string)
		if name != "" && name != selectedName {
			targetStorages = append(targetStorages, forkliftv1beta1.DestinationStorage{
				StorageClass: name,
			})
		}
	}

	klog.V(4).Infof("Returning %d target storages (default: %s)", len(targetStorages), selectedName)
	return targetStorages, nil
}
