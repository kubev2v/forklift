package mapping

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// resolveVSphereStorageNameToID resolves storage name for VMware vSphere provider
func resolveVSphereStorageNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, storageName string, insecureSkipTLS bool) ([]ref.Ref, error) {
	// Fetch datastores from VMware vSphere
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "datastores?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Search for the storage by name
	var matchingRefs []ref.Ref
	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := storage["name"].(string)
		id, _ := storage["id"].(string)

		if name == storageName {
			matchingRefs = append(matchingRefs, ref.Ref{
				Name: name,
				ID:   id,
			})
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("datastore '%s' not found in vSphere provider inventory", storageName)
	}

	return matchingRefs, nil
}

// resolveOvirtStorageNameToID resolves storage name for oVirt provider
func resolveOvirtStorageNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, storageName string, insecureSkipTLS bool) ([]ref.Ref, error) {
	// Fetch storage domains from oVirt
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storagedomains?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Search for the storage by name
	var matchingRefs []ref.Ref
	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := storage["name"].(string)
		id, _ := storage["id"].(string)

		if name == storageName {
			matchingRefs = append(matchingRefs, ref.Ref{
				Name: name,
				ID:   id,
			})
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("storage domain '%s' not found in oVirt provider inventory", storageName)
	}

	return matchingRefs, nil
}

// resolveOpenStackStorageNameToID resolves storage name for OpenStack provider
func resolveOpenStackStorageNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, storageName string, insecureSkipTLS bool) ([]ref.Ref, error) {
	// Handle '__DEFAULT__' as a special case - return ref with type 'default'
	if storageName == "__DEFAULT__" {
		return []ref.Ref{{
			Type: "default",
		}}, nil
	}

	// Fetch storage types from OpenStack
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "volumetypes?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Search for the storage by name
	var matchingRefs []ref.Ref
	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := storage["name"].(string)
		id, _ := storage["id"].(string)

		if name == storageName {
			matchingRefs = append(matchingRefs, ref.Ref{
				Name: name,
				ID:   id,
			})
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("storage type '%s' not found in OpenStack provider inventory", storageName)
	}

	return matchingRefs, nil
}

// resolveOVAStorageNameToID resolves storage name for OVA provider
func resolveOVAStorageNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, storageName string, insecureSkipTLS bool) ([]ref.Ref, error) {
	// Fetch storage from OVA
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Search for the storage by name
	var matchingRefs []ref.Ref
	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := storage["name"].(string)
		id, _ := storage["id"].(string)

		if name == storageName {
			matchingRefs = append(matchingRefs, ref.Ref{
				Name: name,
				ID:   id,
			})
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("storage '%s' not found in OVA provider inventory", storageName)
	}

	return matchingRefs, nil
}

// resolveStorageNameToID resolves a storage name to its ref.Ref by querying the provider inventory
func resolveStorageNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL, storageName string, insecureSkipTLS bool) ([]ref.Ref, error) {
	// Get source provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider '%s': %v", providerName, err)
	}

	// Check provider type to determine which helper to use
	providerType, _, err := unstructured.NestedString(provider.Object, "spec", "type")
	if err != nil {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	switch providerType {
	case "openshift":
		// For OpenShift source providers, only include the name in the source reference
		// Storage classes are cluster-scoped resources, so we don't need to resolve the ID
		return []ref.Ref{{
			Name: storageName,
		}}, nil
	case "ec2":
		return resolveEC2StorageNameToID(ctx, configFlags, inventoryURL, provider, storageName, insecureSkipTLS)
	case "vsphere":
		return resolveVSphereStorageNameToID(ctx, configFlags, inventoryURL, provider, storageName, insecureSkipTLS)
	case "ovirt":
		return resolveOvirtStorageNameToID(ctx, configFlags, inventoryURL, provider, storageName, insecureSkipTLS)
	case "openstack":
		return resolveOpenStackStorageNameToID(ctx, configFlags, inventoryURL, provider, storageName, insecureSkipTLS)
	case "ova":
		return resolveOVAStorageNameToID(ctx, configFlags, inventoryURL, provider, storageName, insecureSkipTLS)
	default:
		// Default to generic storage endpoint for unknown providers
		storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
		}

		storageArray, ok := storageInventory.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
		}

		// Search for all storages matching the name
		var matchingRefs []ref.Ref
		for _, item := range storageArray {
			storage, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := storage["name"].(string)
			id, _ := storage["id"].(string)

			if name == storageName {
				matchingRefs = append(matchingRefs, ref.Ref{
					ID: id,
				})
			}
		}

		if len(matchingRefs) == 0 {
			return nil, fmt.Errorf("storage '%s' not found in provider '%s' inventory", storageName, providerName)
		}

		return matchingRefs, nil
	}
}

// resolveEC2StorageNameToID resolves storage name for EC2 provider
func resolveEC2StorageNameToID(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, storageName string, insecureSkipTLS bool) ([]ref.Ref, error) {
	// Fetch EBS volume types from EC2
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage inventory: %v", err)
	}

	// Extract objects from EC2 envelope
	storageInventory = inventory.ExtractEC2Objects(storageInventory)

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Search for the storage by type (gp2, gp3, io1, io2, st1, sc1, standard)
	// Use case-insensitive matching and deduplicate results
	var matchingRefs []ref.Ref
	seen := make(map[string]struct{})
	storageNameLower := strings.ToLower(storageName)

	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// EC2 storage uses "type" field for EBS volume types
		volumeType, _ := storage["type"].(string)

		// Case-insensitive match
		if strings.ToLower(volumeType) == storageNameLower {
			// Deduplicate - only add if not seen before
			if _, exists := seen[volumeType]; exists {
				continue
			}
			seen[volumeType] = struct{}{}

			matchingRefs = append(matchingRefs, ref.Ref{
				Name: volumeType,
			})
		}
	}

	if len(matchingRefs) == 0 {
		return nil, fmt.Errorf("EBS volume type '%s' not found in EC2 provider inventory", storageName)
	}

	return matchingRefs, nil
}
