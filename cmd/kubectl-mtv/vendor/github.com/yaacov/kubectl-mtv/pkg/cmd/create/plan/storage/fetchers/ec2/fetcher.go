package ec2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// EC2StorageFetcher implements storage fetching for EC2 providers
type EC2StorageFetcher struct{}

// NewEC2StorageFetcher creates a new EC2 storage fetcher
func NewEC2StorageFetcher() fetchers.StorageFetcher {
	return &EC2StorageFetcher{}
}

// FetchSourceStorages fetches EBS volume types from EC2 provider.
// When planVMNames is non-empty, only volume types actually used by those VMs are returned.
func (f *EC2StorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: EC2 - Fetching source storage types from provider: %s", providerName)

	// Get provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get EC2 provider: %v", err)
	}

	// If planVMNames provided, filter to only volume types used by those VMs
	if len(planVMNames) > 0 {
		return f.fetchVolumeTypesFromVMs(ctx, configFlags, inventoryURL, provider, planVMNames, insecureSkipTLS)
	}

	// Fallback: return all available volume types
	return f.fetchAllVolumeTypes(ctx, configFlags, inventoryURL, provider, insecureSkipTLS)
}

// fetchAllVolumeTypes fetches all unique EBS volume types from the storages endpoint.
func (f *EC2StorageFetcher) fetchAllVolumeTypes(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, insecureSkipTLS bool) ([]ref.Ref, error) {
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EC2 storage inventory: %v", err)
	}

	storageInventory = inventory.ExtractEC2Objects(storageInventory)

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	volumeTypeSet := make(map[string]struct{})
	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		volumeType, ok := storage["type"].(string)
		if ok && volumeType != "" {
			volumeTypeSet[strings.ToLower(volumeType)] = struct{}{}
		}
	}

	return f.buildSortedRefs(volumeTypeSet), nil
}

// fetchVolumeTypesFromVMs fetches VMs, extracts their EBS volume IDs, cross-references
// with volumes inventory to get volume types, and returns only those types.
func (f *EC2StorageFetcher) fetchVolumeTypesFromVMs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, inventoryURL string, provider *unstructured.Unstructured, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: EC2 - Filtering source storages by %d plan VMs", len(planVMNames))

	// Fetch VMs inventory
	vmsInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "vms?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EC2 VMs inventory: %v", err)
	}

	vmsInventory = inventory.ExtractEC2Objects(vmsInventory)

	vmsArray, ok := vmsInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for VMs inventory")
	}

	planVMSet := make(map[string]bool, len(planVMNames))
	for _, name := range planVMNames {
		planVMSet[name] = true
	}

	// Extract volume IDs from plan VMs' BlockDeviceMappings
	volumeIDSet := make(map[string]struct{})
	for _, item := range vmsArray {
		vm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vmName, _ := vm["name"].(string)
		if !planVMSet[vmName] {
			continue
		}

		// EC2 VMs have object.BlockDeviceMappings[].Ebs.VolumeId
		obj, _ := vm["object"].(map[string]interface{})
		if obj == nil {
			continue
		}
		bdms, _ := obj["BlockDeviceMappings"].([]interface{})
		for _, bdmItem := range bdms {
			bdm, ok := bdmItem.(map[string]interface{})
			if !ok {
				continue
			}
			ebs, _ := bdm["Ebs"].(map[string]interface{})
			if ebs == nil {
				continue
			}
			if volID, ok := ebs["VolumeId"].(string); ok && volID != "" {
				volumeIDSet[volID] = struct{}{}
				klog.V(4).Infof("DEBUG: EC2 - VM %s uses volume: %s", vmName, volID)
			}
		}
	}

	if len(volumeIDSet) == 0 {
		klog.V(4).Infof("DEBUG: EC2 - No volume IDs found from plan VMs, falling back to all types")
		return f.fetchAllVolumeTypes(ctx, configFlags, inventoryURL, provider, insecureSkipTLS)
	}

	// Fetch volumes inventory to get volume types
	volumesInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "volumes?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EC2 volumes inventory: %v", err)
	}

	volumesInventory = inventory.ExtractEC2Objects(volumesInventory)

	volumesArray, ok := volumesInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for volumes inventory")
	}

	// Match volume IDs to get their types
	volumeTypeSet := make(map[string]struct{})
	for _, item := range volumesArray {
		vol, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		volID, _ := vol["id"].(string)
		if _, needed := volumeIDSet[volID]; !needed {
			continue
		}

		// Volume type can be at top-level or in object
		volumeType := ""
		if vt, ok := vol["volumeType"].(string); ok && vt != "" {
			volumeType = vt
		} else if obj, ok := vol["object"].(map[string]interface{}); ok {
			if vt, ok := obj["VolumeType"].(string); ok && vt != "" {
				volumeType = vt
			}
		}

		if volumeType != "" {
			volumeTypeSet[strings.ToLower(volumeType)] = struct{}{}
			klog.V(4).Infof("DEBUG: EC2 - Volume %s has type: %s", volID, volumeType)
		}
	}

	if len(volumeTypeSet) == 0 {
		klog.V(4).Infof("DEBUG: EC2 - No volume types resolved, falling back to all types")
		return f.fetchAllVolumeTypes(ctx, configFlags, inventoryURL, provider, insecureSkipTLS)
	}

	refs := f.buildSortedRefs(volumeTypeSet)
	klog.V(4).Infof("DEBUG: EC2 - Found %d source storage types from plan VMs", len(refs))
	return refs, nil
}

// buildSortedRefs converts a volume type set to sorted ref.Ref slice with EBS priority ordering.
func (f *EC2StorageFetcher) buildSortedRefs(volumeTypeSet map[string]struct{}) []ref.Ref {
	volumeTypes := make([]string, 0, len(volumeTypeSet))
	for vt := range volumeTypeSet {
		volumeTypes = append(volumeTypes, vt)
	}

	sort.Slice(volumeTypes, func(i, j int) bool {
		priority := map[string]int{
			"gp3": 1, "gp2": 2, "io2": 3, "io1": 4,
			"st1": 5, "sc1": 6, "standard": 7,
		}
		pi, oki := priority[volumeTypes[i]]
		pj, okj := priority[volumeTypes[j]]
		if !oki {
			pi = 99
		}
		if !okj {
			pj = 99
		}
		return pi < pj
	})

	var refs []ref.Ref
	for _, volumeType := range volumeTypes {
		refs = append(refs, ref.Ref{Name: volumeType})
	}

	klog.V(4).Infof("DEBUG: EC2 - Returning %d volume types: %v", len(refs), volumeTypes)
	return refs
}

// FetchTargetStorages fetches target storage from EC2 provider (not typically used as EC2 is usually source)
func (f *EC2StorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("DEBUG: EC2 - Fetching target storage (EC2 is typically not a migration target)")
	// EC2 is typically not used as a migration target, but we implement the interface for completeness
	return []forkliftv1beta1.DestinationStorage{}, nil
}
