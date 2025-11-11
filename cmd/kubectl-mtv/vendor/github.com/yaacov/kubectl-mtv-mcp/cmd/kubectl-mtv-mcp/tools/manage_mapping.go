package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// ManageMappingInput represents the input for ManageMapping
type ManageMappingInput struct {
	Action               string `json:"action" jsonschema:"Action to perform - 'create', 'delete', or 'patch'"`
	MappingType          string `json:"mapping_type" jsonschema:"Type of mapping - 'network' or 'storage'"`
	MappingName          string `json:"mapping_name" jsonschema:"Name of the mapping"`
	Namespace            string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace (optional)"`
	SourceProvider       string `json:"source_provider,omitempty" jsonschema:"Source provider name (required for create)"`
	TargetProvider       string `json:"target_provider,omitempty" jsonschema:"Target provider name (required for create)"`
	Pairs                string `json:"pairs,omitempty" jsonschema:"Initial mapping pairs for create (optional)"`
	AddPairs             string `json:"add_pairs,omitempty" jsonschema:"Pairs to add during patch (optional)"`
	UpdatePairs          string `json:"update_pairs,omitempty" jsonschema:"Pairs to update during patch (optional)"`
	RemovePairs          string `json:"remove_pairs,omitempty" jsonschema:"Source names to remove during patch (optional)"`
	InventoryURL         string `json:"inventory_url,omitempty" jsonschema:"Inventory service URL (optional)"`
	DefaultVolumeMode    string `json:"default_volume_mode,omitempty" jsonschema:"Default volume mode for storage pairs (Filesystem|Block) (optional)"`
	DefaultAccessMode    string `json:"default_access_mode,omitempty" jsonschema:"Default access mode for storage pairs (ReadWriteOnce|ReadWriteMany|ReadOnlyMany) (optional)"`
	DefaultOffloadPlugin string `json:"default_offload_plugin,omitempty" jsonschema:"Default offload plugin type for storage pairs (optional)"`
	DefaultOffloadSecret string `json:"default_offload_secret,omitempty" jsonschema:"Default offload plugin secret name for storage pairs (optional)"`
	DefaultOffloadVendor string `json:"default_offload_vendor,omitempty" jsonschema:"Default offload plugin vendor for storage pairs (optional)"`
	DryRun               bool   `json:"dry_run,omitempty" jsonschema:"If true, shows commands instead of executing (educational mode)"`
}

// GetManageMappingTool returns the tool definition
func GetManageMappingTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "ManageMapping",
		Description: `Manage network and storage mappings with unified operations.

    Unified tool for creating, deleting, and patching both network and storage mappings.
    Mappings define how source resources map to target resources during VM migration.

    Actions:
    - 'create': Create a new mapping
    - 'delete': Delete an existing mapping
    - 'patch': Modify an existing mapping

    Mapping Types:
    - 'network': Network mappings for VM network interfaces
    - 'storage': Storage mappings for VM disk placement

    Action-Specific Parameters:
    Create:
    - source_provider, target_provider (required)
    - pairs (optional, initial mappings)

    Delete:
    - No additional parameters

    Patch:
    - add_pairs, update_pairs, remove_pairs (at least one required)

    Mapping Pairs Format:
    Network pairs: 'source:target-namespace/target-network' or 'source:target-network'
    Storage pairs: 'source:storage-class[;volumeMode=Block|Filesystem][;accessMode=ReadWriteOnce|ReadWriteMany|ReadOnlyMany][;offloadPlugin=vsphere][;offloadSecret=secret-name][;offloadVendor=flashsystem|vantara|ontap|primera3par|pureFlashArray|powerflex|powermax|powerstore|infinibox]' (comma-separated pairs, semicolon-separated parameters)
    Special values: 'source:default' (pod networking), 'source:ignored' (skip network)
    Multiple pairs: comma-separated 'pair1,pair2,pair3'

    Network Mapping Constraints (VALIDATED by MCP):
    - All source networks must be mapped (no source networks can be left unmapped)
    - Pod networking ('default') can only be mapped ONCE across all sources
    - Each specific NAD can only be mapped ONCE across all sources
    - 'ignored' can be used multiple times for sources that don't need network access
    - VALIDATION EXAMPLES:
      • INVALID: "source1:default,source2:default" (pod network mapped twice)
      • INVALID: "source1:nad1,source2:nad1" (same NAD mapped twice)
      • VALID: "source1:default,source2:ignored,source3:nad1"
      • VALID: "source1:nad1,source2:nad2,source3:ignored,source4:ignored"

    Storage Mapping Constraints:
    - All source storages must be mapped (no source storages can be left unmapped)
    - Preferred storage classes have virt annotation, k8s annotation, or "virtualization" in name
    - Virt annotation: storageclass.kubevirt.io/is-default-virt-class=true (highest priority)
    - K8s annotation: storageclass.kubernetes.io/is-default-class=true (fallback if no virt annotation)
    - Name matching: case-insensitive search for "virtualization" in storage class name
    - Selection priority: user-defined > virt annotation > k8s annotation > name match > first available

    Args:
        action: Action to perform - 'create', 'delete', or 'patch'
        mapping_type: Type of mapping - 'network' or 'storage'
        mapping_name: Name of the mapping
        namespace: Kubernetes namespace (optional)
        source_provider: Source provider name (required for create)
        target_provider: Target provider name (required for create)
        pairs: Initial mapping pairs for create (optional)
        add_pairs: Pairs to add during patch (optional)
        update_pairs: Pairs to update during patch (optional)
        remove_pairs: Source names to remove during patch (optional)
        inventory_url: Inventory service URL (optional)
        default_volume_mode: Default volume mode for storage pairs (Filesystem|Block) (optional)
        default_access_mode: Default access mode for storage pairs (ReadWriteOnce|ReadWriteMany|ReadOnlyMany) (optional)
        default_offload_plugin: Default offload plugin type for storage pairs (optional)
            • Supported plugins: vsphere
        default_offload_secret: Default offload plugin secret name for storage pairs (optional)
        default_offload_vendor: Default offload plugin vendor for storage pairs (optional)
            • Supported vendors: flashsystem, vantara, ontap, primera3par, pureFlashArray, powerflex, powermax, powerstore, infinibox

    Returns:
        Command output confirming the mapping operation

    Examples:
        # Create network mapping
        ManageMapping(action="create", mapping_type="network", mapping_name="my-net-mapping",
                     source_provider="vsphere-provider", target_provider="openshift-provider",
                     pairs="VM Network:default,Management:mgmt/mgmt-net")

        # Create storage mapping with enhanced features
        ManageMapping(action="create", mapping_type="storage", mapping_name="my-storage-mapping",
                     source_provider="vsphere-provider", target_provider="openshift-provider",
                     pairs="fast-datastore:ocs-storagecluster-ceph-rbd;volumeMode=Block;accessMode=ReadWriteOnce;offloadPlugin=vsphere;offloadVendor=flashsystem",
                     default_volume_mode="Block")

        # Auto-selection will prioritize storage classes with these annotations:
        # 1. storageclass.kubevirt.io/is-default-virt-class=true (preferred for virtualization)
        # 2. storageclass.kubernetes.io/is-default-class=true (Kubernetes default)
        # 3. Storage classes with "virtualization" in name (e.g., "ocs-virtualization-rbd")
        # 4. First available storage class if none of the above are found

        # Delete mapping
        ManageMapping(action="delete", mapping_type="network", mapping_name="old-mapping")

        # Patch network mapping - add and remove pairs
        ManageMapping(action="patch", mapping_type="network", mapping_name="my-net-mapping",
                     add_pairs="DMZ:dmz-namespace/dmz-net",
                     remove_pairs="OldNetwork,UnusedNetwork")

        # Patch storage mapping with enhanced options
        ManageMapping(action="patch", mapping_type="storage", mapping_name="my-storage-mapping",
                     update_pairs="slow-datastore:standard;volumeMode=Filesystem,fast-datastore:premium;volumeMode=Block;accessMode=ReadWriteOnce",
                     default_offload_plugin="vsphere", default_offload_vendor="ontap")`,
	}
}

func HandleManageMapping(ctx context.Context, req *mcp.CallToolRequest, input ManageMappingInput) (*mcp.CallToolResult, any, error) {
	// Enable dry run mode if requested
	if input.DryRun {
		ctx = mtvmcp.WithDryRun(ctx, true)
	}

	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"action":       input.Action,
		"mapping_type": input.MappingType,
		"mapping_name": input.MappingName,
	}); err != nil {
		return nil, "", err
	}

	// Validate action and mapping type
	validActions := []string{"create", "delete", "patch"}
	validTypes := []string{"network", "storage"}

	actionValid := false
	for _, a := range validActions {
		if input.Action == a {
			actionValid = true
			break
		}
	}
	if !actionValid {
		return nil, "", fmt.Errorf("invalid action '%s'. Valid actions: %v", input.Action, validActions)
	}

	typeValid := false
	for _, t := range validTypes {
		if input.MappingType == t {
			typeValid = true
			break
		}
	}
	if !typeValid {
		return nil, "", fmt.Errorf("invalid mapping_type '%s'. Valid types: %v", input.MappingType, validTypes)
	}

	// Validate action-specific requirements
	if input.Action == "create" && (input.SourceProvider == "" || input.TargetProvider == "") {
		return nil, "", fmt.Errorf("source_provider and target_provider are required for create action")
	}
	if input.Action == "patch" && input.AddPairs == "" && input.UpdatePairs == "" && input.RemovePairs == "" {
		return nil, "", fmt.Errorf("at least one of add_pairs, update_pairs, or remove_pairs is required for patch action")
	}

	// Validate network pairs constraints for network mappings
	if input.MappingType == "network" {
		if input.Action == "create" && input.Pairs != "" {
			if err := mtvmcp.ValidateNetworkPairs(input.Pairs); err != nil {
				return nil, "", err
			}
		}
		if input.Action == "patch" {
			if input.AddPairs != "" {
				if err := mtvmcp.ValidateNetworkPairs(input.AddPairs); err != nil {
					return nil, "", err
				}
			}
			if input.UpdatePairs != "" {
				if err := mtvmcp.ValidateNetworkPairs(input.UpdatePairs); err != nil {
					return nil, "", err
				}
			}
		}
	}

	var args []string

	switch input.Action {
	case "create":
		args = []string{"create", "mapping", input.MappingType, input.MappingName}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}
		if input.SourceProvider != "" {
			args = append(args, "--source", input.SourceProvider)
		}
		if input.TargetProvider != "" {
			args = append(args, "--target", input.TargetProvider)
		}
		if input.Pairs != "" {
			if input.MappingType == "network" {
				args = append(args, "--network-pairs", input.Pairs)
			} else {
				args = append(args, "--storage-pairs", input.Pairs)
			}
		}
		if input.MappingType == "storage" {
			if input.DefaultVolumeMode != "" {
				vm := strings.ToLower(strings.TrimSpace(input.DefaultVolumeMode))
				switch vm {
				case "filesystem", "fs":
					args = append(args, "--default-volume-mode", "Filesystem")
				case "block":
					args = append(args, "--default-volume-mode", "Block")
				default:
					return nil, "", fmt.Errorf("invalid default_volume_mode: %s (valid: Filesystem|Block)", input.DefaultVolumeMode)
				}
			}
			if input.DefaultAccessMode != "" {
				am := strings.ToLower(strings.TrimSpace(input.DefaultAccessMode))
				switch am {
				case "readwriteonce", "rwo":
					args = append(args, "--default-access-mode", "ReadWriteOnce")
				case "readwritemany", "rwx":
					args = append(args, "--default-access-mode", "ReadWriteMany")
				case "readonlymany", "rom":
					args = append(args, "--default-access-mode", "ReadOnlyMany")
				default:
					return nil, "", fmt.Errorf("invalid default_access_mode: %s (valid: ReadWriteOnce|ReadWriteMany|ReadOnlyMany)", input.DefaultAccessMode)
				}
			}
			if input.DefaultOffloadPlugin != "" {
				args = append(args, "--default-offload-plugin", input.DefaultOffloadPlugin)
			}
			if input.DefaultOffloadSecret != "" {
				args = append(args, "--default-offload-secret", input.DefaultOffloadSecret)
			}
			if input.DefaultOffloadVendor != "" {
				args = append(args, "--default-offload-vendor", input.DefaultOffloadVendor)
			}
		}
		if input.InventoryURL != "" {
			args = append(args, "--inventory-url", input.InventoryURL)
		}

	case "delete":
		args = []string{"delete", "mapping", input.MappingType, input.MappingName}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}

	case "patch":
		args = []string{"patch", "mapping", input.MappingType, input.MappingName}
		if input.Namespace != "" {
			args = append(args, "-n", input.Namespace)
		}
		if input.AddPairs != "" {
			args = append(args, "--add-pairs", input.AddPairs)
		}
		if input.UpdatePairs != "" {
			args = append(args, "--update-pairs", input.UpdatePairs)
		}
		if input.RemovePairs != "" {
			args = append(args, "--remove-pairs", input.RemovePairs)
		}
		if input.MappingType == "storage" {
			if input.DefaultVolumeMode != "" {
				vm := strings.ToLower(strings.TrimSpace(input.DefaultVolumeMode))
				switch vm {
				case "filesystem", "fs":
					args = append(args, "--default-volume-mode", "Filesystem")
				case "block":
					args = append(args, "--default-volume-mode", "Block")
				default:
					return nil, "", fmt.Errorf("invalid default_volume_mode: %s (valid: Filesystem|Block)", input.DefaultVolumeMode)
				}
			}
			if input.DefaultAccessMode != "" {
				am := strings.ToLower(strings.TrimSpace(input.DefaultAccessMode))
				switch am {
				case "readwriteonce", "rwo":
					args = append(args, "--default-access-mode", "ReadWriteOnce")
				case "readwritemany", "rwx":
					args = append(args, "--default-access-mode", "ReadWriteMany")
				case "readonlymany", "rom":
					args = append(args, "--default-access-mode", "ReadOnlyMany")
				default:
					return nil, "", fmt.Errorf("invalid default_access_mode: %s (valid: ReadWriteOnce|ReadWriteMany|ReadOnlyMany)", input.DefaultAccessMode)
				}
			}
			if input.DefaultOffloadPlugin != "" {
				args = append(args, "--default-offload-plugin", input.DefaultOffloadPlugin)
			}
			if input.DefaultOffloadSecret != "" {
				args = append(args, "--default-offload-secret", input.DefaultOffloadSecret)
			}
			if input.DefaultOffloadVendor != "" {
				args = append(args, "--default-offload-vendor", input.DefaultOffloadVendor)
			}
		}
		if input.InventoryURL != "" {
			args = append(args, "--inventory-url", input.InventoryURL)
		}
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
