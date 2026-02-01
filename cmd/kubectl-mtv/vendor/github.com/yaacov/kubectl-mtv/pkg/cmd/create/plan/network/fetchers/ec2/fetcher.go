package ec2

import (
	"context"
	"fmt"
	"sort"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// EC2NetworkFetcher implements network fetching for EC2 providers
type EC2NetworkFetcher struct{}

// NewEC2NetworkFetcher creates a new EC2 network fetcher
func NewEC2NetworkFetcher() fetchers.NetworkFetcher {
	return &EC2NetworkFetcher{}
}

// FetchSourceNetworks fetches networks (VPCs and Subnets) from EC2 provider
func (f *EC2NetworkFetcher) FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, _ []string, insecureSkipTLS bool) ([]ref.Ref, error) {
	klog.V(4).Infof("DEBUG: EC2 - Fetching source networks from provider: %s", providerName)

	// Get provider
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get EC2 provider: %v", err)
	}

	// Fetch EC2 networks (VPCs and Subnets)
	networksInventory, err := client.FetchProviderInventoryWithInsecure(ctx, configFlags, inventoryURL, provider, "networks?detail=4", insecureSkipTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch EC2 networks inventory: %v", err)
	}

	// Extract objects from EC2 envelope
	networksInventory = inventory.ExtractEC2Objects(networksInventory)

	networksArray, ok := networksInventory.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected array for networks inventory")
	}

	// Separate VPCs and Subnets
	var sourceNetworks []ref.Ref
	subnets := make([]map[string]interface{}, 0)
	vpcs := make([]map[string]interface{}, 0)

	for _, item := range networksArray {
		network, ok := item.(map[string]interface{})
		if !ok {
			klog.V(4).Infof("DEBUG: EC2 - Skipping network item with unexpected type: %T", item)
			continue
		}

		// Check if it's a subnet (has non-empty SubnetId) or VPC (non-empty VpcId, no subnet)
		if subnetID, ok := network["SubnetId"].(string); ok && subnetID != "" {
			subnets = append(subnets, network)
		} else if vpcID, ok := network["VpcId"].(string); ok && vpcID != "" {
			vpcs = append(vpcs, network)
		}
	}

	// If we have subnets, return them sorted by CIDR
	if len(subnets) > 0 {
		sort.Slice(subnets, func(i, j int) bool {
			cidrI, okI := subnets[i]["CidrBlock"].(string)
			cidrJ, okJ := subnets[j]["CidrBlock"].(string)
			if !okI || !okJ {
				klog.V(4).Infof("DEBUG: EC2 - Missing or invalid CidrBlock during subnet sort (i:%v, j:%v)", okI, okJ)
			}
			return cidrI < cidrJ
		})

		for _, subnet := range subnets {
			// Use id from top level (provided by inventory server)
			if subnetID, ok := subnet["id"].(string); ok {
				sourceNetworks = append(sourceNetworks, ref.Ref{
					ID: subnetID,
				})
			}
		}
	} else if len(vpcs) > 0 {
		// If no subnets, return VPCs sorted by ID
		sort.Slice(vpcs, func(i, j int) bool {
			vpcI, okI := vpcs[i]["id"].(string)
			vpcJ, okJ := vpcs[j]["id"].(string)
			if !okI || !okJ {
				klog.V(4).Infof("DEBUG: EC2 - Missing or invalid id during VPC sort (i:%v, j:%v)", okI, okJ)
			}
			return vpcI < vpcJ
		})

		for _, vpc := range vpcs {
			// Use id from top level (provided by inventory server)
			if vpcID, ok := vpc["id"].(string); ok {
				sourceNetworks = append(sourceNetworks, ref.Ref{
					ID: vpcID,
				})
			}
		}
	}

	klog.V(4).Infof("DEBUG: EC2 - Found %d source networks (%d subnets, %d VPCs)", len(sourceNetworks), len(subnets), len(vpcs))
	return sourceNetworks, nil
}

// FetchTargetNetworks fetches target networks from EC2 provider (not typically used as EC2 is usually source)
func (f *EC2NetworkFetcher) FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationNetwork, error) {
	klog.V(4).Infof("DEBUG: EC2 - Fetching target networks (EC2 is typically not a migration target)")
	// EC2 is typically not used as a migration target, but we implement the interface for completeness
	return []forkliftv1beta1.DestinationNetwork{}, nil
}
