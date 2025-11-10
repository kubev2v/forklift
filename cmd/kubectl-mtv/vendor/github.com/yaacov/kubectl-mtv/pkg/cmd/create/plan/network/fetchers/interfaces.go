package fetchers

import (
	"context"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// SourceNetworkFetcher interface for extracting network information from source VMs
type SourceNetworkFetcher interface {
	// FetchSourceNetworks extracts network references from VMs to be migrated
	FetchSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error)
}

// TargetNetworkFetcher interface for extracting available target networks
type TargetNetworkFetcher interface {
	// FetchTargetNetworks extracts available destination networks from target provider
	FetchTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationNetwork, error)
}

// NetworkFetcher combines both source and target fetching for providers that can act as both
type NetworkFetcher interface {
	SourceNetworkFetcher
	TargetNetworkFetcher
}
