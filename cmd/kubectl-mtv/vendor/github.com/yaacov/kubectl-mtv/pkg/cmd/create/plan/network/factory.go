package network

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

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers"
	ec2Fetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers/ec2"
	openshiftFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers/openshift"
	openstackFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers/openstack"
	ovaFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers/ova"
	ovirtFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers/ovirt"
	vsphereFetcher "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/fetchers/vsphere"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper"
	ec2Mapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper/ec2"
	openshiftMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper/openshift"
	openstackMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper/openstack"
	ovaMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper/ova"
	ovirtMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper/ovirt"
	vsphereMapper "github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper/vsphere"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// NetworkMapperInterface defines the interface for network mapping operations
type NetworkMapperInterface interface {
	// GetSourceNetworks extracts network information from the source provider for the specified VMs
	GetSourceNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, planVMNames []string, insecureSkipTLS bool) ([]ref.Ref, error)

	// GetTargetNetworks extracts available network information from the target provider
	GetTargetNetworks(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace, inventoryURL string, insecureSkipTLS bool) ([]forkliftv1beta1.DestinationNetwork, error)

	// CreateNetworkPairs creates network mapping pairs based on source networks, target networks, and optional default network
	CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, defaultTargetNetwork string, namespace string) ([]forkliftv1beta1.NetworkPair, error)
}

// NetworkMapperOptions contains common options for network mapping
type NetworkMapperOptions struct {
	Name                     string
	Namespace                string
	TargetNamespace          string // Where VMs will be created (plan.spec.targetNamespace)
	SourceProvider           string
	SourceProviderNamespace  string
	TargetProvider           string
	TargetProviderNamespace  string
	ConfigFlags              *genericclioptions.ConfigFlags
	InventoryURL             string
	InventoryInsecureSkipTLS bool
	PlanVMNames              []string
	DefaultTargetNetwork     string
}

// GetSourceNetworkFetcher returns the appropriate source network fetcher based on provider type
func GetSourceNetworkFetcher(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace string, insecureSkipTLS bool) (fetchers.SourceNetworkFetcher, error) {
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

	klog.V(4).Infof("DEBUG: GetSourceNetworkFetcher - Provider: %s, Type: %s", providerName, providerType)

	// Return the appropriate fetcher based on provider type
	switch providerType {
	case "ec2":
		klog.V(4).Infof("DEBUG: Using EC2 source network fetcher for %s", providerName)
		return ec2Fetcher.NewEC2NetworkFetcher(), nil
	case "openstack":
		klog.V(4).Infof("DEBUG: Using OpenStack source network fetcher for %s", providerName)
		return openstackFetcher.NewOpenStackNetworkFetcher(), nil
	case "vsphere":
		klog.V(4).Infof("DEBUG: Using VSphere source network fetcher for %s", providerName)
		return vsphereFetcher.NewVSphereNetworkFetcher(), nil
	case "openshift":
		klog.V(4).Infof("DEBUG: Using OpenShift source network fetcher for %s", providerName)
		return openshiftFetcher.NewOpenShiftNetworkFetcher(), nil
	case "ova":
		klog.V(4).Infof("DEBUG: Using OVA source network fetcher for %s", providerName)
		return ovaFetcher.NewOVANetworkFetcher(), nil
	case "ovirt":
		klog.V(4).Infof("DEBUG: Using oVirt source network fetcher for %s", providerName)
		return ovirtFetcher.NewOvirtNetworkFetcher(), nil
	default:
		return nil, fmt.Errorf("unsupported source provider type: %s", providerType)
	}
}

// GetTargetNetworkFetcher returns the appropriate target network fetcher based on provider type
func GetTargetNetworkFetcher(ctx context.Context, configFlags *genericclioptions.ConfigFlags, providerName, namespace string, insecureSkipTLS bool) (fetchers.TargetNetworkFetcher, error) {
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

	klog.V(4).Infof("DEBUG: GetTargetNetworkFetcher - Provider: %s, Type: %s", providerName, providerType)

	// Return the appropriate fetcher based on provider type
	switch providerType {
	case "openstack":
		klog.V(4).Infof("DEBUG: Using OpenStack target network fetcher for %s", providerName)
		return openstackFetcher.NewOpenStackNetworkFetcher(), nil
	case "vsphere":
		klog.V(4).Infof("DEBUG: Using VSphere target network fetcher for %s", providerName)
		return vsphereFetcher.NewVSphereNetworkFetcher(), nil
	case "openshift":
		klog.V(4).Infof("DEBUG: Using OpenShift target network fetcher for %s", providerName)
		return openshiftFetcher.NewOpenShiftNetworkFetcher(), nil
	case "ova":
		klog.V(4).Infof("DEBUG: Using OVA target network fetcher for %s", providerName)
		return ovaFetcher.NewOVANetworkFetcher(), nil
	case "ovirt":
		klog.V(4).Infof("DEBUG: Using oVirt target network fetcher for %s", providerName)
		return ovirtFetcher.NewOvirtNetworkFetcher(), nil
	default:
		return nil, fmt.Errorf("unsupported target provider type: %s", providerType)
	}
}

// GetNetworkMapper returns the appropriate network mapper based on source provider type
func GetNetworkMapper(ctx context.Context, configFlags *genericclioptions.ConfigFlags, sourceProviderName, sourceProviderNamespace, targetProviderName, targetProviderNamespace string, insecureSkipTLS bool) (mapper.NetworkMapper, string, string, error) {
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

	targetProviderClient := inventory.NewProviderClientWithInsecure(configFlags, targetProvider, "", insecureSkipTLS)
	targetProviderType, err := targetProviderClient.GetProviderType()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get target provider type: %v", err)
	}

	klog.V(4).Infof("DEBUG: GetNetworkMapper - Source provider: %s (type: %s), Target provider: %s (type: %s)",
		sourceProviderName, sourceProviderType, targetProviderName, targetProviderType)

	// Return the appropriate mapper based on source provider type
	switch sourceProviderType {
	case "ec2":
		klog.V(4).Infof("DEBUG: Using EC2 network mapper for source %s", sourceProviderName)
		return ec2Mapper.NewEC2NetworkMapper(), sourceProviderType, targetProviderType, nil
	case "openstack":
		klog.V(4).Infof("DEBUG: Using OpenStack network mapper for source %s", sourceProviderName)
		return openstackMapper.NewOpenStackNetworkMapper(), sourceProviderType, targetProviderType, nil
	case "vsphere":
		klog.V(4).Infof("DEBUG: Using vSphere network mapper for source %s", sourceProviderName)
		return vsphereMapper.NewVSphereNetworkMapper(), sourceProviderType, targetProviderType, nil
	case "openshift":
		klog.V(4).Infof("DEBUG: Using OpenShift network mapper for source %s", sourceProviderName)
		return openshiftMapper.NewOpenShiftNetworkMapper(), sourceProviderType, targetProviderType, nil
	case "ova":
		klog.V(4).Infof("DEBUG: Using OVA network mapper for source %s", sourceProviderName)
		return ovaMapper.NewOVANetworkMapper(), sourceProviderType, targetProviderType, nil
	case "ovirt":
		klog.V(4).Infof("DEBUG: Using oVirt network mapper for source %s", sourceProviderName)
		return ovirtMapper.NewOvirtNetworkMapper(), sourceProviderType, targetProviderType, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported source provider type: %s", sourceProviderType)
	}
}

// CreateNetworkMap creates a network map using the new fetcher-based architecture
func CreateNetworkMap(ctx context.Context, opts NetworkMapperOptions) (string, error) {
	klog.V(4).Infof("DEBUG: Creating network map - Source: %s, Target: %s, DefaultTargetNetwork: '%s'",
		opts.SourceProvider, opts.TargetProvider, opts.DefaultTargetNetwork)

	// Get source network fetcher using the provider's namespace
	sourceProviderNamespace := client.GetProviderNamespace(opts.SourceProviderNamespace, opts.Namespace)
	sourceFetcher, err := GetSourceNetworkFetcher(ctx, opts.ConfigFlags, opts.SourceProvider, sourceProviderNamespace, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to get source network fetcher: %v", err)
	}
	klog.V(4).Infof("DEBUG: Source fetcher created for provider: %s", opts.SourceProvider)

	// Get target network fetcher using the provider's namespace
	targetProviderNamespace := client.GetProviderNamespace(opts.TargetProviderNamespace, opts.Namespace)
	targetFetcher, err := GetTargetNetworkFetcher(ctx, opts.ConfigFlags, opts.TargetProvider, targetProviderNamespace, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to get target network fetcher: %v", err)
	}
	klog.V(4).Infof("DEBUG: Target fetcher created for provider: %s", opts.TargetProvider)

	// Fetch source networks
	sourceNetworks, err := sourceFetcher.FetchSourceNetworks(ctx, opts.ConfigFlags, opts.SourceProvider, sourceProviderNamespace, opts.InventoryURL, opts.PlanVMNames, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to fetch source networks: %v", err)
	}
	klog.V(4).Infof("DEBUG: Fetched %d source networks", len(sourceNetworks))

	// Fetch target networks
	var targetNetworks []forkliftv1beta1.DestinationNetwork
	if opts.DefaultTargetNetwork == "" || (opts.DefaultTargetNetwork != "default" && opts.DefaultTargetNetwork != "") {
		klog.V(4).Infof("DEBUG: Fetching target networks from target provider: %s", opts.TargetProvider)
		targetNetworks, err = targetFetcher.FetchTargetNetworks(ctx, opts.ConfigFlags, opts.TargetProvider, targetProviderNamespace, opts.InventoryURL, opts.InventoryInsecureSkipTLS)
		if err != nil {
			return "", fmt.Errorf("failed to fetch target networks: %v", err)
		}
		klog.V(4).Infof("DEBUG: Fetched %d target networks", len(targetNetworks))

		// Filter target networks to only include NADs in target namespace or "default" namespace
		targetNetworks = filterTargetNetworksByNamespace(targetNetworks, opts.TargetNamespace)
		klog.V(4).Infof("DEBUG: After filtering: %d target networks in namespace %s or default", len(targetNetworks), opts.TargetNamespace)
	} else {
		klog.V(4).Infof("DEBUG: Skipping target network fetch due to DefaultTargetNetwork='%s'", opts.DefaultTargetNetwork)
	}

	// Get provider-specific network mapper
	networkMapper, sourceProviderType, targetProviderType, err := GetNetworkMapper(ctx, opts.ConfigFlags, opts.SourceProvider, sourceProviderNamespace, opts.TargetProvider, targetProviderNamespace, opts.InventoryInsecureSkipTLS)
	if err != nil {
		return "", fmt.Errorf("failed to get network mapper: %v", err)
	}

	// Create network pairs using provider-specific mapping logic
	mappingOpts := mapper.NetworkMappingOptions{
		DefaultTargetNetwork: opts.DefaultTargetNetwork,
		Namespace:            opts.Namespace,
		SourceProviderType:   sourceProviderType,
		TargetProviderType:   targetProviderType,
	}
	networkPairs, err := networkMapper.CreateNetworkPairs(sourceNetworks, targetNetworks, mappingOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create network pairs: %v", err)
	}

	// Create the network map using the existing infrastructure
	return createNetworkMap(opts, networkPairs)
}

// createNetworkMap helper function to create the actual network map resource
func createNetworkMap(opts NetworkMapperOptions, networkPairs []forkliftv1beta1.NetworkPair) (string, error) {
	// If no network pairs, create a dummy pair
	if len(networkPairs) == 0 {
		klog.V(4).Infof("DEBUG: No network pairs found, creating dummy pair")
		networkPairs = []forkliftv1beta1.NetworkPair{
			{
				Source: ref.Ref{
					Type: "pod", // Use "pod" type for dummy entry
				},
				Destination: forkliftv1beta1.DestinationNetwork{
					Type: "pod", // Use pod networking as default
				},
			},
		}
	}

	// Create the network map name
	networkMapName := opts.Name + "-network-map"

	// Create NetworkMap object
	networkMap := &forkliftv1beta1.NetworkMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      networkMapName,
			Namespace: opts.Namespace,
		},
		Spec: forkliftv1beta1.NetworkMapSpec{
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
			Map: networkPairs,
		},
	}
	networkMap.Kind = "NetworkMap"
	networkMap.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	// Convert to Unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(networkMap)
	if err != nil {
		return "", fmt.Errorf("failed to convert NetworkMap to Unstructured: %v", err)
	}

	networkMapUnstructured := &unstructured.Unstructured{Object: unstructuredMap}

	// Create the network map
	c, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return "", fmt.Errorf("failed to get client: %v", err)
	}

	_, err = c.Resource(client.NetworkMapGVR).Namespace(opts.Namespace).Create(context.TODO(), networkMapUnstructured, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create network map: %v", err)
	}

	klog.V(4).Infof("DEBUG: Created network map '%s' with %d network pairs", networkMapName, len(networkPairs))
	return networkMapName, nil
}

// filterTargetNetworksByNamespace filters target networks to only include NADs
// that are in the target namespace or the "default" namespace.
// This ensures VMs can only use NADs that are accessible from their namespace.
func filterTargetNetworksByNamespace(networks []forkliftv1beta1.DestinationNetwork, targetNamespace string) []forkliftv1beta1.DestinationNetwork {
	var filtered []forkliftv1beta1.DestinationNetwork
	for _, network := range networks {
		if network.Type != "multus" {
			// Keep non-multus networks (pod networking, etc.)
			filtered = append(filtered, network)
			continue
		}
		// Only keep multus networks in target namespace or "default" namespace
		if network.Namespace == targetNamespace || network.Namespace == "default" {
			filtered = append(filtered, network)
			klog.V(4).Infof("DEBUG: Keeping NAD %s/%s (matches target namespace or default)", network.Namespace, network.Name)
		} else {
			klog.V(4).Infof("DEBUG: Filtering out NAD %s/%s (not in target namespace %s or default)", network.Namespace, network.Name, targetNamespace)
		}
	}
	return filtered
}
