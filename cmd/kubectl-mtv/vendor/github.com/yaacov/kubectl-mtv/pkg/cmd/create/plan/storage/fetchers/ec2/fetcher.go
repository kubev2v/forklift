package ec2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
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

// FetchSourceStorages fetches EBS volume types from EC2 provider
func (f *EC2StorageFetcher) FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: EC2 - Fetching source storage types from provider: %s", providerName)

	// Get provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get EC2 provider: %v", err)
	}

	// Fetch EC2 storage types (EBS volume types)
	storageInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "storages?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EC2 storage inventory: %v", err)
	}

	// Extract objects from EC2 envelope
	storageInventory = inventory.ExtractEC2Objects(storageInventory)

	storageArray, ok := storageInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for storage inventory")
	}

	// Extract unique EBS volume types using a set for deduplication
	var sourceStorages []ref.Ref
	volumeTypeSet := make(map[string]struct{})

	for _, item := range storageArray {
		storage, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Get the EC2 volume type (e.g., "gp3", "io2", "st1")
		// Normalize to lowercase to handle inventory variations (GP3, Gp3, gp3)
		volumeType, ok := storage["type"].(string)
		if ok && volumeType != "" {
			volumeType = strings.ToLower(volumeType)
			volumeTypeSet[volumeType] = struct{}{}
		}
	}

	// Convert set to slice
	volumeTypes := make([]string, 0, len(volumeTypeSet))
	for vt := range volumeTypeSet {
		volumeTypes = append(volumeTypes, vt)
	}

	// Sort volume types for consistent ordering (prioritize SSD types)
	sort.Slice(volumeTypes, func(i, j int) bool {
		// Priority order: gp3, gp2, io2, io1, st1, sc1, standard
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

	// Create refs for each volume type
	for _, volumeType := range volumeTypes {
		sourceStorages = append(sourceStorages, ref.Ref{
			Name: volumeType,
		})
	}

	klog.V(4).Infof("DEBUG: EC2 - Found %d source storage types: %v", len(sourceStorages), volumeTypes)
	return sourceStorages, nil
}

// FetchTargetStorages fetches target storage from EC2 provider (not typically used as EC2 is usually source)
func (f *EC2StorageFetcher) FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error) {
	klog.V(4).Infof("DEBUG: EC2 - Fetching target storage (EC2 is typically not a migration target)")
	// EC2 is typically not used as a migration target, but we implement the interface for completeness
	return []forkliftv1beta1.DestinationStorage{}, nil
}
