package inventory

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// ProviderClient provides a unified client for all provider types
type ProviderClient struct {
	configFlags     *genericclioptions.ConfigFlags
	provider        *unstructured.Unstructured
	inventoryURL    string
	insecureSkipTLS bool
}

// NewProviderClientWithInsecure creates a new provider client with optional insecure TLS skip verification
func NewProviderClientWithInsecure(configFlags *genericclioptions.ConfigFlags, provider *unstructured.Unstructured, inventoryURL string, insecureSkipTLS bool) *ProviderClient {
	return &ProviderClient{
		configFlags:     configFlags,
		provider:        provider,
		inventoryURL:    inventoryURL,
		insecureSkipTLS: insecureSkipTLS,
	}
}

// GetResource fetches a resource from the provider using the specified path
func (pc *ProviderClient) GetResource(ctx context.Context, resourcePath string) (interface{}, error) {
	// Get provider info for logging
	providerName := pc.GetProviderName()
	providerNamespace := pc.GetProviderNamespace()
	providerType, _ := pc.GetProviderType()
	providerUID, _ := pc.GetProviderUID()

	// Check if provider has a ready condition
	if err := pc.checkProviderReady(); err != nil {
		return nil, err
	}

	// Log the inventory fetch request
	klog.V(2).Infof("Fetching inventory from provider %s/%s (type=%s, uid=%s) - path: %s, baseURL: %s, insecure=%v",
		providerNamespace, providerName, providerType, providerUID, resourcePath, pc.inventoryURL, pc.insecureSkipTLS)

	result, err := client.FetchProviderInventoryWithInsecure(ctx, pc.configFlags, pc.inventoryURL, pc.provider, resourcePath, pc.insecureSkipTLS)

	if err != nil {
		klog.V(1).Infof("Failed to fetch inventory from provider %s/%s - path: %s, error: %v",
			providerNamespace, providerName, resourcePath, err)
		return nil, err
	}

	// Log success with some response details
	resultType := "unknown"
	resultSize := 0

	switch v := result.(type) {
	case []interface{}:
		resultType = "array"
		resultSize = len(v)
	case map[string]interface{}:
		resultType = "object"
		resultSize = len(v)
	}

	klog.V(2).Infof("Successfully fetched inventory from provider %s/%s - path: %s, result_type: %s, result_size: %d",
		providerNamespace, providerName, resourcePath, resultType, resultSize)

	// Dump the full response at trace level (v=3)
	klog.V(3).Infof("Full inventory response from provider %s/%s - path: %s, response: %+v",
		providerNamespace, providerName, resourcePath, result)

	return result, nil
}

// GetResourceWithQuery fetches a resource with query parameters
func (pc *ProviderClient) GetResourceWithQuery(ctx context.Context, resourcePath, query string) (interface{}, error) {
	if query != "" {
		resourcePath = fmt.Sprintf("%s?%s", resourcePath, query)
	}
	return pc.GetResource(ctx, resourcePath)
}

// GetResourceCollection fetches a collection of resources
func (pc *ProviderClient) GetResourceCollection(ctx context.Context, collection string, detail int) (interface{}, error) {
	return pc.GetResourceWithQuery(ctx, collection, fmt.Sprintf("detail=%d", detail))
}

// GetResourceByID fetches a specific resource by ID
func (pc *ProviderClient) GetResourceByID(ctx context.Context, collection, id string, detail int) (interface{}, error) {
	return pc.GetResourceWithQuery(ctx, fmt.Sprintf("%s/%s", collection, id), fmt.Sprintf("detail=%d", detail))
}

// oVirt Provider Resources
func (pc *ProviderClient) GetDataCenters(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "datacenters", detail)
}

func (pc *ProviderClient) GetDataCenter(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "datacenters", id, detail)
}

func (pc *ProviderClient) GetClusters(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "clusters", detail)
}

func (pc *ProviderClient) GetCluster(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "clusters", id, detail)
}

func (pc *ProviderClient) GetHosts(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "hosts", detail)
}

func (pc *ProviderClient) GetHost(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "hosts", id, detail)
}

func (pc *ProviderClient) GetVMs(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "vms", detail)
}

func (pc *ProviderClient) GetVM(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "vms", id, detail)
}

func (pc *ProviderClient) GetStorageDomains(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "storagedomains", detail)
}

func (pc *ProviderClient) GetStorageDomain(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "storagedomains", id, detail)
}

func (pc *ProviderClient) GetNetworks(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "networks", detail)
}

func (pc *ProviderClient) GetNetwork(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "networks", id, detail)
}

func (pc *ProviderClient) GetDisks(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "disks", detail)
}

func (pc *ProviderClient) GetDisk(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "disks", id, detail)
}

func (pc *ProviderClient) GetDiskProfiles(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "diskprofiles", detail)
}

func (pc *ProviderClient) GetDiskProfile(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "diskprofiles", id, detail)
}

func (pc *ProviderClient) GetNICProfiles(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "nicprofiles", detail)
}

func (pc *ProviderClient) GetNICProfile(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "nicprofiles", id, detail)
}

func (pc *ProviderClient) GetWorkloads(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "workloads", detail)
}

func (pc *ProviderClient) GetWorkload(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "workloads", id, detail)
}

func (pc *ProviderClient) GetTree(ctx context.Context) (interface{}, error) {
	return pc.GetResource(ctx, "tree")
}

func (pc *ProviderClient) GetClusterTree(ctx context.Context) (interface{}, error) {
	return pc.GetResource(ctx, "tree/cluster")
}

// vSphere Provider Resources (aliases to generic resources with vSphere context)
func (pc *ProviderClient) GetDatastores(ctx context.Context, detail int) (interface{}, error) {
	// vSphere datastores map to generic storage resources
	return pc.GetResourceCollection(ctx, "datastores", detail)
}

func (pc *ProviderClient) GetDatastore(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "datastores", id, detail)
}

func (pc *ProviderClient) GetResourcePools(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "resourcepools", detail)
}

func (pc *ProviderClient) GetResourcePool(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "resourcepools", id, detail)
}

func (pc *ProviderClient) GetFolders(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "folders", detail)
}

func (pc *ProviderClient) GetFolder(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "folders", id, detail)
}

// OpenStack Provider Resources
func (pc *ProviderClient) GetInstances(ctx context.Context, detail int) (interface{}, error) {
	// OpenStack instances are equivalent to VMs
	return pc.GetResourceCollection(ctx, "instances", detail)
}

func (pc *ProviderClient) GetInstance(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "instances", id, detail)
}

func (pc *ProviderClient) GetImages(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "images", detail)
}

func (pc *ProviderClient) GetImage(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "images", id, detail)
}

func (pc *ProviderClient) GetFlavors(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "flavors", detail)
}

func (pc *ProviderClient) GetFlavor(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "flavors", id, detail)
}

func (pc *ProviderClient) GetSubnets(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "subnets", detail)
}

func (pc *ProviderClient) GetSubnet(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "subnets", id, detail)
}

func (pc *ProviderClient) GetPorts(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "ports", detail)
}

func (pc *ProviderClient) GetPort(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "ports", id, detail)
}

func (pc *ProviderClient) GetVolumeTypes(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "volumetypes", detail)
}

func (pc *ProviderClient) GetVolumeType(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "volumetypes", id, detail)
}

func (pc *ProviderClient) GetVolumes(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "volumes", detail)
}

func (pc *ProviderClient) GetVolume(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "volumes", id, detail)
}

func (pc *ProviderClient) GetSecurityGroups(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "securitygroups", detail)
}

func (pc *ProviderClient) GetSecurityGroup(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "securitygroups", id, detail)
}

func (pc *ProviderClient) GetFloatingIPs(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "floatingips", detail)
}

func (pc *ProviderClient) GetFloatingIP(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "floatingips", id, detail)
}

func (pc *ProviderClient) GetProjects(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "projects", detail)
}

func (pc *ProviderClient) GetProject(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "projects", id, detail)
}

func (pc *ProviderClient) GetSnapshots(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "snapshots", detail)
}

func (pc *ProviderClient) GetSnapshot(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "snapshots", id, detail)
}

// Kubernetes/OpenShift Provider Resources
func (pc *ProviderClient) GetStorageClasses(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "storageclasses", detail)
}

func (pc *ProviderClient) GetStorageClass(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "storageclasses", id, detail)
}

func (pc *ProviderClient) GetPersistentVolumeClaims(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "persistentvolumeclaims", detail)
}

func (pc *ProviderClient) GetPersistentVolumeClaim(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "persistentvolumeclaims", id, detail)
}

func (pc *ProviderClient) GetNamespaces(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "namespaces", detail)
}

func (pc *ProviderClient) GetNamespace(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "namespaces", id, detail)
}

func (pc *ProviderClient) GetDataVolumes(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "datavolumes", detail)
}

func (pc *ProviderClient) GetDataVolume(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "datavolumes", id, detail)
}

// OVA Provider Resources
func (pc *ProviderClient) GetOVAFiles(ctx context.Context, detail int) (interface{}, error) {
	return pc.GetResourceCollection(ctx, "ovafiles", detail)
}

func (pc *ProviderClient) GetOVAFile(ctx context.Context, id string, detail int) (interface{}, error) {
	return pc.GetResourceByID(ctx, "ovafiles", id, detail)
}

// Generic helper functions for provider-agnostic operations
func (pc *ProviderClient) GetProviderType() (string, error) {
	providerType, found, err := unstructured.NestedString(pc.provider.Object, "spec", "type")
	if err != nil || !found {
		return "", fmt.Errorf("provider type not found or error retrieving it: %v", err)
	}
	return providerType, nil
}

func (pc *ProviderClient) GetProviderUID() (string, error) {
	providerUID, found, err := unstructured.NestedString(pc.provider.Object, "metadata", "uid")
	if err != nil || !found {
		return "", fmt.Errorf("provider UID not found or error retrieving it: %v", err)
	}
	return providerUID, nil
}

func (pc *ProviderClient) GetProviderName() string {
	return pc.provider.GetName()
}

func (pc *ProviderClient) GetProviderNamespace() string {
	return pc.provider.GetNamespace()
}

// checkProviderReady checks if the provider has a ready condition in its status
func (pc *ProviderClient) checkProviderReady() error {
	// Get the status conditions from the provider
	conditions, found, err := unstructured.NestedSlice(pc.provider.Object, "status", "conditions")
	if err != nil {
		return fmt.Errorf("error retrieving provider status conditions: %v", err)
	}

	if !found || len(conditions) == 0 {
		return fmt.Errorf("provider %s/%s does not have ready condition", pc.GetProviderNamespace(), pc.GetProviderName())
	}

	// Look for a "Ready" condition
	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, typeOk := condition["type"].(string)
		conditionStatus, statusOk := condition["status"].(string)

		if typeOk && statusOk && conditionType == "Ready" {
			if conditionStatus == "True" {
				return nil // Provider is ready
			}
			// Ready condition exists but is not True
			reason, _ := condition["reason"].(string)
			message, _ := condition["message"].(string)
			return fmt.Errorf("provider %s/%s is not ready (status: %s, reason: %s, message: %s)",
				pc.GetProviderNamespace(), pc.GetProviderName(), conditionStatus, reason, message)
		}
	}

	// Ready condition not found
	return fmt.Errorf("provider %s/%s does not have ready condition", pc.GetProviderNamespace(), pc.GetProviderName())
}
