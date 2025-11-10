package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetMigrationStorageInput represents the input for GetMigrationStorage
type GetMigrationStorageInput struct {
	ResourceType  string `json:"resource_type,omitempty" jsonschema:"Type of storage resource - 'all', 'pvc', or 'datavolume' (default 'all')"`
	MigrationID   string `json:"migration_id,omitempty" jsonschema:"Migration UUID to filter by (optional) - get from plan VM status"`
	PlanID        string `json:"plan_id,omitempty" jsonschema:"Plan UUID to filter by (optional) - get from plan metadata.uid"`
	VMID          string `json:"vm_id,omitempty" jsonschema:"VM ID to filter by (optional) - e.g., vm-47, vm-73"`
	Namespace     string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace to search in (optional)"`
	AllNamespaces bool   `json:"all_namespaces,omitempty" jsonschema:"Search across all namespaces"`
}

// GetGetMigrationStorageTool returns the tool definition
func GetGetMigrationStorageTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "GetMigrationStorage",
		Description: `Get storage resources (PVCs and DataVolumes) related to VM migrations.

    Unified tool to access migration storage resources with granular control over resource types.
    Supports filtering by migration labels to find specific storage resources.

    Resource Types:
    - 'all': Get both PVCs and DataVolumes (default)
    - 'pvc': Get only PersistentVolumeClaims
    - 'datavolume': Get only DataVolumes

    Find storage resources that are part of VM migrations by searching for specific labels:
    - migration: Migration UUID (NOT the migration name)
    - plan: Plan UUID (NOT the plan name)
    - vmID: VM identifier (e.g., vm-47)

    IMPORTANT: Use UUIDs, not names!
    - CORRECT: migration_id="4399056b-4f08-497d-a559-3dd530de3459" (UUID from plan status)
    - WRONG: migration_id="migrate-small-vm-mmpj4" (migration name - won't work)
    - CORRECT: plan_id="3943f9a2-d4a4-4326-b25c-57d06ff53c21" (UUID from plan metadata)
    - WRONG: plan_id="migrate-small-vm" (plan name - won't work)

    How to get the correct UUIDs:
    1. Use GetPlanVms() to get migration UUIDs from plan status
    2. Use ListResources(resource_type="plan") with json output to get plan UUIDs from metadata.uid
    3. Check kubectl labels: kubectl get pvc,dv -n <namespace> --show-labels

    Args:
        resource_type: Type of storage resource - 'all', 'pvc', or 'datavolume' (default 'all')
        migration_id: Migration UUID to filter by (optional) - get from plan VM status
        plan_id: Plan UUID to filter by (optional) - get from plan metadata.uid
        vm_id: VM ID to filter by (optional) - e.g., vm-47, vm-73
        namespace: Kubernetes namespace to search in (optional)
        all_namespaces: Search across all namespaces

    Returns:
        JSON formatted storage information

        Enhanced JSON Output:
        Resources include:
        - "describe" field with kubectl describe output
        - Complete diagnostic information and events

    Examples:
        # Get all storage for specific migration
        GetMigrationStorage(resource_type="all", migration_id="4399056b-4f08-497d-a559-3dd530de3459",
                           plan_id="3943f9a2-d4a4-4326-b25c-57d06ff53c21", vm_id="vm-47", namespace="demo")

        # Get only PVCs in namespace
        GetMigrationStorage(resource_type="pvc", namespace="demo")

        # Get only DataVolumes for specific plan
        GetMigrationStorage(resource_type="datavolume", plan_id="3943f9a2-d4a4-4326-b25c-57d06ff53c21")`,
	}
}

func HandleGetMigrationStorage(ctx context.Context, req *mcp.CallToolRequest, input GetMigrationStorageInput) (*mcp.CallToolResult, any, error) {
	resourceType := input.ResourceType
	if resourceType == "" {
		resourceType = "all"
	}

	// Validate resource type
	validTypes := []string{"all", "pvc", "datavolume"}
	found := false
	for _, t := range validTypes {
		if resourceType == t {
			found = true
			break
		}
	}
	if !found {
		return nil, "", fmt.Errorf("invalid resource_type '%s'. Valid types: %v", resourceType, validTypes)
	}

	if resourceType == "pvc" {
		return getMigrationPVCs(ctx, input.MigrationID, input.PlanID, input.VMID, input.Namespace, input.AllNamespaces)
	} else if resourceType == "datavolume" {
		return getMigrationDataVolumes(ctx, input.MigrationID, input.PlanID, input.VMID, input.Namespace, input.AllNamespaces)
	} else {
		// Get both
		_, pvcData, pvcErr := getMigrationPVCs(ctx, input.MigrationID, input.PlanID, input.VMID, input.Namespace, input.AllNamespaces)
		_, dvData, dvErr := getMigrationDataVolumes(ctx, input.MigrationID, input.PlanID, input.VMID, input.Namespace, input.AllNamespaces)

		if pvcErr != nil && dvErr != nil {
			return nil, "", fmt.Errorf("failed to get PVCs: %v; failed to get DataVolumes: %v", pvcErr, dvErr)
		}

		combined := map[string]interface{}{
			"pvcs":        map[string]interface{}{},
			"datavolumes": map[string]interface{}{},
		}

		if pvcErr == nil {
			combined["pvcs"] = pvcData
		}

		if dvErr == nil {
			combined["datavolumes"] = dvData
		}

		return nil, combined, nil
	}
}
