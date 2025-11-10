package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	mapping "github.com/yaacov/kubectl-mtv/pkg/cmd/create/mapping"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/storage"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/provider/defaultprovider"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/get/inventory"
	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// CreatePlanOptions encapsulates the parameters for the Create function.
type CreatePlanOptions struct {
	Name                      string
	Namespace                 string
	SourceProvider            string
	TargetProvider            string
	SourceProviderNamespace   string // parsed from SourceProvider if it contains namespace/name pattern
	TargetProviderNamespace   string // parsed from TargetProvider if it contains namespace/name pattern
	NetworkMapping            string
	StorageMapping            string
	InventoryURL              string
	DefaultTargetNetwork      string
	DefaultTargetStorageClass string
	PlanSpec                  forkliftv1beta1.PlanSpec
	ConfigFlags               *genericclioptions.ConfigFlags
	NetworkPairs              string
	StoragePairs              string

	// Storage enhancement options
	DefaultVolumeMode    string
	DefaultAccessMode    string
	DefaultOffloadPlugin string
	DefaultOffloadSecret string
	DefaultOffloadVendor string

	// Offload secret creation fields
	OffloadVSphereUsername string
	OffloadVSpherePassword string
	OffloadVSphereURL      string
	OffloadStorageUsername string
	OffloadStoragePassword string
	OffloadStorageEndpoint string
	OffloadCACert          string
	OffloadInsecureSkipTLS bool
}

// parseProviderName parses a provider name that might contain namespace/name pattern
// Returns the name and namespace separately. If no namespace is specified, returns the default namespace.
func parseProviderName(providerName, defaultNamespace string) (name, namespace string) {
	if strings.Contains(providerName, "/") {
		parts := strings.SplitN(providerName, "/", 2)
		namespace = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
	} else {
		name = strings.TrimSpace(providerName)
		namespace = defaultNamespace
	}
	return name, namespace
}

// Create creates a new migration plan
func Create(ctx context.Context, opts CreatePlanOptions) error {
	c, err := client.GetDynamicClient(opts.ConfigFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Parse provider names to extract namespaces and names
	sourceProviderName, sourceProviderNamespace := parseProviderName(opts.SourceProvider, opts.Namespace)
	opts.SourceProvider = sourceProviderName
	opts.SourceProviderNamespace = sourceProviderNamespace

	// If the plan already exists, return an error
	_, err = c.Resource(client.PlansGVR).Namespace(opts.Namespace).Get(context.TODO(), opts.Name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("plan '%s' already exists in namespace '%s'", opts.Name, opts.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check if plan exists: %v", err)
	}

	// If target provider is not provided, find the first OpenShift provider
	if opts.TargetProvider == "" {
		defaultProvider, err := defaultprovider.GetDefaultOpenShiftProvider(opts.ConfigFlags, opts.Namespace)
		if err != nil {
			return fmt.Errorf("failed to get default target provider: %v", err)
		}
		opts.TargetProvider = defaultProvider
		fmt.Printf("No target provider specified, using default OpenShift provider: %s\n", opts.TargetProvider)
	}

	// Parse target provider name to extract namespace and name (do this after default provider logic)
	targetProviderName, targetProviderNamespace := parseProviderName(opts.TargetProvider, opts.Namespace)
	opts.TargetProvider = targetProviderName
	opts.TargetProviderNamespace = targetProviderNamespace

	// Validate that VMs exist in the source provider
	err = validateVMs(ctx, opts.ConfigFlags, &opts)
	if err != nil {
		return fmt.Errorf("VM validation failed: %v", err)
	}

	// Track which maps we create for cleanup if needed
	createdNetworkMap := false
	createdStorageMap := false

	// Extract VM names from the plan
	var planVMNames []string
	for _, planVM := range opts.PlanSpec.VMs {
		planVMNames = append(planVMNames, planVM.Name)
	}

	// If network map is not provided, create a default network map
	if opts.NetworkMapping == "" {
		if opts.NetworkPairs != "" {
			// Create network mapping from pairs
			networkMapName := fmt.Sprintf("%s-network", opts.Name)
			// For mapping creation, we need to pass the full provider references with namespaces
			sourceProviderRef := opts.SourceProvider
			if opts.SourceProviderNamespace != opts.Namespace {
				sourceProviderRef = fmt.Sprintf("%s/%s", opts.SourceProviderNamespace, opts.SourceProvider)
			}
			targetProviderRef := opts.TargetProvider
			if opts.TargetProviderNamespace != opts.Namespace {
				targetProviderRef = fmt.Sprintf("%s/%s", opts.TargetProviderNamespace, opts.TargetProvider)
			}
			err := mapping.CreateNetwork(opts.ConfigFlags, networkMapName, opts.Namespace, sourceProviderRef, targetProviderRef, opts.NetworkPairs, opts.InventoryURL)
			if err != nil {
				return fmt.Errorf("failed to create network map from pairs: %v", err)
			}
			opts.NetworkMapping = networkMapName
			createdNetworkMap = true
			fmt.Printf("Created network mapping '%s' from provided pairs\n", networkMapName)
		} else {
			// Create default network mapping using existing logic
			networkMapName, err := network.CreateNetworkMap(ctx, network.NetworkMapperOptions{
				Name:                    opts.Name,
				Namespace:               opts.Namespace,
				SourceProvider:          opts.SourceProvider,
				SourceProviderNamespace: opts.SourceProviderNamespace,
				TargetProvider:          opts.TargetProvider,
				TargetProviderNamespace: opts.TargetProviderNamespace,
				ConfigFlags:             opts.ConfigFlags,
				InventoryURL:            opts.InventoryURL,
				PlanVMNames:             planVMNames,
				DefaultTargetNetwork:    opts.DefaultTargetNetwork,
			})
			if err != nil {
				return fmt.Errorf("failed to create default network map: %v", err)
			}
			opts.NetworkMapping = networkMapName
			createdNetworkMap = true
		}
	}

	// If storage map is not provided, create a default storage map
	// Skip storage mapping for conversion-only migrations
	if opts.StorageMapping == "" && opts.PlanSpec.Type != forkliftv1beta1.MigrationOnlyConversion {
		if opts.StoragePairs != "" {
			// Create storage mapping from pairs
			storageMapName := fmt.Sprintf("%s-storage", opts.Name)
			// For mapping creation, we need to pass the full provider references with namespaces
			sourceProviderRef := opts.SourceProvider
			if opts.SourceProviderNamespace != opts.Namespace {
				sourceProviderRef = fmt.Sprintf("%s/%s", opts.SourceProviderNamespace, opts.SourceProvider)
			}
			targetProviderRef := opts.TargetProvider
			if opts.TargetProviderNamespace != opts.Namespace {
				targetProviderRef = fmt.Sprintf("%s/%s", opts.TargetProviderNamespace, opts.TargetProvider)
			}
			err := mapping.CreateStorageWithOptions(mapping.StorageCreateOptions{
				ConfigFlags:          opts.ConfigFlags,
				Name:                 storageMapName,
				Namespace:            opts.Namespace,
				SourceProvider:       sourceProviderRef,
				TargetProvider:       targetProviderRef,
				StoragePairs:         opts.StoragePairs,
				InventoryURL:         opts.InventoryURL,
				DefaultVolumeMode:    opts.DefaultVolumeMode,
				DefaultAccessMode:    opts.DefaultAccessMode,
				DefaultOffloadPlugin: opts.DefaultOffloadPlugin,
				DefaultOffloadSecret: opts.DefaultOffloadSecret,
				DefaultOffloadVendor: opts.DefaultOffloadVendor,
				// Offload secret creation options
				OffloadVSphereUsername: opts.OffloadVSphereUsername,
				OffloadVSpherePassword: opts.OffloadVSpherePassword,
				OffloadVSphereURL:      opts.OffloadVSphereURL,
				OffloadStorageUsername: opts.OffloadStorageUsername,
				OffloadStoragePassword: opts.OffloadStoragePassword,
				OffloadStorageEndpoint: opts.OffloadStorageEndpoint,
				OffloadCACert:          opts.OffloadCACert,
				OffloadInsecureSkipTLS: opts.OffloadInsecureSkipTLS,
			})
			if err != nil {
				// Clean up the network map if we created it
				if createdNetworkMap {
					if delErr := deleteMap(opts.ConfigFlags, client.NetworkMapGVR, opts.NetworkMapping, opts.Namespace); delErr != nil {
						fmt.Printf("Warning: failed to delete network map: %v\n", delErr)
					}
				}
				return fmt.Errorf("failed to create storage map from pairs: %v", err)
			}
			opts.StorageMapping = storageMapName
			createdStorageMap = true
			fmt.Printf("Created storage mapping '%s' from provided pairs\n", storageMapName)
		} else {
			// Create default storage mapping using existing logic
			storageMapName, err := storage.CreateStorageMap(ctx, storage.StorageMapperOptions{
				Name:                      opts.Name,
				Namespace:                 opts.Namespace,
				SourceProvider:            opts.SourceProvider,
				SourceProviderNamespace:   opts.SourceProviderNamespace,
				TargetProvider:            opts.TargetProvider,
				TargetProviderNamespace:   opts.TargetProviderNamespace,
				ConfigFlags:               opts.ConfigFlags,
				InventoryURL:              opts.InventoryURL,
				PlanVMNames:               planVMNames,
				DefaultTargetStorageClass: opts.DefaultTargetStorageClass,
			})
			if err != nil {
				// Clean up the network map if we created it
				if createdNetworkMap {
					if delErr := deleteMap(opts.ConfigFlags, client.NetworkMapGVR, opts.NetworkMapping, opts.Namespace); delErr != nil {
						fmt.Printf("Warning: failed to delete network map: %v\n", delErr)
					}
				}
				return fmt.Errorf("failed to create default storage map: %v", err)
			}
			opts.StorageMapping = storageMapName
			createdStorageMap = true
		}
	}

	// If target namespace is not provided, use the plan's namespace
	if opts.PlanSpec.TargetNamespace == "" {
		opts.PlanSpec.TargetNamespace = opts.Namespace
		fmt.Printf("No target namespace specified, using plan namespace: %s\n", opts.PlanSpec.TargetNamespace)
	}

	// Create a new Plan object using the PlanSpec
	planObj := &forkliftv1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Spec: opts.PlanSpec,
	}

	// Set provider references
	planObj.Spec.Provider = provider.Pair{
		Source: corev1.ObjectReference{
			Kind:       "Provider",
			APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
			Name:       opts.SourceProvider,
			Namespace:  opts.SourceProviderNamespace,
		},
		Destination: corev1.ObjectReference{
			Kind:       "Provider",
			APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
			Name:       opts.TargetProvider,
			Namespace:  opts.TargetProviderNamespace,
		},
	}

	// Set map references
	planObj.Spec.Map = plan.Map{
		Network: corev1.ObjectReference{
			Kind:       "NetworkMap",
			APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
			Name:       opts.NetworkMapping,
			Namespace:  opts.Namespace,
		},
	}

	// Only set storage mapping for non-conversion migrations
	if opts.PlanSpec.Type != forkliftv1beta1.MigrationOnlyConversion {
		planObj.Spec.Map.Storage = corev1.ObjectReference{
			Kind:       "StorageMap",
			APIVersion: forkliftv1beta1.SchemeGroupVersion.String(),
			Name:       opts.StorageMapping,
			Namespace:  opts.Namespace,
		}
	}
	planObj.Kind = "Plan"
	planObj.APIVersion = forkliftv1beta1.SchemeGroupVersion.String()

	// Convert Plan object to Unstructured
	unstructuredPlan, err := runtime.DefaultUnstructuredConverter.ToUnstructured(planObj)
	if err != nil {
		// Clean up created maps if conversion fails
		if createdNetworkMap {
			if delErr := deleteMap(opts.ConfigFlags, client.NetworkMapGVR, opts.NetworkMapping, opts.Namespace); delErr != nil {
				fmt.Printf("Warning: failed to delete network map: %v\n", delErr)
			}
		}
		if createdStorageMap {
			if delErr := deleteMap(opts.ConfigFlags, client.StorageMapGVR, opts.StorageMapping, opts.Namespace); delErr != nil {
				fmt.Printf("Warning: failed to delete storage map: %v\n", delErr)
			}
		}
		return fmt.Errorf("failed to convert Plan to Unstructured: %v", err)
	}
	planUnstructured := &unstructured.Unstructured{Object: unstructuredPlan}

	// Create the plan in the specified namespace
	createdPlan, err := c.Resource(client.PlansGVR).Namespace(opts.Namespace).Create(context.TODO(), planUnstructured, metav1.CreateOptions{})
	if err != nil {
		// Clean up created maps if plan creation fails
		if createdNetworkMap {
			if delErr := deleteMap(opts.ConfigFlags, client.NetworkMapGVR, opts.NetworkMapping, opts.Namespace); delErr != nil {
				fmt.Printf("Warning: failed to delete network map: %v\n", delErr)
			}
		}
		if createdStorageMap {
			if delErr := deleteMap(opts.ConfigFlags, client.StorageMapGVR, opts.StorageMapping, opts.Namespace); delErr != nil {
				fmt.Printf("Warning: failed to delete storage map: %v\n", delErr)
			}
		}
		return fmt.Errorf("failed to create plan: %v", err)
	}

	// MTV automatically sets the PVCNameTemplateUseGenerateName field to true, if opts.PlanSpec.PVCNameTemplateUseGenerateName is false
	// we need to patch the plan to re-set the PVCNameTemplateUseGenerateName field to false.
	if !opts.PlanSpec.PVCNameTemplateUseGenerateName {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"pvcNameTemplateUseGenerateName": false,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			// Ignore error here, we will still create the plan
			fmt.Printf("Warning: failed to marshal patch for PVCNameTemplateUseGenerateName: %v\n", err)
		} else {
			_, err = c.Resource(client.PlansGVR).Namespace(opts.Namespace).Patch(
				context.TODO(),
				createdPlan.GetName(),
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{},
			)
			if err != nil {
				// Ignore error here, we will still create the plan
				fmt.Printf("Warning: failed to patch plan for PVCNameTemplateUseGenerateName: %v\n", err)
			}
		}
	}

	// MTV automatically sets the MigrateSharedDisks field to true, if opts.PlanSpec.MigrateSharedDisks is false
	// we need to patch the plan to re-set the MigrateSharedDisks field to false.
	if !opts.PlanSpec.MigrateSharedDisks {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"migrateSharedDisks": false,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			// Ignore error here, we will still create the plan
			fmt.Printf("Warning: failed to marshal patch for MigrateSharedDisks: %v\n", err)
		} else {
			_, err = c.Resource(client.PlansGVR).Namespace(opts.Namespace).Patch(
				context.TODO(),
				createdPlan.GetName(),
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{},
			)
			if err != nil {
				// Ignore error here, we will still create the plan
				fmt.Printf("Warning: failed to patch plan for MigrateSharedDisks: %v\n", err)
			}
		}
	}

	// MTV UseCompatibilityMode sets the UseCompatibilityMode field to true, if opts.PlanSpec.UseCompatibilityMode is false
	// we need to patch the plan to re-set the UseCompatibilityMode field to false.
	if !opts.PlanSpec.UseCompatibilityMode {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"useCompatibilityMode": false,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			// Ignore error here, we will still create the plan
			fmt.Printf("Warning: failed to marshal patch for UseCompatibilityMode: %v\n", err)
		} else {
			_, err = c.Resource(client.PlansGVR).Namespace(opts.Namespace).Patch(
				context.TODO(),
				createdPlan.GetName(),
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{},
			)
			if err != nil {
				// Ignore error here, we will still create the plan
				fmt.Printf("Warning: failed to patch plan for UseCompatibilityMode: %v\n", err)
			}
		}
	}

	// MTV automatically sets the PreserveStaticIPs field to true, if opts.PlanSpec.PreserveStaticIPs is false
	// we need to patch the plan to re-set the PreserveStaticIPs field to false.
	if !opts.PlanSpec.PreserveStaticIPs {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"preserveStaticIPs": false,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			// Ignore error here, we will still create the plan
			fmt.Printf("Warning: failed to marshal patch for PreserveStaticIPs: %v\n", err)
		} else {
			_, err = c.Resource(client.PlansGVR).Namespace(opts.Namespace).Patch(
				context.TODO(),
				createdPlan.GetName(),
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{},
			)
			if err != nil {
				// Ignore error here, we will still create the plan
				fmt.Printf("Warning: failed to patch plan for PreserveStaticIPs: %v\n", err)
			}
		}
	}

	// MTV automatically sets the RunPreflightInspection field to true, if opts.PlanSpec.RunPreflightInspection is false
	// we need to patch the plan to re-set the RunPreflightInspection field to false.
	if !opts.PlanSpec.RunPreflightInspection {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"runPreflightInspection": false,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			// Ignore error here, we will still create the plan
			fmt.Printf("Warning: failed to marshal patch for RunPreflightInspection: %v\n", err)
		} else {
			_, err = c.Resource(client.PlansGVR).Namespace(opts.Namespace).Patch(
				context.TODO(),
				createdPlan.GetName(),
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{},
			)
			if err != nil {
				// Ignore error here, we will still create the plan
				fmt.Printf("Warning: failed to patch plan for RunPreflightInspection: %v\n", err)
			}
		}
	}

	// Set ownership of maps if we created them
	if createdNetworkMap {
		err = setMapOwnership(opts.ConfigFlags, createdPlan, client.NetworkMapGVR, opts.NetworkMapping, opts.Namespace)
		if err != nil {
			fmt.Printf("Warning: failed to set ownership for network map: %v\n", err)
		}
	}

	if createdStorageMap {
		err = setMapOwnership(opts.ConfigFlags, createdPlan, client.StorageMapGVR, opts.StorageMapping, opts.Namespace)
		if err != nil {
			fmt.Printf("Warning: failed to set ownership for storage map: %v\n", err)
		}
	}

	fmt.Printf("plan/%s created\n", opts.Name)
	return nil
}

// validateVMs validates that all VMs in the VMList exist in the source provider,
// sets their IDs based on the names, and removes any that don't exist.
// Returns an error if no valid VMs remain.
func validateVMs(ctx context.Context, configFlags *genericclioptions.ConfigFlags, opts *CreatePlanOptions) error {
	// Fetch source provider using the parsed namespace
	sourceProvider, err := inventory.GetProviderByName(ctx, configFlags, opts.SourceProvider, opts.SourceProviderNamespace)
	if err != nil {
		return fmt.Errorf("failed to get source provider: %v", err)
	}

	// Fetch source VMs inventory
	sourceVMsInventory, err := client.FetchProviderInventory(configFlags, opts.InventoryURL, sourceProvider, "vms")
	if err != nil {
		return fmt.Errorf("failed to fetch source VMs inventory: %v", err)
	}

	sourceVMsArray, ok := sourceVMsInventory.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected data format: expected array for source VMs inventory")
	}

	// Create maps for VM names to VM IDs and VM IDs to VM names for lookup
	vmNameToIDMap := make(map[string]string)
	vmIDToNameMap := make(map[string]string)
	vmIDToNamespaceMap := make(map[string]string)

	for _, item := range sourceVMsArray {
		vm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		vmName, ok := vm["name"].(string)
		if !ok {
			continue
		}

		vmID, ok := vm["id"].(string)
		if !ok {
			continue
		}

		vmNamespace, ok := vm["namespace"].(string)
		if !ok {
			// If namespace is not available, set it to empty
			vmNamespace = ""
		}

		vmNameToIDMap[vmName] = vmID
		vmIDToNameMap[vmID] = vmName
		vmIDToNamespaceMap[vmID] = vmNamespace
	}

	// Process VMs: first those with IDs, then those with only names
	var validVMs []plan.VM

	// First process VMs that already have IDs
	for _, planVM := range opts.PlanSpec.VMs {
		if planVM.ID != "" {
			// Check if VM with this ID exists in inventory
			if vmName, exists := vmIDToNameMap[planVM.ID]; exists {
				// If name is empty or different, update it
				if planVM.Name == "" {
					planVM.Name = vmName
				}
				validVMs = append(validVMs, planVM)
			} else {
				fmt.Printf("Warning: VM with ID '%s' not found in source provider, removing from plan\n", planVM.ID)
			}
		}
	}

	// Then process VMs that only have names (and need IDs)
	for _, planVM := range opts.PlanSpec.VMs {
		if planVM.ID == "" && planVM.Name != "" {
			vmID, exists := vmNameToIDMap[planVM.Name]
			if exists {
				planVM.ID = vmID
				validVMs = append(validVMs, planVM)
			} else {
				fmt.Printf("Warning: VM with name '%s' not found in source provider, removing from plan\n", planVM.Name)
			}
		}
	}

	// Add namespaces to VMs that don't have them, if available
	for i, planVM := range validVMs {
		if vmNamespace, exists := vmIDToNamespaceMap[planVM.ID]; exists {
			validVMs[i].Namespace = vmNamespace
		}
	}

	// Update the VM list
	opts.PlanSpec.VMs = validVMs

	// Check if any VMs remain
	if len(opts.PlanSpec.VMs) == 0 {
		return fmt.Errorf("no valid VMs found in source provider matching the plan VMs")
	}

	return nil
}

// setMapOwnership sets the plan as the owner of the map
func setMapOwnership(configFlags *genericclioptions.ConfigFlags, plan *unstructured.Unstructured, mapGVR schema.GroupVersionResource, mapName, namespace string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	// Create the owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion: plan.GetAPIVersion(),
		Kind:       plan.GetKind(),
		Name:       plan.GetName(),
		UID:        plan.GetUID(),
		Controller: boolPtr(true),
	}

	// Patch map to add the owner reference
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"ownerReferences": []metav1.OwnerReference{ownerRef},
		},
	}

	// Convert patch to JSON bytes
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %v", err)
	}

	// Apply the patch to the map
	_, err = c.Resource(mapGVR).Namespace(namespace).Patch(
		context.Background(),
		mapName,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch map with owner reference: %v", err)
	}

	return nil
}

// deleteMap deletes a map resource
func deleteMap(configFlags *genericclioptions.ConfigFlags, mapGVR schema.GroupVersionResource, mapName, namespace string) error {
	c, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return fmt.Errorf("failed to get client: %v", err)
	}

	err = c.Resource(mapGVR).Namespace(namespace).Delete(
		context.Background(),
		mapName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete map '%s': %v", mapName, err)
	}

	return nil
}

// boolPtr returns a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}
