package storage

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers"
	ec2Fetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers/ec2"
	openshiftFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers/openshift"
	openstackFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers/openstack"
	ovaFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers/ova"
	ovirtFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers/ovirt"
	vsphereFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/fetchers/vsphere"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper"
	ec2Mapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper/ec2"
	openshiftMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper/openshift"
	openstackMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper/openstack"
	ovaMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper/ova"
	ovirtMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper/ovirt"
	vsphereMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage/mapper/vsphere"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// StorageMapperInterface defines the interface that all provider-specific storage mappers must implement
type StorageMapperInterface interface {
	// GetSourceStorages extracts storage information from the source provider for the specified VMs
	GetSourceStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error)

	// GetTargetStorages extracts available storage information from the target provider
	GetTargetStorages(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationStorage, error)

	// CreateStoragePairs creates storage mapping pairs based on source storages, target storages, and optional default storage class
	CreateStoragePairs(sourceStorages []ref.Ref, targetStorages []forkliftv1beta1.DestinationStorage, defaultTargetStorageClass string) ([]forkliftv1beta1.StoragePair, error)
}

// StorageMapperOptions contains common options for storage mapping
type StorageMapperOptions struct {
	Name                      string
	Namespace                 string
	SourceProvider            string
	SourceProviderNamespace   string
	TargetProvider            string
	TargetProviderNamespace   string
	ConfigFlags               *genericclioptions.ConfigFlags
	InventoryURL              string
	InventoryInsecureSkipTLS  bool
	PlanVMNames               []string
	DefaultTargetStorageClass string
}

// CreateStorageMap creates a storage map using the new fetcher-based architecture
func CreateStorageMap(ctx context.Context, opts StorageMapperOptions) (string, error) {
	klog.V(4).Infof("DEBUG: Creating storage map - Source: %s, Target: %s, DefaultTargetStorageClass: '%s'",
		opts.SourceProvider, opts.TargetProvider, opts.DefaultTargetStorageClass)

	// Get source storage fetcher using the provider's namespace
	sourceProviderNamespace := client.GetProviderNamespace(opts.SourceProviderNamespace, opts.Namespace)
	sourceFetcher, err := GetSourceStorageFetcher(ctx, opts.ConfigFlags, opts.SourceProvider, sourceProviderNamespace, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to get source storage fetcher: %v", err)
	}
	klog.V(4).Infof("DEBUG: Source storage fetcher created for provider: %s", opts.SourceProvider)

	// Get target storage fetcher using the provider's namespace
	targetProviderNamespace := client.GetProviderNamespace(opts.TargetProviderNamespace, opts.Namespace)
	targetFetcher, err := GetTargetStorageFetcher(ctx, opts.ConfigFlags, opts.TargetProvider, targetProviderNamespace, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to get target storage fetcher: %v", err)
	}
	klog.V(4).Infof("DEBUG: Target storage fetcher created for provider: %s", opts.TargetProvider)

	// Fetch source storages
	sourceStorages, err := sourceFetcher.FetchSourceStorages(ctx, opts.ConfigFlags, opts.SourceProvider, sourceProviderNamespace, opts.InventoryURL, opts.PlanVMNames, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to fetch source storages: %v", err)
	}
	klog.V(4).Infof("DEBUG: Fetched %d source storages", len(sourceStorages))

	// Fetch target storages
	var targetStorages []forkliftv1beta1.DestinationStorage
	if opts.DefaultTargetStorageClass == "" {
		klog.V(4).Infof("DEBUG: Fetching target storages from target provider: %s", opts.TargetProvider)
		targetStorages, err = targetFetcher.FetchTargetStorages(ctx, opts.ConfigFlags, opts.TargetProvider, targetProviderNamespace, opts.InventoryURL, opts.InventoryInsecureSkipTLS)
		if err != nil {
			return "", fmt.Errorf("failed to fetch target storages: %v", err)
		}
		klog.V(4).Infof("DEBUG: Fetched %d target storages", len(targetStorages))
	} else {
		klog.V(4).Infof("DEBUG: Skipping target storage fetch due to DefaultTargetStorageClass='%s'", opts.DefaultTargetStorageClass)
	}

	// Get provider-specific storage mapper
	storageMapper, sourceProviderType, targetProviderType, err := GetStorageMapper(ctx, opts.ConfigFlags, opts.SourceProvider, sourceProviderNamespace, opts.TargetProvider, targetProviderNamespace, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to get storage mapper: %v", err)
	}

	// Create storage pairs using provider-specific mapping logic
	mappingOpts := mapper.StorageMappingOptions{
		DefaultTargetStorageClass: opts.DefaultTargetStorageClass,
		SourceProviderType:        sourceProviderType,
		TargetProviderType:        targetProviderType,
	}
	storagePairs, err := storageMapper.CreateStoragePairs(sourceStorages, targetStorages, mappingOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create storage pairs: %v", err)
	}

	// Create the storage map using the existing infrastructure
	return createStorageMap(opts, storagePairs)
}

// createStorageMap helper function to create the actual storage map resource
func createStorageMap(opts StorageMapperOptions, storagePairs []forkliftv1beta1.StoragePair) (string, error) {
	// If no storage pairs, create a dummy pair
	if len(storagePairs) == 0 {
		klog.V(4).Infof("DEBUG: No storage pairs found, creating dummy pair")
		storagePairs = []forkliftv1beta1.StoragePair{
			{
				Source: ref.Ref{
					Type: "default", // Use "default" type for dummy entry
				},
				Destination: forkliftv1beta1.DestinationStorage{
					// Empty StorageClass means system default
				},
			},
		}
	}

	// Create the storage map name
	storageMapName := opts.Name + "-storage-map"

	// Create StorageMap object
	storageMap := &forkliftv1beta1.StorageMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      storageMapName,
			Namespace: opts.Namespace,
		},
		Spec: forkliftv1beta1.StorageMapSpec{
			Provider: provider.Pair{
				Source: corev1.ObjectReference{
					Kind:       "Provider",
					APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
					Name:       opts.SourceProvider,
					Namespace:  client.GetProviderNamespace(opts.SourceProviderNamespace, opts.Namespace),
				},
				Destination: corev1.ObjectReference{
					Kind:       "Provider",
					APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
					Name:       opts.TargetProvider,
					Namespace:  client.GetProviderNamespace(opts.TargetProviderNamespace, opts.Namespace),
				},
			},
			Map: storagePairs,
		},
	}
	storageMap.Kind = "StorageMap"
	storageMap.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	// Convert to Unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(storageMap)
	if err != nil {
		return "", fmt.Errorf("failed to convert StorageMap to Unstructured: %v", err)
	}

	storageMapUnstructured := &unstructured.Unstructured{Object: unstructuredMap}

	// Create the storage map
	c, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return "", fmt.Errorf("failed to get client: %v", err)
	}

	_, err = c.Resource(client.StorageMapGVR).Namespace(opts.Namespace).Create(context.TODO(), storageMapUnstructured, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create storage map: %v", err)
	}

	klog.V(4).Infof("DEBUG: Created storage map '%s' with %d storage pairs", storageMapName, len(storagePairs))
	return storageMapName, nil
}

// GetSourceStorageFetcher returns the appropriate source storage fetcher based on provider type
func GetSourceStorageFetcher(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace string, insecureSkipTLS bool) (fetchers.SourceStorageFetcher, error) {
	// Get the provider object to determine its type
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %v", err)
	}

	// Create a provider client to get the provider type
	// Note: GetProviderType() only reads from CRD spec (no HTTPS calls), but we pass insecureSkipTLS for consistency
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, "", insecureSkipTLS)
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	klog.V(4).Infof("DEBUG: GetSourceStorageFetcher - Provider: %s, Type: %s", providerName, providerType)

	// Return the appropriate fetcher based on provider type
	switch providerType {
	case "ec2":
		klog.V(4).Infof("DEBUG: Using EC2 source storage fetcher for %s", providerName)
		return ec2Fetcher.NewEC2StorageFetcher(), nil
	case "openstack":
		klog.V(4).Infof("DEBUG: Using OpenStack source storage fetcher for %s", providerName)
		return openstackFetcher.NewOpenStackStorageFetcher(), nil
	case "vsphere":
		klog.V(4).Infof("DEBUG: Using VSphere source storage fetcher for %s", providerName)
		return vsphereFetcher.NewVSphereStorageFetcher(), nil
	case "openshift":
		klog.V(4).Infof("DEBUG: Using OpenShift source storage fetcher for %s", providerName)
		return openshiftFetcher.NewOpenShiftStorageFetcher(), nil
	case "ova":
		klog.V(4).Infof("DEBUG: Using OVA source storage fetcher for %s", providerName)
		return ovaFetcher.NewOVAStorageFetcher(), nil
	case "ovirt":
		klog.V(4).Infof("DEBUG: Using oVirt source storage fetcher for %s", providerName)
		return ovirtFetcher.NewOvirtStorageFetcher(), nil
	default:
		return nil, fmt.Errorf("unsupported source provider type: %s", providerType)
	}
}

// GetTargetStorageFetcher returns the appropriate target storage fetcher based on provider type
func GetTargetStorageFetcher(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace string, insecureSkipTLS bool) (fetchers.TargetStorageFetcher, error) {
	// Get the provider object to determine its type
	provider, err := inventory.GetProviderByName(ctx, configFlags, providerName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %v", err)
	}

	// Create a provider client to get the provider type
	// Note: GetProviderType() only reads from CRD spec (no HTTPS calls), but we pass insecureSkipTLS for consistency
	providerClient := inventory.NewProviderClientWithInsecure(configFlags, provider, "", insecureSkipTLS)
	providerType, err := providerClient.GetProviderType()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider type: %v", err)
	}

	klog.V(4).Infof("DEBUG: GetTargetStorageFetcher - Provider: %s, Type: %s", providerName, providerType)

	// Return the appropriate fetcher based on provider type
	switch providerType {
	case "ec2":
		// Note: EC2 is typically used as a source provider for migrations to OpenShift/Kubernetes.
		// EC2 as a migration target is not a common use case, but we provide the fetcher for interface completeness.
		klog.V(4).Infof("DEBUG: Using EC2 target storage fetcher for %s (note: EC2 is typically a source, not target)", providerName)
		return ec2Fetcher.NewEC2StorageFetcher(), nil
	case "openstack":
		klog.V(4).Infof("DEBUG: Using OpenStack target storage fetcher for %s", providerName)
		return openstackFetcher.NewOpenStackStorageFetcher(), nil
	case "vsphere":
		klog.V(4).Infof("DEBUG: Using VSphere target storage fetcher for %s", providerName)
		return vsphereFetcher.NewVSphereStorageFetcher(), nil
	case "openshift":
		klog.V(4).Infof("DEBUG: Using OpenShift target storage fetcher for %s", providerName)
		return openshiftFetcher.NewOpenShiftStorageFetcher(), nil
	case "ova":
		klog.V(4).Infof("DEBUG: Using OVA target storage fetcher for %s", providerName)
		return ovaFetcher.NewOVAStorageFetcher(), nil
	case "ovirt":
		klog.V(4).Infof("DEBUG: Using oVirt target storage fetcher for %s", providerName)
		return ovirtFetcher.NewOvirtStorageFetcher(), nil
	default:
		return nil, fmt.Errorf("unsupported target provider type: %s", providerType)
	}
}

// GetStorageMapper returns the appropriate storage mapper based on source provider type
func GetStorageMapper(ctx context.Context, configFlags *genericclioptions.ConfigFlags, sourceProviderName, sourceProviderNamespace, targetProviderName, targetProviderNamespace string, insecureSkipTLS bool) (mapper.StorageMapper, string, string, error) {
	// Get source provider type
	sourceProvider, err := inventory.GetProviderByName(ctx, configFlags, sourceProviderName, sourceProviderNamespace)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get source provider: %v", err)
	}

	// Note: GetProviderType() only reads from CRD spec (no HTTPS calls), but we pass insecureSkipTLS for consistency
	sourceProviderClient := inventory.NewProviderClientWithInsecure(configFlags, sourceProvider, "", insecureSkipTLS)
	sourceProviderType, err := sourceProviderClient.GetProviderType()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get source provider type: %v", err)
	}

	// Get target provider type
	targetProvider, err := inventory.GetProviderByName(ctx, configFlags, targetProviderName, targetProviderNamespace)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get target provider: %v", err)
	}

	// Note: GetProviderType() only reads from CRD spec (no HTTPS calls), but we pass insecureSkipTLS for consistency
	targetProviderClient := inventory.NewProviderClientWithInsecure(configFlags, targetProvider, "", insecureSkipTLS)
	targetProviderType, err := targetProviderClient.GetProviderType()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get target provider type: %v", err)
	}

	klog.V(4).Infof("DEBUG: GetStorageMapper - Source provider: %s (type: %s), Target provider: %s (type: %s)",
		sourceProviderName, sourceProviderType, targetProviderName, targetProviderType)

	// Return the appropriate mapper based on source provider type
	switch sourceProviderType {
	case "ec2":
		klog.V(4).Infof("DEBUG: Using EC2 storage mapper for source %s", sourceProviderName)
		return ec2Mapper.NewEC2StorageMapper(), sourceProviderType, targetProviderType, nil
	case "openstack":
		klog.V(4).Infof("DEBUG: Using OpenStack storage mapper for source %s", sourceProviderName)
		return openstackMapper.NewOpenStackStorageMapper(), sourceProviderType, targetProviderType, nil
	case "vsphere":
		klog.V(4).Infof("DEBUG: Using vSphere storage mapper for source %s", sourceProviderName)
		return vsphereMapper.NewVSphereStorageMapper(), sourceProviderType, targetProviderType, nil
	case "openshift":
		klog.V(4).Infof("DEBUG: Using OpenShift storage mapper for source %s", sourceProviderName)
		return openshiftMapper.NewOpenShiftStorageMapper(), sourceProviderType, targetProviderType, nil
	case "ova":
		klog.V(4).Infof("DEBUG: Using OVA storage mapper for source %s", sourceProviderName)
		return ovaMapper.NewOVAStorageMapper(), sourceProviderType, targetProviderType, nil
	case "ovirt":
		klog.V(4).Infof("DEBUG: Using oVirt storage mapper for source %s", sourceProviderName)
		return ovirtMapper.NewOvirtStorageMapper(), sourceProviderType, targetProviderType, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported source provider type: %s", sourceProviderType)
	}
}
