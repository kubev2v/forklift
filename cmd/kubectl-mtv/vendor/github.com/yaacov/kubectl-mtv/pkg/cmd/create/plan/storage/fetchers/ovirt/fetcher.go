package ovirt

import "context"

import (
	"fmt"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// OvirtStorageFetcher implements storage fetching for oVirt providers
type OvirtStorageFetcher struct{}

// NewOvirtStorageFetcher creates a new oVirt storage fetcher
func NewOvirtStorageFetcher() *OvirtStorageFetcher {
	return &OvirtStorageFetcher{}
}

// FetchSourceStorages extracts storage references from oVirt VMs
func (f *OvirtStorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("oVirt storage fetcher - extracting source storages for provider: %s", providerName)

	// Get the provider object
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch storage domains inventory first to create ID-to-storage mapping
	storageDomainsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storagedomains?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch storage domains inventory: %v", err)
	}

	storageDomainsArray, ok := storageDomainsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage domains inventory")
	}

	// Create ID-to-storage domain mapping
	storageDomainIDToStorageDomain := make(map[string]map[string]interface{})
	for _, item := range storageDomainsArray {
		if storageDomain, ok := item.(map[string]interface{}); ok {
			if storageDomainID, ok := storageDomain["id"].(string); ok {
				storageDomainIDToStorageDomain[storageDomainID] = storageDomain
			}
		}
	}

	klog.V(4).Infof("Available storage domain mappings:")
	for id, storageDomainItem := range storageDomainIDToStorageDomain {
		if name, ok := storageDomainItem["name"].(string); ok {
			klog.V(4).Infof("  %s -> %s", id, name)
		}
	}

	// Fetch VMs inventory to get disk references from VMs
	vmsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "vms?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VMs inventory: %v", err)
	}

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	// Extract disk IDs used by the plan VMs
	diskIDSet := make(map[string]bool)
	planVMSet := make(map[string]bool)
	for _, vmName := range planVMNames {
		planVMSet[vmName] = true
	}

	for _, vmItem := range vmsArray {
		if vm, ok := vmItem.(map[string]interface{}); ok {
			if vmName, ok := vm["name"].(string); ok && planVMSet[vmName] {
				klog.V(4).Infof("Processing VM: %s", vmName)

				// Extract disk IDs from VM diskAttachments
				if diskAttachments, ok := vm["diskAttachments"].([]interface{}); ok {
					for _, diskAttachmentItem := range diskAttachments {
						if diskAttachment, ok := diskAttachmentItem.(map[string]interface{}); ok {
							if diskID, ok := diskAttachment["disk"].(string); ok {
								klog.V(4).Infof("Found disk ID: %s", diskID)
								diskIDSet[diskID] = true
							}
						}
					}
				}
			}
		}
	}

	// Fetch disk details to get storage domain information
	storageDomainIDSet := make(map[string]bool)

	// Try to fetch disks inventory to get storage domain mappings
	disksInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "disks?detail=4", insecureSkipTLS)
	if err != nil {
		// If disks endpoint doesn't work, try disk profiles as fallback
		klog.V(4).Infof("Disks endpoint failed, trying disk profiles: %v", err)

		// Fetch disk profiles to map disks to storage domains
		diskProfilesInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "diskprofiles?detail=4", insecureSkipTLS)
		if err != nil {
			klog.V(4).Infof("Warning: Could not fetch disk profiles either: %v", err)
			// Return all storage domains as fallback
			for storageDomainID := range storageDomainIDToStorageDomain {
				storageDomainIDSet[storageDomainID] = true
			}
		} else {
			diskProfilesArray, ok := diskProfilesInventory.([]interface{})
			if ok {
				// Use all storage domains from disk profiles as fallback
				for _, item := range diskProfilesArray {
					if profile, ok := item.(map[string]interface{}); ok {
						if storageDomainID, ok := profile["storageDomain"].(string); ok {
							storageDomainIDSet[storageDomainID] = true
						}
					}
				}
			}
		}
	} else {
		disksArray, ok := disksInventory.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected data format: expected array for disks inventory")
		}

		// Map disk IDs to storage domains
		for _, diskItem := range disksArray {
			if disk, ok := diskItem.(map[string]interface{}); ok {
				if diskID, ok := disk["id"].(string); ok {
					// Check if this disk is used by our VMs
					if diskIDSet[diskID] {
						// Extract storage domain from disk
						if storageDomainID, ok := disk["storageDomain"].(string); ok {
							klog.V(4).Infof("Disk %s uses storage domain: %s", diskID, storageDomainID)
							storageDomainIDSet[storageDomainID] = true
						}
					}
				}
			}
		}
	}

	// Create source storage references for the storage domains used by VMs
	var sourceStorages []ref.Ref
	for storageDomainID := range storageDomainIDSet {
		sourceStorages = append(sourceStorages, ref.Ref{
			ID: storageDomainID,
		})
	}

	klog.V(4).Infof("oVirt storage fetcher - found %d source storages", len(sourceStorages))
	return sourceStorages, nil
}

// FetchTargetStorages is not supported for oVirt as target - only OpenShift is supported as target
func (f *OvirtStorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("oVirt provider does not support target storage fetching - only OpenShift is supported as target")
	return nil, fmt.Errorf("oVirt provider does not support target storage fetching - only OpenShift is supported as migration target")
}
