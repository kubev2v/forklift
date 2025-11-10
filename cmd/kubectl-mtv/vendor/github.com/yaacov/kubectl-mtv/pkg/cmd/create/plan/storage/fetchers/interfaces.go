package fetchers

import (
	"context"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// SourceStorageFetcher interface for extracting storage information from source VMs
type SourceStorageFetcher interface {
	// FetchSourceStorages extracts storage references from VMs to be migrated
	FetchSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string) ([]ref.Ref, error)
}

// TargetStorageFetcher interface for extracting available target storage
type TargetStorageFetcher interface {
	// FetchTargetStorages extracts available destination storage from target provider
	FetchTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string) ([]forkliftv1beta1.DestinationStorage, error)
}

// StorageFetcher combines both source and target fetching for providers that can act as both
type StorageFetcher interface {
	SourceStorageFetcher
	TargetStorageFetcher
}
