package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// CreatePlanInput represents the input for CreatePlan
type CreatePlanInput struct {
	PlanName                       string `json:"plan_name" jsonschema:"required"`
	SourceProvider                 string `json:"source_provider" jsonschema:"required"`
	Namespace                      string `json:"namespace,omitempty"`
	TargetProvider                 string `json:"target_provider,omitempty"`
	NetworkMapping                 string `json:"network_mapping,omitempty"`
	StorageMapping                 string `json:"storage_mapping,omitempty"`
	NetworkPairs                   string `json:"network_pairs,omitempty"`
	StoragePairs                   string `json:"storage_pairs,omitempty"`
	VMs                            string `json:"vms,omitempty"`
	PreHook                        string `json:"pre_hook,omitempty"`
	PostHook                       string `json:"post_hook,omitempty"`
	Description                    string `json:"description,omitempty"`
	TargetNamespace                string `json:"target_namespace,omitempty"`
	TransferNetwork                string `json:"transfer_network,omitempty"`
	PreserveClusterCPUModel        *bool  `json:"preserve_cluster_cpu_model,omitempty"`
	PreserveStaticIPs              *bool  `json:"preserve_static_ips,omitempty"`
	PVCNameTemplate                string `json:"pvc_name_template,omitempty"`
	VolumeNameTemplate             string `json:"volume_name_template,omitempty"`
	NetworkNameTemplate            string `json:"network_name_template,omitempty"`
	MigrateSharedDisks             *bool  `json:"migrate_shared_disks,omitempty"`
	Archived                       *bool  `json:"archived,omitempty"`
	PVCNameTemplateUseGenerateName *bool  `json:"pvc_name_template_use_generate_name,omitempty"`
	DeleteGuestConversionPod       *bool  `json:"delete_guest_conversion_pod,omitempty"`
	DeleteVMOnFailMigration        *bool  `json:"delete_vm_on_fail_migration,omitempty"`
	SkipGuestConversion            *bool  `json:"skip_guest_conversion,omitempty"`
	InstallLegacyDrivers           string `json:"install_legacy_drivers,omitempty"`
	MigrationType                  string `json:"migration_type,omitempty"`
	DefaultTargetNetwork           string `json:"default_target_network,omitempty"`
	DefaultTargetStorageClass      string `json:"default_target_storage_class,omitempty"`
	UseCompatibilityMode           *bool  `json:"use_compatibility_mode,omitempty"`
	TargetLabels                   string `json:"target_labels,omitempty"`
	TargetNodeSelector             string `json:"target_node_selector,omitempty"`
	Warm                           *bool  `json:"warm,omitempty"`
	TargetAffinity                 string `json:"target_affinity,omitempty"`
	TargetPowerState               string `json:"target_power_state,omitempty"`
	InventoryURL                   string `json:"inventory_url,omitempty"`
	DefaultVolumeMode              string `json:"default_volume_mode,omitempty"`
	DefaultAccessMode              string `json:"default_access_mode,omitempty"`
	DefaultOffloadPlugin           string `json:"default_offload_plugin,omitempty"`
	DefaultOffloadSecret           string `json:"default_offload_secret,omitempty"`
	DefaultOffloadVendor           string `json:"default_offload_vendor,omitempty"`
	RunPreflightInspection         *bool  `json:"run_preflight_inspection,omitempty"`
	ConvertorLabels                string `json:"convertor_labels,omitempty"`
	ConvertorNodeSelector          string `json:"convertor_node_selector,omitempty"`
	ConvertorAffinity              string `json:"convertor_affinity,omitempty"`
}

// GetCreatePlanTool returns the tool definition
func GetCreatePlanTool() *mcp.Tool {
	return &mcp.Tool{
		Name: "CreatePlan",
		Description: `Create a new migration plan with comprehensive configuration options.

    Migration plans define which VMs to migrate and all the configuration for how they should be migrated.
    Plans coordinate providers, mappings, VM selection, and migration behavior.

    Automatic Behaviors:
    - Target provider: If not specified, uses first available OpenShift provider automatically
    - Target namespace: If not specified, uses the plan's namespace
    - Network/Storage mappings: Auto-created if not provided or specified as pairs (except for conversion-only migrations which skip storage mapping creation)
    - VM validation: All VMs are validated against provider inventory before plan creation
    - Missing VM handling: VMs not found in provider are automatically removed with warnings

    VM Selection Options:
    - vms: Three methods supported:
      1. Comma-separated VM names
      2. @filename for YAML/JSON file with VM structures
      3. Query string (prefix with "where ") for dynamic VM selection from inventory
    - All approaches support automatic ID resolution from provider inventory

    VM Selection Examples:
    - Comma-separated: "web-server-01,database-02,cache-03"
    - File-based: "@vm-list.yaml" or "@vm-list.json"
    - Query-based: "where name like 'prod%'" or "where powerState = 'On' and cpuCount >= 4"

    File Format (@filename):
    Files can contain VM structures in YAML or JSON format. VM IDs are optional and will be
    auto-resolved from inventory if not provided:

    YAML format (minimal - names only):
    - name: vm1
    - name: vm2
    - name: vm3

    YAML format (with IDs - from planvms output):
    - name: vm1
      id: vm-123
    - name: vm2
      id: vm-456

    JSON format (equivalent):
    [
      {"name": "vm1"},
      {"name": "vm2", "id": "vm-456"}
    ]

    Integration with read tools:
    1. Use ListInventory("vm", "provider", output_format="planvms") to get complete VM structures
    2. Save the YAML output to a file: vm-list.yaml
    3. Edit file to select desired VMs (optional)
    4. Use file: vms="@vm-list.yaml"

    Alternative: Create minimal files with just VM names, IDs will be auto-resolved

    Query Format (where ...):
    Query strings enable dynamic VM selection from inventory at plan creation time.
    The query must start with "where " and uses the same query language as ListInventory.

    Query syntax is validated before fetching inventory (fast failure for invalid queries).
    
    Query Language Support:
    - Filtering: where <condition>
    - Logical operators: and, or
    - Comparison: =, !=, <, >, <=, >=, like, not like
    - Limiting: limit <number>
    
    Common Query Fields:
    - name: VM name
    - powerState: Power state (On, Off, etc.)
    - cpuCount: Number of CPUs
    - memoryMB / memoryGB: Memory
    - guestId: Guest OS identifier
    - criticalConcerns / warningConcerns / infoConcerns: Migration concerns
    
    Query Examples:
    - "where name like 'prod%'" - All VMs starting with 'prod'
    - "where powerState = 'Off'" - All powered-off VMs
    - "where cpuCount >= 4 and memoryMB > 8192" - VMs with 4+ CPUs and >8GB RAM
    - "where criticalConcerns = 0 order by name limit 10" - First 10 VMs with no critical issues
    
    Testing Queries:
    Use ListInventory("vm", "provider-name", query="where ...") to test queries before creating plans

    Migration Types:
    - cold: VMs are shut down during migration (default, most reliable)
    - warm: Initial copy while VM runs, brief downtime for final sync
    - live: Minimal downtime migration (advanced, limited compatibility)
    - conversion: Only perform guest OS conversion without disk transfer (storage mappings not allowed)

    Note: Both migration_type and warm parameters are supported. If both are specified,
    migration_type takes precedence over the warm flag.

    Conversion-Only Migration Constraints:
    - Cannot use storage_mapping or storage_pairs parameters
    - Storage mapping will be empty in the resulting plan
    - Only network mapping is created/used for VM networking configuration

    Target Power State Options:
    - on: Start VMs after migration
    - off: Leave VMs stopped after migration
    - auto: Match source VM power state (default)

    Template Variables:
    Templates support Go template syntax with different variables for each template type:

    PVC Name Template Variables:
    - {{.VmName}} - VM name
    - {{.PlanName}} - Migration plan name
    - {{.DiskIndex}} - Initial volume index of the disk
    - {{.WinDriveLetter}} - Windows drive letter (lowercase, requires guest agent)
    - {{.RootDiskIndex}} - Index of the root disk
    - {{.Shared}} - True if volume is shared by multiple VMs
    - {{.FileName}} - Source file name (vSphere only, requires guest agent)

    Volume Name Template Variables:
    - {{.PVCName}} - Name of the PVC mounted to the VM
    - {{.VolumeIndex}} - Sequential index of volume interface (0-based)

    Network Name Template Variables:
    - {{.NetworkName}} - Multus network attachment definition name (if applicable)
    - {{.NetworkNamespace}} - Namespace of network attachment definition (if applicable)
    - {{.NetworkType}} - Network type ("Multus" or "Pod")
    - {{.NetworkIndex}} - Sequential index of network interface (0-based)

    Template Examples:
    - PVC: "{{.VmName}}-disk-{{.DiskIndex}}" → "web-server-01-disk-0"
    - PVC: "{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data{{end}}-{{.DiskIndex}}" → "root-0"
    - PVC: "{{if .Shared}}shared-{{end}}{{.VmName}}-{{.DiskIndex}}" → "shared-web-server-01-0"
    - Volume: "disk-{{.VolumeIndex}}" → "disk-0"
    - Volume: "pvc-{{.PVCName}}" → "pvc-web-server-01-disk-0"
    - Network: "net-{{.NetworkIndex}}" → "net-0"
    - Network: "{{if eq .NetworkType \"Pod\"}}pod{{else}}multus-{{.NetworkIndex}}{{end}}" → "pod"

    Available Template Functions:
    Templates support Go text template syntax including the following built-in functions:

    String Functions:
    - lower: Converts string to lowercase → {{ lower "TEXT" }} → text
    - upper: Converts string to uppercase → {{ upper "text" }} → TEXT
    - contains: Checks if string contains substring → {{ contains "hello" "lo" }} → true
    - replace: Replaces occurrences in a string → {{"I Am Henry VIII" | replace " " "-"}} → I-Am-Henry-VIII
    - trim: Removes whitespace from both ends → {{ trim "  text  " }} → text
    - trimAll: Removes specified characters from both ends → {{ trimAll "$" "$5.00$" }} → 5.00
    - trimSuffix: Removes suffix if present → {{ trimSuffix ".go" "file.go" }} → file
    - trimPrefix: Removes prefix if present → {{ trimPrefix "go." "go.file" }} → file
    - title: Converts to title case → {{ title "hello world" }} → Hello World
    - untitle: Converts to lowercase → {{ untitle "Hello World" }} → hello world
    - repeat: Repeats string n times → {{ repeat 3 "abc" }} → abcabcabc
    - substr: Extracts substring from start to end → {{ substr 1 4 "abcdef" }} → bcd
    - nospace: Removes all whitespace → {{ nospace "a b  c" }} → abc
    - trunc: Truncates string to specified length → {{ trunc 3 "abcdef" }} → abc
    - initials: Extracts first letter of each word → {{ initials "John Doe" }} → JD
    - hasPrefix: Checks if string starts with prefix → {{ hasPrefix "go" "golang" }} → true
    - hasSuffix: Checks if string ends with suffix → {{ hasSuffix "ing" "coding" }} → true
    - mustRegexReplaceAll: Replaces matches using regex → {{ mustRegexReplaceAll "a(x*)b" "-ab-axxb-" "${1}W" }} → -W-xxW-

    Math Functions:
    - add: Sum numbers → {{ add 1 2 3 }} → 6
    - add1: Increment by 1 → {{ add1 5 }} → 6
    - sub: Subtract second number from first → {{ sub 5 3 }} → 2
    - div: Integer division → {{ div 10 3 }} → 3
    - mod: Modulo operation → {{ mod 10 3 }} → 1
    - mul: Multiply numbers → {{ mul 2 3 4 }} → 24
    - max: Return largest integer → {{ max 1 5 3 }} → 5
    - min: Return smallest integer → {{ min 1 5 3 }} → 1
    - floor: Round down to nearest integer → {{ floor 3.75 }} → 3.0
    - ceil: Round up to nearest integer → {{ ceil 3.25 }} → 4.0
    - round: Round to specified decimal places → {{ round 3.75159 2 }} → 3.75

    Template Function Examples:
    - PVC with filename processing: "{{.FileName | trimSuffix \".vmdk\" | replace \"_\" \"-\" | lower}}"
    - PVC with conditional formatting: "{{if .Shared}}shared-{{else}}{{.VmName | lower}}-{{end}}disk-{{.DiskIndex}}"
    - Volume with uppercase naming: "{{.VmName | upper}}-VOL-{{.VolumeIndex}}"

    Args:
        plan_name: Name for the new migration plan (required)
        source_provider: Name of the source provider to migrate from (required). Supports namespace/name pattern (e.g., 'other-namespace/my-provider') to reference providers in different namespaces, defaults to plan namespace if not specified.
        namespace: Kubernetes namespace to create the plan in (optional)
        target_provider: Name of the target provider to migrate to (optional, auto-detects first OpenShift provider if not specified). Supports namespace/name pattern (e.g., 'other-namespace/my-provider') to reference providers in different namespaces, defaults to plan namespace if not specified.
        network_mapping: Name of existing network mapping to use (optional, auto-created if not provided)
        storage_mapping: Name of existing storage mapping to use (optional, auto-created if not provided)
        network_pairs: Network mapping pairs (optional, creates mapping if provided) - supports multiple formats:
            • 'source:target-namespace/target-network' - explicit namespace/name format
            • 'source:target-network' - uses plan namespace if no namespace specified
            • 'source:default' - maps to pod networking
            • 'source:ignored' - ignores the source network (can be used multiple times)

            VALIDATION RULES (enforced by MCP):
            • Pod networking ('default') can only be mapped ONCE across all sources
            • Each specific NAD can only be mapped ONCE across all sources
            • 'ignored' can be used multiple times for sources that don't need network access
            • INVALID: "source1:default,source2:default" (pod network mapped twice)
            • INVALID: "source1:nad1,source2:nad1" (same NAD mapped twice)
            • VALID: "source1:default,source2:ignored,source3:nad1"
            • VALID: "source1:nad1,source2:nad2,source3:ignored,source4:ignored"

            Note: All source networks must be mapped, validation prevents duplicate targets
        storage_pairs: Storage mapping pairs (optional, creates mapping if provided) - enhanced format with optional parameters:
            • Basic: 'source:storage-class' - simple storage class mapping
            • Enhanced: 'source:storage-class;volumeMode=Block;accessMode=ReadWriteOnce;offloadPlugin=vsphere;offloadSecret=secret;offloadVendor=flashsystem'
            • All semicolon-separated parameters are optional: volumeMode, accessMode, offloadPlugin, offloadSecret, offloadVendor
            Note: All source storages must be mapped, auto-selection uses
            storageclass.kubevirt.io/is-default-virt-class > storageclass.kubernetes.io/is-default-class >
            name with "virtualization"
        vms: VM selection - comma-separated names, @filename for YAML/JSON file, or query string with "where " prefix (optional)
        pre_hook: Pre-migration hook to add to all VMs (optional)
        post_hook: Post-migration hook to add to all VMs (optional)
        description: Plan description (optional)
        target_namespace: Target namespace for migrated VMs (optional, defaults to plan namespace)
        transfer_network: Network attachment definition for VM data transfer - supports 'namespace/network-name' or just 'network-name' (uses plan namespace) (optional)
        preserve_cluster_cpu_model: Preserve CPU model and flags from oVirt cluster (optional, default False)
        preserve_static_ips: Preserve static IPs of vSphere VMs (optional, default True, auto-patched if False)
        pvc_name_template: Template for generating PVC names for VM disks (optional)
        volume_name_template: Template for generating volume interface names (optional)
        network_name_template: Template for generating network interface names (optional)
        migrate_shared_disks: Whether to migrate shared disks (optional, default True, auto-patched if False)
        archived: Whether plan should be archived (optional, default False)
        pvc_name_template_use_generate_name: Use generateName for PVC template (optional, default True, auto-patched if False)
        delete_guest_conversion_pod: Delete conversion pod after migration (optional, default False)
        skip_guest_conversion: Skip guest conversion process (optional, default False)
        use_compatibility_mode: Use compatibility devices when skipping conversion (optional, default True, auto-patched if False)
        install_legacy_drivers: Install legacy Windows drivers - 'true'/'false' (optional)
        migration_type: Migration type - 'cold', 'warm', 'live', or 'conversion' (optional). Note: 'conversion' type cannot be used with storage_mapping or storage_pairs
        default_target_network: Default target network - 'default' for pod networking, 'namespace/network-name', or just 'network-name' (uses plan namespace) (optional)
        default_target_storage_class: Default target storage class (optional)
        target_labels: Target VM labels - 'key1=value1,key2=value2' format (optional)
        target_node_selector: Target node selector - 'key1=value1,key2=value2' format (optional)
        warm: Enable warm migration - prefer migration_type parameter (optional, default False)
        target_affinity: Target affinity using KARL syntax (optional)
            KARL (Kubernetes Affinity Rule Language) provides human-readable syntax for pod scheduling rules.

            KARL Rule Syntax: [RULE_TYPE] pods([SELECTORS]) on [TOPOLOGY]
            - Rule Types: REQUIRE, PREFER, AVOID, REPEL
            - Target: Only pods() supported (no node affinity)
            - Topology: node, zone, region, rack
            - No AND/OR logic support (single rule only)

            KARL Examples:
            - 'REQUIRE pods(app=database) on node' - Co-locate with database pods
            - 'PREFER pods(tier=web) on zone' - Prefer same zone as web pods
            - 'AVOID pods(app=cache) on node' - Separate from cache pods on same node
            - 'REPEL pods(workload=heavy) on zone weight=80' - Soft avoid in same zone
        target_power_state: Target power state - 'on', 'off', or 'auto' (optional)
        inventory_url: Base URL for inventory service (optional, auto-discovered if not provided)
        default_volume_mode: Default volume mode for storage pairs (Filesystem|Block) (optional)
        default_access_mode: Default access mode for storage pairs (ReadWriteOnce|ReadWriteMany|ReadOnlyMany) (optional)
        default_offload_plugin: Default offload plugin type for storage pairs (optional)
            • Supported plugins: vsphere
        default_offload_secret: Default offload plugin secret name for storage pairs (optional)
        default_offload_vendor: Default offload plugin vendor for storage pairs (optional)
            • Supported vendors: flashsystem, vantara, ontap, primera3par, pureFlashArray, powerflex, powermax, powerstore, infinibox
        run_preflight_inspection: Run preflight inspection on VM base disks before starting disk transfer (optional, default True, applies only to warm migrations from VMware)
        convertor_labels: Labels to be added to virt-v2v convertor pods - 'key1=value1,key2=value2' format (optional)
        convertor_node_selector: Node selector to constrain convertor pod scheduling - 'key1=value1,key2=value2' format (optional)
        convertor_affinity: Convertor affinity to constrain convertor pod scheduling using KARL syntax (optional)
            KARL syntax examples for convertor pods:
            - 'REQUIRE pods(app=storage) on node' - Co-locate convertor with storage pods
            - 'PREFER pods(tier=compute) on zone' - Prefer same zone as compute pods
            - 'AVOID pods(workload=heavy) on node' - Separate convertor from heavy workloads

    Returns:
        Command output confirming plan creation

    Examples:
        # Create basic plan (auto-detects target provider, creates mappings)
        create_plan("my-plan", "vsphere-provider")

        # Create plan with providers from different namespaces
        create_plan("my-plan", "source-ns/vsphere-provider",
                   target_provider="target-ns/openshift-provider",
                   namespace="demo")

        # Create comprehensive plan showing optional parameters and namespace/name syntax
        create_plan("my-plan", "vsphere-provider",
                   target_provider="openshift-target",
                   target_namespace="migrated-vms",
                   vms="vm1,vm2,vm3",
                   migration_type="warm",
                   # Network pairs: shows different formats (namespace/name, name-only, default, ignored)
                   network_pairs="VM Network:default,Management:mgmt-ns/mgmt-net,Production:prod-net,DMZ:ignored",
                   # Storage pairs: shows basic and enhanced format with optional parameters
                   storage_pairs="fast-datastore:premium-ssd,slow-datastore:standard-hdd;volumeMode=Block;accessMode=ReadWriteOnce;offloadPlugin=vsphere;offloadVendor=flashsystem",
                   default_volume_mode="Block",
                   default_target_network="openshift-sriov-network/high-perf-net",
                   transfer_network="sriov-namespace/transfer-network",
                   target_power_state="on",
                   description="Production VM migration showing optional parameters and formats")

        # Plan with comma-separated VM names
        create_plan("my-plan", "vsphere-provider",
                   vms="vm1,vm2,vm3")  # VMs identified by name, IDs auto-resolved

        # Plan with file-based VM selection (from planvms output)
        create_plan("my-plan", "vsphere-provider",
                   vms="@vm-list.yaml",  # from ListInventory("vm", "provider", output_format="planvms")
                   network_mapping="existing-net-map",
                   storage_mapping="existing-storage-map")

        # Plan with minimal file (names only, IDs auto-resolved)
        create_plan("my-plan", "vsphere-provider",
                   vms="@vm-names.yaml")  # YAML with just VM names

        # Create plan with KARL affinity rules
        create_plan("db-plan", "vsphere-provider",
                   vms="database-vm",
                   target_affinity="REQUIRE pods(app=database) on node",
                   description="Co-locate with existing database pods")

        # Plan with query-based VM selection (dynamic)
        create_plan("prod-migration", "vsphere-provider",
                   vms="where name like 'prod%' and powerState = 'On'",
                   migration_type="warm",
                   description="Migrate all running production VMs")

        # Plan with query - powered-off VMs for cold migration
        create_plan("cold-batch", "vsphere-provider",
                   vms="where powerState = 'Off' limit 10",
                   migration_type="cold",
                   description="Cold migration of first 10 powered-off VMs")`,
	}
}

func HandleCreatePlan(ctx context.Context, req *mcp.CallToolRequest, input CreatePlanInput) (*mcp.CallToolResult, any, error) {
	// Validate required parameters
	if err := mtvmcp.ValidateRequiredParams(map[string]string{
		"plan_name":       input.PlanName,
		"source_provider": input.SourceProvider,
	}); err != nil {
		return nil, "", err
	}

	// Validate conversion-only migration constraints
	if input.MigrationType == "conversion" {
		if input.StorageMapping != "" {
			return nil, "", fmt.Errorf("cannot use storage_mapping with migration_type 'conversion'")
		}
		if input.StoragePairs != "" {
			return nil, "", fmt.Errorf("cannot use storage_pairs with migration_type 'conversion'")
		}
	}

	// Validate network pairs constraints
	if err := mtvmcp.ValidateNetworkPairs(input.NetworkPairs); err != nil {
		return nil, "", err
	}

	args := []string{"create", "plan", input.PlanName}

	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}

	args = append(args, "--source", input.SourceProvider)
	if input.TargetProvider != "" {
		args = append(args, "--target", input.TargetProvider)
	}
	if input.NetworkMapping != "" {
		args = append(args, "--network-mapping", input.NetworkMapping)
	}
	if input.StorageMapping != "" {
		args = append(args, "--storage-mapping", input.StorageMapping)
	}
	if input.NetworkPairs != "" {
		args = append(args, "--network-pairs", input.NetworkPairs)
	}
	if input.StoragePairs != "" {
		args = append(args, "--storage-pairs", input.StoragePairs)
	}
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
	if input.VMs != "" {
		args = append(args, "--vms", input.VMs)
	}
	if input.PreHook != "" {
		args = append(args, "--pre-hook", input.PreHook)
	}
	if input.PostHook != "" {
		args = append(args, "--post-hook", input.PostHook)
	}

	// Plan configuration
	if input.Description != "" {
		args = append(args, "--description", input.Description)
	}
	if input.TargetNamespace != "" {
		args = append(args, "--target-namespace", input.TargetNamespace)
	}
	if input.TransferNetwork != "" {
		args = append(args, "--transfer-network", input.TransferNetwork)
	}
	mtvmcp.AddBooleanFlag(&args, "preserve-cluster-cpu-model", input.PreserveClusterCPUModel)
	mtvmcp.AddBooleanFlag(&args, "preserve-static-ips", input.PreserveStaticIPs)
	if input.PVCNameTemplate != "" {
		args = append(args, "--pvc-name-template", input.PVCNameTemplate)
	}
	if input.VolumeNameTemplate != "" {
		args = append(args, "--volume-name-template", input.VolumeNameTemplate)
	}
	if input.NetworkNameTemplate != "" {
		args = append(args, "--network-name-template", input.NetworkNameTemplate)
	}
	mtvmcp.AddBooleanFlag(&args, "migrate-shared-disks", input.MigrateSharedDisks)
	mtvmcp.AddBooleanFlag(&args, "archived", input.Archived)
	mtvmcp.AddBooleanFlag(&args, "pvc-name-template-use-generate-name", input.PVCNameTemplateUseGenerateName)
	mtvmcp.AddBooleanFlag(&args, "delete-guest-conversion-pod", input.DeleteGuestConversionPod)
	mtvmcp.AddBooleanFlag(&args, "delete-vm-on-fail-migration", input.DeleteVMOnFailMigration)
	mtvmcp.AddBooleanFlag(&args, "skip-guest-conversion", input.SkipGuestConversion)
	if input.InstallLegacyDrivers != "" {
		args = append(args, "--install-legacy-drivers", input.InstallLegacyDrivers)
	}
	if input.MigrationType != "" {
		mt := strings.ToLower(strings.TrimSpace(input.MigrationType))
		switch mt {
		case "cold", "warm", "live", "conversion":
			args = append(args, "--migration-type", mt)
		default:
			return nil, "", fmt.Errorf("invalid migration_type: %s (valid: cold|warm|live|conversion)", input.MigrationType)
		}
	}
	if input.DefaultTargetNetwork != "" {
		args = append(args, "--default-target-network", input.DefaultTargetNetwork)
	}
	if input.DefaultTargetStorageClass != "" {
		args = append(args, "--default-target-storage-class", input.DefaultTargetStorageClass)
	}
	mtvmcp.AddBooleanFlag(&args, "use-compatibility-mode", input.UseCompatibilityMode)
	if input.TargetLabels != "" {
		args = append(args, "--target-labels", input.TargetLabels)
	}
	if input.TargetNodeSelector != "" {
		args = append(args, "--target-node-selector", input.TargetNodeSelector)
	}
	if input.MigrationType == "" {
		mtvmcp.AddBooleanFlag(&args, "warm", input.Warm)
	}
	if input.TargetAffinity != "" {
		args = append(args, "--target-affinity", input.TargetAffinity)
	}
	if input.TargetPowerState != "" {
		ps := strings.ToLower(strings.TrimSpace(input.TargetPowerState))
		switch ps {
		case "on", "off", "auto":
			args = append(args, "--target-power-state", ps)
		default:
			return nil, "", fmt.Errorf("invalid target_power_state: %s (valid: on|off|auto)", input.TargetPowerState)
		}
	}
	if input.InventoryURL != "" {
		args = append(args, "--inventory-url", input.InventoryURL)
	}

	// Add new preflight and convertor flags
	mtvmcp.AddBooleanFlag(&args, "run-preflight-inspection", input.RunPreflightInspection)
	if input.ConvertorLabels != "" {
		args = append(args, "--convertor-labels", input.ConvertorLabels)
	}
	if input.ConvertorNodeSelector != "" {
		args = append(args, "--convertor-node-selector", input.ConvertorNodeSelector)
	}
	if input.ConvertorAffinity != "" {
		args = append(args, "--convertor-affinity", input.ConvertorAffinity)
	}

	result, err := mtvmcp.RunKubectlMTVCommand(args)
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
