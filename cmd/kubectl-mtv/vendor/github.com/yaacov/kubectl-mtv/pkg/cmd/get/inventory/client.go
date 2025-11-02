package inventory

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// ProviderClient provides a unified client for all provider types
type ProviderClient struct {
	configFlags  *genericclioptions.ConfigFlags
	provider     *unstructured.Unstructured
	inventoryURL string
}

// NewProviderClient creates a new provider client
func NewProviderClient(configFlags *genericclioptions.ConfigFlags, provider *unstructured.Unstructured, inventoryURL string) *ProviderClient {
	return &ProviderClient{
		configFlags:  configFlags,
		provider:     provider,
		inventoryURL: inventoryURL,
	}
}

// GetResource fetches a resource from the provider using the specified path
func (pc *ProviderClient) GetResource(resourcePath string) (interface{}, error) {
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
	klog.V(2).Infof("Fetching inventory from provider %s/%s (type=%s, uid=%s) - path: %s, baseURL: %s",
		providerNamespace, providerName, providerType, providerUID, resourcePath, pc.inventoryURL)

	result, err := client.FetchProviderInventory(pc.configFlags, pc.inventoryURL, pc.provider, resourcePath)

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
func (pc *ProviderClient) GetResourceWithQuery(resourcePath, query string) (interface{}, error) {
	if query != "" {
		resourcePath = fmt.Sprintf("%s?%s", resourcePath, query)
	}
	return pc.GetResource(resourcePath)
}

// GetResourceCollection fetches a collection of resources
func (pc *ProviderClient) GetResourceCollection(collection string, detail int) (interface{}, error) {
	return pc.GetResourceWithQuery(collection, fmt.Sprintf("detail=%d", detail))
}

// GetResourceByID fetches a specific resource by ID
func (pc *ProviderClient) GetResourceByID(collection, id string, detail int) (interface{}, error) {
	return pc.GetResourceWithQuery(fmt.Sprintf("%s/%s", collection, id), fmt.Sprintf("detail=%d", detail))
}

// oVirt Provider Resources
func (pc *ProviderClient) GetDataCenters(detail int) (interface{}, error) {
	return pc.GetResourceCollection("datacenters", detail)
}

func (pc *ProviderClient) GetDataCenter(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("datacenters", id, detail)
}

func (pc *ProviderClient) GetClusters(detail int) (interface{}, error) {
	return pc.GetResourceCollection("clusters", detail)
}

func (pc *ProviderClient) GetCluster(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("clusters", id, detail)
}

func (pc *ProviderClient) GetHosts(detail int) (interface{}, error) {
	return pc.GetResourceCollection("hosts", detail)
}

func (pc *ProviderClient) GetHost(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("hosts", id, detail)
}

func (pc *ProviderClient) GetVMs(detail int) (interface{}, error) {
	return pc.GetResourceCollection("vms", detail)
}

func (pc *ProviderClient) GetVM(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("vms", id, detail)
}

func (pc *ProviderClient) GetStorageDomains(detail int) (interface{}, error) {
	return pc.GetResourceCollection("storagedomains", detail)
}

func (pc *ProviderClient) GetStorageDomain(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("storagedomains", id, detail)
}

func (pc *ProviderClient) GetNetworks(detail int) (interface{}, error) {
	return pc.GetResourceCollection("networks", detail)
}

func (pc *ProviderClient) GetNetwork(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("networks", id, detail)
}

func (pc *ProviderClient) GetDisks(detail int) (interface{}, error) {
	return pc.GetResourceCollection("disks", detail)
}

func (pc *ProviderClient) GetDisk(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("disks", id, detail)
}

func (pc *ProviderClient) GetDiskProfiles(detail int) (interface{}, error) {
	return pc.GetResourceCollection("diskprofiles", detail)
}

func (pc *ProviderClient) GetDiskProfile(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("diskprofiles", id, detail)
}

func (pc *ProviderClient) GetNICProfiles(detail int) (interface{}, error) {
	return pc.GetResourceCollection("nicprofiles", detail)
}

func (pc *ProviderClient) GetNICProfile(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("nicprofiles", id, detail)
}

func (pc *ProviderClient) GetWorkloads(detail int) (interface{}, error) {
	return pc.GetResourceCollection("workloads", detail)
}

func (pc *ProviderClient) GetWorkload(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("workloads", id, detail)
}

func (pc *ProviderClient) GetTree() (interface{}, error) {
	return pc.GetResource("tree")
}

func (pc *ProviderClient) GetClusterTree() (interface{}, error) {
	return pc.GetResource("tree/cluster")
}

// vSphere Provider Resources (aliases to generic resources with vSphere context)
func (pc *ProviderClient) GetDatastores(detail int) (interface{}, error) {
	// vSphere datastores map to generic storage resources
	return pc.GetResourceCollection("datastores", detail)
}

func (pc *ProviderClient) GetDatastore(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("datastores", id, detail)
}

func (pc *ProviderClient) GetResourcePools(detail int) (interface{}, error) {
	return pc.GetResourceCollection("resourcepools", detail)
}

func (pc *ProviderClient) GetResourcePool(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("resourcepools", id, detail)
}

func (pc *ProviderClient) GetFolders(detail int) (interface{}, error) {
	return pc.GetResourceCollection("folders", detail)
}

func (pc *ProviderClient) GetFolder(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("folders", id, detail)
}

// OpenStack Provider Resources
func (pc *ProviderClient) GetInstances(detail int) (interface{}, error) {
	// OpenStack instances are equivalent to VMs
	return pc.GetResourceCollection("instances", detail)
}

func (pc *ProviderClient) GetInstance(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("instances", id, detail)
}

func (pc *ProviderClient) GetImages(detail int) (interface{}, error) {
	return pc.GetResourceCollection("images", detail)
}

func (pc *ProviderClient) GetImage(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("images", id, detail)
}

func (pc *ProviderClient) GetFlavors(detail int) (interface{}, error) {
	return pc.GetResourceCollection("flavors", detail)
}

func (pc *ProviderClient) GetFlavor(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("flavors", id, detail)
}

func (pc *ProviderClient) GetSubnets(detail int) (interface{}, error) {
	return pc.GetResourceCollection("subnets", detail)
}

func (pc *ProviderClient) GetSubnet(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("subnets", id, detail)
}

func (pc *ProviderClient) GetPorts(detail int) (interface{}, error) {
	return pc.GetResourceCollection("ports", detail)
}

func (pc *ProviderClient) GetPort(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("ports", id, detail)
}

func (pc *ProviderClient) GetVolumeTypes(detail int) (interface{}, error) {
	return pc.GetResourceCollection("volumetypes", detail)
}

func (pc *ProviderClient) GetVolumeType(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("volumetypes", id, detail)
}

func (pc *ProviderClient) GetVolumes(detail int) (interface{}, error) {
	return pc.GetResourceCollection("volumes", detail)
}

func (pc *ProviderClient) GetVolume(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("volumes", id, detail)
}

func (pc *ProviderClient) GetSecurityGroups(detail int) (interface{}, error) {
	return pc.GetResourceCollection("securitygroups", detail)
}

func (pc *ProviderClient) GetSecurityGroup(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("securitygroups", id, detail)
}

func (pc *ProviderClient) GetFloatingIPs(detail int) (interface{}, error) {
	return pc.GetResourceCollection("floatingips", detail)
}

func (pc *ProviderClient) GetFloatingIP(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("floatingips", id, detail)
}

func (pc *ProviderClient) GetProjects(detail int) (interface{}, error) {
	return pc.GetResourceCollection("projects", detail)
}

func (pc *ProviderClient) GetProject(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("projects", id, detail)
}

func (pc *ProviderClient) GetSnapshots(detail int) (interface{}, error) {
	return pc.GetResourceCollection("snapshots", detail)
}

func (pc *ProviderClient) GetSnapshot(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("snapshots", id, detail)
}

// Kubernetes/OpenShift Provider Resources
func (pc *ProviderClient) GetStorageClasses(detail int) (interface{}, error) {
	return pc.GetResourceCollection("storageclasses", detail)
}

func (pc *ProviderClient) GetStorageClass(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("storageclasses", id, detail)
}

func (pc *ProviderClient) GetPersistentVolumeClaims(detail int) (interface{}, error) {
	return pc.GetResourceCollection("persistentvolumeclaims", detail)
}

func (pc *ProviderClient) GetPersistentVolumeClaim(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("persistentvolumeclaims", id, detail)
}

func (pc *ProviderClient) GetNamespaces(detail int) (interface{}, error) {
	return pc.GetResourceCollection("namespaces", detail)
}

func (pc *ProviderClient) GetNamespace(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("namespaces", id, detail)
}

func (pc *ProviderClient) GetDataVolumes(detail int) (interface{}, error) {
	return pc.GetResourceCollection("datavolumes", detail)
}

func (pc *ProviderClient) GetDataVolume(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("datavolumes", id, detail)
}

// OVA Provider Resources
func (pc *ProviderClient) GetOVAFiles(detail int) (interface{}, error) {
	return pc.GetResourceCollection("ovafiles", detail)
}

func (pc *ProviderClient) GetOVAFile(id string, detail int) (interface{}, error) {
	return pc.GetResourceByID("ovafiles", id, detail)
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
