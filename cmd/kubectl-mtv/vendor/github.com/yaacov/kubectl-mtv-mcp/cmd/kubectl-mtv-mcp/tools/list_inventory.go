package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// ListInventoryInput represents the input for ListInventory
type ListInventoryInput struct {
	ResourceType  string `json:"resource_type" jsonschema:"Type of inventory resource to list"`
	ProviderName  string `json:"provider_name,omitempty" jsonschema:"Name of the provider to query (required for most resource types, optional for 'provider' type)"`
	Namespace     string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace containing the provider (optional)"`
	AllNamespaces bool   `json:"all_namespaces,omitempty" jsonschema:"Search across all namespaces (optional, only applicable for 'provider' resource type)"`
	Query         string `json:"query,omitempty" jsonschema:"Optional filter query using SQL-like syntax with WHERE/SELECT/ORDER BY/LIMIT"`
	OutputFormat  string `json:"output_format,omitempty" jsonschema:"Output format - 'json' for full data or 'planvms' for plan-compatible VM structures (default 'json')"`
	InventoryURL  string `json:"inventory_url,omitempty" jsonschema:"Base URL for inventory service (optional, auto-discovered if not provided)"`
	DryRun        bool   `json:"dry_run,omitempty" jsonschema:"If true, shows commands instead of executing (educational mode)"`
}

// GetListInventoryTool returns the tool definition
func GetListInventoryTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "ListInventory",
		Description: `List inventory resources from a provider.

    Unified tool to query various resource types from provider inventories.
    Supports all resource types with powerful SQL-like query capabilities.

    AVAILABLE RESOURCE TYPES BY PROVIDER:
    vSphere: vm, network, storage, host, cluster, datacenter, datastore, folder, resource-pool
    oVirt: vm, network, storage, host, cluster, datacenter, disk, disk-profile, nic-profile
    OpenStack: vm, network, storage, flavor, image, instance, project, volume, volumetype, snapshot, subnet
    OpenShift: vm, network, storage, namespace, pvc, data-volume
    OVA: vm, network, storage

    SPECIAL RESOURCE TYPES:
    provider: Get provider inventory information including status, resource counts, and full CRD objects
             - provider_name is optional (omit to get all providers)
             - Includes: vmCount, hostCount, datastoreCount, networkCount, clusterCount, datacenterCount
             - Contains full provider CRD object with status conditions and configuration
             - Supports all standard TSL queries for filtering and selection

    QUERY SYNTAX:
    All inventory tools support SQL-like queries with Tree Search Language (TSL):
    - SELECT field1, field2 AS alias, function(field3) AS name
    - WHERE condition (using TSL operators and functions)
    - ORDER BY field1 [ASC|DESC], field2
    - LIMIT n

    ORDER REQUIREMENT: Parts can be omitted but MUST follow this sequence if present.
    Valid: "WHERE x = 1", "SELECT a WHERE b = 2", "WHERE x = 1 ORDER BY y LIMIT 5"
    Invalid: "WHERE x = 1 SELECT a", "LIMIT 5 WHERE x = 1"

    TSL OPERATORS: =, !=, <, <=, >, >=, LIKE, ILIKE, ~= (regex), ~! (regex), IN, BETWEEN, AND, OR, NOT
    TSL FUNCTIONS: sum(), len(), any(), all()
    TSL LITERALS: strings ('text'), numbers (1024, 2.5Gi), dates ('2023-01-01'), booleans (true/false)
    TSL ARRAY ACCESS: Use [*] for array elements, dot notation for nested fields (e.g., disks[*].capacity, parent.name)

    TSL USAGE RULES:
    - LIKE patterns: '%' = any chars, '_' = single char (case-sensitive), ILIKE = case-insensitive
    - String values MUST be quoted: 'text', "text", or backtick-text
    - Array functions: len(networks) > 2, sum(disks[*].capacity) > 1000, any(tags[*] = 'prod')
    - Use parentheses for complex logic: (a = 1 OR b = 2) AND c = 3
    - Regex match (~=) and not match (~!): name ~= '^web.*', status ~! 'test.*'

    EXAMPLE QUERIES FOR SPECIALIZED RESOURCES:

    Folders (vSphere):
    - "WHERE name LIKE '%VM%' AND type = 'vm'"
    - "SELECT name, path, parent.name, childrenCount ORDER BY name"

    Flavors (OpenStack):
    - "WHERE vcpus >= 4 AND ram >= 8192 ORDER BY vcpus DESC"
    - "SELECT name, vcpus, ram, disk, isPublic WHERE isPublic = true"

    Disk Profiles (oVirt):
    - "WHERE storageDomain = 'specific-domain-id'"
    - "SELECT name, description, storageDomain, qosId ORDER BY name"

    NIC Profiles (oVirt):
    - "WHERE networkFilter != ''"
    - "SELECT name, description, network, portMirroring, customProperties ORDER BY name"

    Projects (OpenStack):
    - "WHERE enabled = true ORDER BY name"
    - "SELECT name, description, enabled, isDomain ORDER BY name"

    Resource Pools (vSphere):
    - "WHERE cpuAllocation.limit > 0"
    - "SELECT name, cpuLimit, memoryLimit, parent.name ORDER BY cpuLimit DESC"

    Subnets (OpenStack):
    - "WHERE enableDhcp = true AND ipVersion = 4"
    - "SELECT name, cidr, gatewayIp, networkId, enableDhcp ORDER BY name"

    Images (OpenStack):
    - "WHERE status IN ['active', 'queued', 'saving'] AND visibility = 'public'"
    - "SELECT name, status, visibility, diskFormat, containerFormat, size ORDER BY name"

    Volumes (OpenStack):
    - "WHERE status = 'available' AND size >= 10"
    - "SELECT name, id, status, size, volumeType, bootable ORDER BY size DESC"
    - "WHERE bootable = true ORDER BY name"

    Volume Types (OpenStack):
    - "WHERE isPublic = true ORDER BY name"
    - "SELECT name, id, description, isPublic ORDER BY name"

    DataVolumes (OpenShift):
    - "WHERE object.status.phase = 'Succeeded'"
    - "SELECT name, namespace, object.spec.source, object.status.phase ORDER BY name"

    PVCs (OpenShift):
    - "WHERE object.status.phase = 'Bound'"
    - "SELECT name, namespace, object.spec.storageClassName, object.status.capacity.storage ORDER BY name"

    Providers (Special):
    - "WHERE name ~= 'chho'" - Find providers with names matching regex pattern
    - "WHERE object.status.phase = 'Ready' AND vmCount > 0" - Ready providers with VMs
    - "SELECT name, type, object.status.phase, vmCount, hostCount ORDER BY vmCount DESC" - Provider overview sorted by VM count
    - "WHERE type = 'vsphere' AND vmCount > 10" - vSphere providers with more than 10 VMs
    - "SELECT name, type, apiVersion, product, vmCount, datastoreCount, networkCount WHERE type = 'vsphere'" - vSphere provider details

    Output Formats:
    - 'json': Full inventory data with all fields (default)
    - 'planvms': Plan-compatible VM structures for use with create_plan(vms="@file.yaml")

    The 'planvms' format is specifically useful when listing VMs for plan creation:
    - Returns minimal VM structures suitable for plan VM selection
    - Output can be saved to a file and used directly with create_plan
    - Example: ListInventory("vm", "my-provider", output_format="planvms") > vm-list.yaml

    Args:
        resource_type: Type of inventory resource to list
        provider_name: Name of the provider to query (required for most resource types, optional for 'provider' type)
        namespace: Kubernetes namespace containing the provider (optional)
        all_namespaces: Search across all namespaces (optional, only applicable for 'provider' resource type)
        query: Optional filter query using SQL-like syntax with WHERE/SELECT/ORDER BY/LIMIT
        output_format: Output format - 'json' for full data or 'planvms' for plan-compatible VM structures (default 'json')
        inventory_url: Base URL for inventory service (optional, auto-discovered if not provided)

    Returns:
        JSON formatted inventory or plan-compatible VM structures (planvms format)

    Examples:
        # Get VMs from a specific provider
        ListInventory(resource_type="vm", provider_name="vsphere-provider", namespace="demo")

        # Get storage inventory with filtering
        ListInventory(resource_type="storage", provider_name="openstack-provider", query="WHERE capacity > 100000000000")

        # Get provider inventory for all providers
        ListInventory(resource_type="provider")

        # Get provider inventory across all namespaces
        ListInventory(resource_type="provider", all_namespaces=true)

        # Get VMs in planvms format for migration planning
        ListInventory(resource_type="vm", provider_name="vsphere-provider", output_format="planvms")

        # Get OpenStack volumes with filtering
        ListInventory(resource_type="volume", provider_name="openstack-provider", query="WHERE status = 'available' AND size >= 10")

        # Get OpenStack volume types
        ListInventory(resource_type="volumetype", provider_name="openstack-provider")`,
	}
}

func HandleListInventory(ctx context.Context, req *mcp.CallToolRequest, input ListInventoryInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"resource_type": input.ResourceType,
	}); err != nil {
		return nil, "", err
	}

	args := []string{"get", "inventory", input.ResourceType}

	// Special handling for provider resource type
	if input.ResourceType == "provider" {
		if input.ProviderName != "" {
			args = append(args, input.ProviderName)
		}
	} else {
		args = append(args, input.ProviderName)
	}

	if input.AllNamespaces {
		args = append(args, "-A")
	} else if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	if input.Query != "" {
		args = append(args, "-q", input.Query)
	}

	if input.InventoryURL != "" {
		args = append(args, "--inventory-url", input.InventoryURL)
	}

	// Support both json and planvms output formats
	outputFormat := input.OutputFormat
	if outputFormat == "" {
		outputFormat = "json"
	}
	if outputFormat == "json" || outputFormat == "planvms" {
		args = append(args, "-o", outputFormat)
	} else {
		args = append(args, "-o", "json") // Default to json for unsupported formats
	}

	result, err := mtvmcp.RunKubectlMTVCommand(ctx, args)
	if err != nil {
		return nil, "", err
	}

	// Unmarshal the full CommandResponse to provide complete diagnostic information
	data, err := mtvmcp.UnmarshalJSONResponse(result)
	if err != nil {
		return nil, "", err
	}
	return nil, data, nil
}
